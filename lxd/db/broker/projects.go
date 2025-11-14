package broker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

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
}

func (p projects) filterByNameFunc(projectNames []string) func(i int, project *Project) (bool, error) {
	return func(i int, project *Project) (bool, error) {
		return slices.Contains(projectNames, project.Name), nil
	}
}

func (p projects) transformDereference(i int, project *Project) (Project, error) {
	if project == nil {
		return Project{}, fmt.Errorf("Encountered nil project with ID %d", i)
	}

	return *project, nil
}

type Project struct {
	ID          int
	Name        string
	Description string
	Config      map[string]string
}

func (p Project) ToAPI() api.Project {
	return api.Project{
		Name:        p.Name,
		Description: p.Description,
		Config:      p.Config,
	}
}

func (p Project) DatabaseID() int {
	return p.ID
}

func (p Project) EntityType() entity.Type {
	return entity.TypeProject
}

func (p Project) Parent() Reference {
	return serverReference{}
}

func (p Project) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "projects", p.Name)
}

func GetAllProjects(ctx context.Context) ([]Project, error) {
	model, err := getModelFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed getting broker model: %w", err)
	}

	getFromCache := func() ([]Project, error) {
		return shared.MapFilterTransformSlice(model.projects.projects, func(i int, project *Project) (bool, error) {
			return true, nil
		}, model.projects.transformDereference)
	}

	if model.projects.allLoaded {
		return getFromCache()
	}

	err = model.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = model.projects.loadAll(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return getFromCache()
}

func GetProjectByName(ctx context.Context, projectName string) (*Project, error) {
	projects, err := GetProjectsByName(ctx, projectName)
	if err != nil {
		return nil, err
	}

	return &projects[0], nil
}

func GetProjectsByName(ctx context.Context, projectNames ...string) ([]Project, error) {
	model, err := getModelFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed getting broker model: %w", err)
	}

	if len(projectNames) == 0 {
		return nil, errors.New("Must provide one or more project names")
	}

	getFromCache := func(expectLoaded bool) ([]Project, error) {
		projects, err := shared.MapFilterTransformSlice(model.projects.projects, model.projects.filterByNameFunc(projectNames), model.projects.transformDereference)
		if err != nil {
			return nil, fmt.Errorf("Failed to filter project cache: %w", err)
		}

		if len(projects) != len(projectNames) {
			if expectLoaded {
				// Unhappy path, figure out which projects weren't found.
				if len(projectNames) == 1 {
					return nil, api.StatusErrorf(http.StatusNotFound, "Project %q not found", projectNames[0])
				}

				if len(projects) == 0 {
					return nil, api.StatusErrorf(http.StatusNotFound, "Multiple projects were not found (%s)", strings.Join(projectNames, ", "))
				}

				missingProjectNames := make([]string, 0, len(projectNames))
				for _, projectName := range projectNames {
					if !slices.ContainsFunc(projects, func(project Project) bool {
						return projectName == project.Name
					}) {
						missingProjectNames = append(missingProjectNames, projectName)
					}
				}

				return nil, api.StatusErrorf(http.StatusNotFound, "Multiple projects were not found (%s)", strings.Join(missingProjectNames, ", "))
			}

			return nil, nil
		}

		return projects, nil
	}

	project, err := getFromCache(model.projects.allLoaded)
	if err != nil {
		return nil, err
	}

	if project != nil {
		return project, nil
	}

	err = model.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = model.projects.loadByName(ctx, tx.Tx(), projectNames...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true)
}

func (p *projects) ensureInit() {
	if p.projects == nil {
		p.projects = make(map[int]*Project)
	}
}

func (p *projects) loadAll(ctx context.Context, tx *sql.Tx) error {
	return p.loadBySQL(ctx, tx, "")
}

func (p *projects) loadByName(ctx context.Context, tx *sql.Tx, projectNames ...string) error {
	args := make([]any, 0, len(projectNames))
	for name := range slices.Values(projectNames) {
		args = append(args, name)
	}

	return p.loadBySQL(ctx, tx, "WHERE projects.name IN "+query.Params(len(args)), args...)
}

func (p *projects) loadBySQL(ctx context.Context, tx *sql.Tx, condition string, args ...any) error {
	p.ensureInit()

	q := `
SELECT 
	projects.id, 
	projects.name, 
	projects.description,
	json_group_object(coalesce(projects_confimodel.key, ''), coalesce(projects_confimodel.value, '')) AS config
FROM projects
` + condition + `
JOIN projects_config ON projects.id = projects_confimodel.project_id
GROUP BY projects.id`

	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var project Project
		var config ConfigMap
		err := scan(&project.ID, &project.Name, &project.Description, &config)
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
