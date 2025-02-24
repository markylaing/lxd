package types

import (
	"fmt"
)

// entityTypeAuthGroup implements EntityTypeDB for an AuthGroup.
type entityTypeAuthGroup struct{}

func (e entityTypeAuthGroup) Code() int64 {
	return entityTypeCodeAuthGroup
}

func (e entityTypeAuthGroup) AllURLsQuery() string {
	return fmt.Sprintf(`SELECT %d, auth_groups.id, '', '', json_array(auth_groups.name) FROM auth_groups`, e.Code())
}

func (e entityTypeAuthGroup) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeAuthGroup) URLByIDQuery() string {
	return e.AllURLsQuery() + " WHERE auth_groups.id = ?"
}

func (e entityTypeAuthGroup) IDFromURLQuery() string {
	return `
SELECT ?, auth_groups.id 
FROM auth_groups 
WHERE '' = ? 
	AND '' = ? 
	AND auth_groups.name = ?`
}

func (e entityTypeAuthGroup) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_auth_group_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON auth_groups
	BEGIN
	DELETE FROM warnings
		WHERE entity_type_code = %d
		AND entity_id = OLD.id;
	END
`, name, e.Code())
}
