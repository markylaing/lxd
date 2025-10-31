package broker

import (
	"context"
	"database/sql"
	"slices"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared"
)

// GetProjectsUsingRestrictedClusterGroups returns project URLs for all projects whose "restricted.cluster.groups" project configuration includes the specified groupName.
func GetProjectsUsingRestrictedClusterGroups(ctx context.Context, tx *sql.Tx, groupName string) ([]auth.Entity, error) {
	q := `
SELECT projects.id, projects.name, projects_config.value FROM projects 
JOIN projects_config ON projects.id = projects_config.project_id 
WHERE projects_config.key = 'restricted.cluster.groups'`

	var projects []auth.Entity
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var project Project
		var configValue string
		err := scan(&project.ID, &project.Name, &configValue)
		if err != nil {
			return err
		}

		if slices.Contains(shared.SplitNTrimSpace(configValue, ",", -1, false), groupName) {
			projects = append(projects, project)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return projects, nil
}
