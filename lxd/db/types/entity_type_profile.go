package types

import (
	"fmt"
)

// entityTypeProfile implements EntityTypeDB for a Profile.
type entityTypeProfile struct{}

func (e entityTypeProfile) Code() int64 {
	return entityTypeCodeProfile
}

func (e entityTypeProfile) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, profiles.id, projects.name, '', json_array(profiles.name) 
FROM profiles 
JOIN projects ON profiles.project_id = projects.id`, e.Code())
}

func (e entityTypeProfile) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeProfile) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE profiles.id = ?`, e.AllURLsQuery())
}

func (e entityTypeProfile) IDFromURLQuery() string {
	return `
SELECT ?, profiles.id 
FROM profiles 
JOIN projects ON profiles.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND profiles.name = ?`
}

func (e entityTypeProfile) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_profile_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON profiles
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
