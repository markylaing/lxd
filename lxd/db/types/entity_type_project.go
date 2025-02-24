package types

import (
	"fmt"
)

// entityTypeProject implements EntityTypeDB for a Project.
type entityTypeProject struct{}

func (e entityTypeProject) Code() int64 {
	return entityTypeCodeProject
}

func (e entityTypeProject) AllURLsQuery() string {
	return fmt.Sprintf(`SELECT %d, projects.id, projects.name, '', json_array(projects.name) FROM projects`, e.Code())
}

func (e entityTypeProject) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeProject) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE id = ?`, e.AllURLsQuery())
}

func (e entityTypeProject) IDFromURLQuery() string {
	return `
SELECT ?, projects.id 
FROM projects 
WHERE projects.name = ? 
	AND '' = ? 
	AND projects.name = ?`
}

func (e entityTypeProject) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_project_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON projects
	BEGIN
	DELETE FROM auth_groups_permissions 
		WHERE entity_type = %d 
		AND entity_id = OLD.id;
	DELETE FROM warnings
		WHERE entity_type_code = %d
		AND entity_id = OLD.id;
	END
`, name, e.Code(), e.Code())
}
