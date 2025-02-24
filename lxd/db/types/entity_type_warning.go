package types

import (
	"fmt"
	"strings"
)

// entityTypeWarning implements EntityTypeDB for a Warning.
type entityTypeWarning struct{}

func (e entityTypeWarning) Code() int64 {
	return entityTypeCodeWarning
}

func (e entityTypeWarning) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, warnings.id, coalesce(projects.name, ''), replace(coalesce(nodes.name, ''), 'none', ''), json_array(warnings.uuid) 
FROM warnings 
LEFT JOIN projects ON warnings.project_id = projects.id 
LEFT JOIN nodes ON warnings.node_id = nodes.id`, e.Code())
}

func (e entityTypeWarning) URLsByProjectQuery() string {
	return strings.Replace(e.AllURLsQuery(), "LEFT JOIN projects", "JOIN projects", 1) + " WHERE projects.name = ?"
}

func (e entityTypeWarning) URLByIDQuery() string {
	return e.AllURLsQuery() + " WHERE warnings.id = ?"
}

func (e entityTypeWarning) IDFromURLQuery() string {
	return `
SELECT ?, warnings.id 
FROM warnings 
LEFT JOIN projects ON warnings.project_id = projects.id 
WHERE coalesce(projects.name, '') = ? 
	AND '' = ? 
	AND warnings.uuid = ?`
}

func (e entityTypeWarning) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_warning_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON warnings
	BEGIN
	DELETE FROM auth_groups_permissions 
		WHERE entity_type = %d 
		AND entity_id = OLD.id;
	END
`, name, e.Code())
}
