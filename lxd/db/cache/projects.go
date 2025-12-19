package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type projects struct {
	allLoaded atomic.Bool
	projects  datastructures.SyncMap[int64, *Project]
	config    config
}

type Project struct {
	ID          int64
	Name        string
	Description string
}

func (n Project) DatabaseID() int64 {
	return n.ID
}

func (n Project) EntityType() entity.Type {
	return entity.TypeProject
}

func (n Project) Parent() auth.Entity {
	return serverEntity{}
}

func (n Project) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "projects", n.Name)
}

type ProjectFull struct {
	Project
	Config map[string]string
}

func (p ProjectFull) ToAPI() api.Project {
	return api.Project{
		Name:        p.Name,
		Description: p.Description,
		Config:      p.Config,
	}
}

func GetAllProjectsFull(ctx context.Context) ([]ProjectFull, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	getCached := func(lock bool) ([]ProjectFull, error) {
		if c.projects.allLoaded.Load() {
			return nil, api.NewStatusError(http.StatusNotFound, "Projects not yet loaded")
		}

		projectsFull, err := datastructures.SyncMapToSlice(&c.projects.projects, lock, func(k int64, v *Project) (ProjectFull, error) {
			config, ok := c.projects.config.configs.Get(true, k)
			if !ok {
				return ProjectFull{}, errNeedsExpansion
			}

			return ProjectFull{
				Project: *v,
				Config:  config,
			}, nil
		})
		if err != nil {
			return nil, err
		}

		return projectsFull, nil
	}

	projectsFull, err := getCached(true)
	if err == nil {
		return projectsFull, nil
	}

	if errors.Is(err, errNeedsExpansion) {
		err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
			return c.projects.config.load(ctx, tx.Tx())
		})
		if err != nil {
			return nil, err
		}

		return getCached(true)
	}

	c.projects.projects.Lock()
	defer c.projects.projects.Unlock()
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		projectsFull, err = c.projects.loadAllFull(ctx, tx.Tx())
		return err
	})
	if err != nil {
		return nil, err
	}

	return projectsFull, nil
}

func GetProjectsFullByID(ctx context.Context, projectIDs ...int64) ([]ProjectFull, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.projects.init()
	c.projects.config.init()

	if len(c.projects.projects) > 0 {
		c.projects.mu.RLock()
		missingConfigs := make([]int64, 0, len(projectIDs))
		cachedProjects, _ := datastructures.MapToSliceFilter(c.projects.projects, func(k int64, _ *Project) (bool, error) {
			return slices.Contains(projectIDs, k), nil
		}, func(_ int64, v *Project) (ProjectFull, error) {
			config := c.projects.config.getEntityConfig(v.ID)
			if config == nil {
				missingConfigs = append(missingConfigs, v.ID)
			}

			return ProjectFull{
				Project: *v,
				Config:  config,
			}, nil
		})

		c.projects.mu.RUnlock()
		if len(cachedProjects) == len(projectIDs) {
			if len(missingConfigs) == 0 {
				return cachedProjects, nil
			}

			err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
				return c.projects.config.load(ctx, tx.Tx(), missingConfigs...)
			})
			if err != nil {
				return nil, err
			}

			for _, cp := range cachedProjects {
				cp.Config = c.projects.config.getEntityConfig(cp.ID)
			}

			return cachedProjects, nil
		}
	}

	var ids []ProjectFull
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.projects.loadFullByIDs(ctx, tx.Tx(), projectIDs...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func GetProjectsFullByName(ctx context.Context, projectNames ...string) ([]ProjectFull, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.projects.init()
	c.projects.config.init()

	if len(c.projects.projects) > 0 {
		c.projects.mu.RLock()
		missingConfigs := make([]int64, 0, len(projectNames))
		cachedProjects, _ := datastructures.MapToSliceFilter(c.projects.projects, func(k int64, p *Project) (bool, error) {
			return slices.Contains(projectNames, p.Name), nil
		}, func(_ int64, v *Project) (ProjectFull, error) {
			config := c.projects.config.getEntityConfig(v.ID)
			if config == nil {
				missingConfigs = append(missingConfigs, v.ID)
			}

			return ProjectFull{
				Project: *v,
				Config:  config,
			}, nil
		})

		c.projects.mu.RUnlock()
		if len(cachedProjects) == len(projectNames) {
			if len(missingConfigs) == 0 {
				return cachedProjects, nil
			}

			err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
				return c.projects.config.load(ctx, tx.Tx(), missingConfigs...)
			})
			if err != nil {
				return nil, err
			}

			for _, cp := range cachedProjects {
				cp.Config = c.projects.config.getEntityConfig(cp.ID)
			}

			return cachedProjects, nil
		}
	}

	var ids []ProjectFull
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.projects.loadFullByNames(ctx, tx.Tx(), projectNames...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(ids) != len(projectNames) {
		return nil, api.StatusErrorf(http.StatusNotFound, "One or more projects from (%s) were not found", strings.Join(projectNames, ", "))
	}

	return ids, nil
}

func GetProjectsByID(ctx context.Context, projectIDs ...int64) ([]Project, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.projects.init()

	if len(c.projects.projects) > 0 {
		c.projects.mu.RLock()
		cachedProjects, _ := datastructures.MapToSliceFilter(c.projects.projects, func(k int64, _ *Project) (bool, error) {
			return slices.Contains(projectIDs, k), nil
		}, func(_ int64, v *Project) (Project, error) {
			return *v, nil
		})

		c.projects.mu.RUnlock()
		if len(cachedProjects) == len(projectIDs) {
			return cachedProjects, nil
		}
	}

	var ids []Project
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.projects.loadByIDs(ctx, tx.Tx(), projectIDs...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func GetAllProjects(ctx context.Context) ([]Project, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.projects.init()

	c.projects.mu.RLock()
	if c.projects.allLoaded {
		defer c.projects.mu.RUnlock()
		return datastructures.MapToSlice(c.projects.projects, func(_ int64, v *Project) (Project, error) {
			return *v, nil
		})
	}

	c.projects.mu.RUnlock()
	c.projects.mu.Lock()
	defer c.projects.mu.Unlock()
	var ids []Project
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.projects.loadAll(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func (p *projects) init() {
	if p.config == nil {
		p.config = &config{
			entityTable: "projects",
			configTable: "projects_config",
			foreignKey:  "project_id",
		}

		p.config.init()
	}
}

func (p *projects) loadAll(ctx context.Context, tx *sql.Tx) ([]Project, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded.Store(true)
	return result, nil
}

func (p *projects) loadByIDs(ctx context.Context, tx *sql.Tx, projectIDs ...int64) ([]Project, error) {
	p.init()

	return p.loadBySQL(ctx, tx, "WHERE projects.id IN "+query.IntParams(projectIDs...))
}

func (p *projects) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]Project, error) {
	p.init()

	q := `
SELECT 
	projects.id,
	projects.name,
	projects.description
FROM projects
` + sqlCondition + `
GROUP BY projects.id
`

	var rows []Project
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		project := Project{}
		err := scan(&project.ID, &project.Name, &project.Description)
		if err != nil {
			return err
		}

		p.projects.Set(false, project.ID, &project)
		rows = append(rows, project)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load projects: %w", err)
	}

	return rows, nil
}

func (p *projects) loadAllFull(ctx context.Context, tx *sql.Tx) ([]ProjectFull, error) {
	p.init()
	result, err := p.loadFullBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded.Store(true)
	return result, nil
}

func (p *projects) loadFullByIDs(ctx context.Context, tx *sql.Tx, ids ...int64) ([]ProjectFull, error) {
	return p.loadFullBySQL(ctx, tx, "WHERE projects.id IN "+query.IntParams(ids...))
}

func (p *projects) loadFullByNames(ctx context.Context, tx *sql.Tx, projectNames ...string) ([]ProjectFull, error) {
	args, _ := datastructures.SliceToSlice(projectNames, func(i int, e string) (any, error) {
		return e, nil
	})

	return p.loadFullBySQL(ctx, tx, "WHERE projects.name IN "+query.Params(len(args)), args...)
}

func (p *projects) loadFullBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]ProjectFull, error) {
	p.init()

	q := `
SELECT 
	projects.id,
	projects.name,
	projects.description,
	json_group_object(coalesce(projects_config.key, ""), coalesce(projects_config.value, "")) AS config
FROM projects
	LEFT JOIN projects_config ON projects.id = projects_config.project_id
` + sqlCondition + `
GROUP BY projects.id
`

	var rows []ProjectFull
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		project := Project{}
		var configJSON []byte
		err := scan(&project.ID, &project.Name, &project.Description, &configJSON)
		if err != nil {
			return err
		}

		var config map[string]string
		err = json.Unmarshal(configJSON, &config)
		if err != nil {
			return err
		}

		if len(config) == 1 && config[""] == "" {
			delete(config, "")
		}

		p.projects.Set(false, project.ID, &project)
		p.config.configs.Set(true, project.ID, config)
		rows = append(rows, ProjectFull{
			Project: project,
			Config:  config,
		})

		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load projects: %w", err)
	}

	return rows, nil
}
