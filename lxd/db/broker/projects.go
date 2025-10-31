package broker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type projects struct {
	allLoaded bool
	projects  map[int]*Project
	config    *Configs
}

type Project struct {
	ID          int
	Name        string
	Description string
}

func (n Project) DatabaseID() int {
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

func (g *Model) GetProjectByName(ctx context.Context, name string) (*Project, error) {
	getFromCache := func(expectLoaded bool, name string) (*Project, error) {
		_, project, err := shared.FilterMapOnceFunc(g.projects.projects, func(i int, project *Project) bool {
			return project.Name == name
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Project not found")
			}

			return nil, nil
		}

		return project, nil
	}

	project, err := getFromCache(g.projects.allLoaded, name)
	if err != nil {
		return nil, err
	}

	if project != nil {
		return project, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = g.projects.loadByName(ctx, tx.Tx(), name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name)
}

func (g *Model) GetProjectFullByName(ctx context.Context, name string) (*ProjectFull, error) {
	getFromCache := func(expectLoaded bool, name string) (*ProjectFull, error) {
		_, project, err := shared.FilterMapOnceFunc(g.projects.projects, func(i int, project *Project) bool {
			return project.Name == name
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Project not found")
			}

			return nil, nil
		}

		return &ProjectFull{
			Project: *project,
			Config:  g.projects.config.configs[project.ID],
		}, nil
	}

	project, err := getFromCache(g.projects.allLoaded, name)
	if err != nil {
		return nil, err
	}

	if project == nil {
		err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
			return g.projects.loadByNameFull(ctx, tx.Tx(), name)
		})
		if err != nil {
			return nil, err
		}

		return getFromCache(true, name)
	} else if project.Config == nil {
		err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
			return g.projects.config.load(ctx, tx.Tx(), project.ID)
		})
		if err != nil {
			return nil, err
		}

		return getFromCache(true, name)
	} else {
		return project, nil
	}
}

func (g *Model) GetProjectsFull(ctx context.Context) ([]ProjectFull, error) {
	getFromCache := func() []ProjectFull {
		projects := make([]ProjectFull, 0, len(g.projects.projects))
		for id, project := range g.projects.projects {
			projects = append(projects, ProjectFull{
				Project: *project,
				Config:  g.projects.config.configs[id],
			})
		}

		return projects
	}

	if g.projects.allLoaded {
		return getFromCache(), nil
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.projects.loadAllFull(ctx, tx.Tx())
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(), nil
}

func (p *projects) initialiseIfNeeded() {
	if p.projects == nil {
		p.projects = make(map[int]*Project)
	}

	if p.config == nil {
		p.config = &Configs{
			entityTable: "projects",
			configTable: "projects_config",
			foreignKey:  "project_id",
		}
	}
}

func (p *projects) loadAll(ctx context.Context, tx *sql.Tx) error {
	p.initialiseIfNeeded()

	q := `SELECT projects.id, projects.name, projects.description FROM projects`
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		project := Project{}
		err := scan(&project.ID, &project.Name, &project.Description)
		if err != nil {
			return err
		}

		p.projects[project.ID] = &project
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to load projects: %w", err)
	}

	p.allLoaded = true
	return nil
}

func (p *projects) loadByName(ctx context.Context, tx *sql.Tx, projectNames ...string) error {
	p.initialiseIfNeeded()

	args := make([]any, 0, len(projectNames))
	for name := range slices.Values(projectNames) {
		args = append(args, name)
	}

	q := `
SELECT 
	projects.id, 
	projects.name, 
	projects.description
FROM projects
WHERE projects.name IN ` + query.Params(len(args))

	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var project Project
		err := scan(&project.ID, &project.Name, &project.Description)
		if err != nil {
			return err
		}

		p.projects[project.ID] = &project
		return nil
	}, args...)
	if err != nil {
		return fmt.Errorf("Failed to load projects: %w", err)
	}

	return nil
}

func (p *projects) loadAllFull(ctx context.Context, tx *sql.Tx) error {
	p.initialiseIfNeeded()

	err := p.loadAll(ctx, tx)
	if err != nil {
		return err
	}

	err = p.config.load(ctx, tx)
	if err != nil {
		return err
	}

	return nil
}

func (p *projects) loadByNameFull(ctx context.Context, tx *sql.Tx, projectNames ...string) error {
	p.initialiseIfNeeded()

	args := make([]any, 0, len(projectNames))
	for name := range slices.Values(projectNames) {
		args = append(args, name)
	}

	q := `
SELECT 
	projects.id, 
	projects.name, 
	projects.description
FROM projects
WHERE projects.name IN ` + query.Params(len(args)) + `
GROUP BY projects.id`

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var project ProjectFull
		err := scan(&project.ID, &project.Name, &project.Description, &project.Config)
		if err != nil {
			return err
		}

		p.config.configs[project.ID] = project.Config
		p.projects[project.ID] = &project.Project
		ids = append(ids, project.ID)
		return nil
	}, args...)
	if err != nil {
		return fmt.Errorf("Failed to load projects: %w", err)
	}

	return p.config.load(ctx, tx, ids...)
}
