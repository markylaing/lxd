package types

import (
	"fmt"
	"strings"
)

// entityTypeOperation implements EntityTypeDB for an Operation.
type entityTypeOperation struct{}

func (e entityTypeOperation) Code() int64 {
	return entityTypeCodeOperation
}

func (e entityTypeOperation) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, operations.id, coalesce(projects.name, ''), '', json_array(operations.uuid) 
FROM operations 
LEFT JOIN projects ON operations.project_id = projects.id`, e.Code())
}

func (e entityTypeOperation) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, strings.Replace(e.AllURLsQuery(), "LEFT JOIN projects", "JOIN projects", 1))
}

func (e entityTypeOperation) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE operations.id = ?`, e.AllURLsQuery())
}

func (e entityTypeOperation) IDFromURLQuery() string {
	return `
SELECT ?, operations.id 
FROM operations 
LEFT JOIN projects ON operations.project_id = projects.id 
WHERE coalesce(projects.name, '') = ? 
	AND '' = ? 
	AND operations.uuid = ?`
}

func (e entityTypeOperation) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_operation_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON operations
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
