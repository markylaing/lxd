package types

import (
	"fmt"
)

// entityTypeInstanceBackup implements EntityTypeDB for an InstanceBackup.
type entityTypeInstanceBackup struct{}

func (e entityTypeInstanceBackup) Code() int64 {
	return entityTypeCodeInstanceBackup
}

func (e entityTypeInstanceBackup) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, instances_backups.id, projects.name, '', json_array(instances.name, instances_backups.name)
FROM instances_backups 
JOIN instances ON instances_backups.instance_id = instances.id 
JOIN projects ON instances.project_id = projects.id`, e.Code())
}

func (e entityTypeInstanceBackup) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeInstanceBackup) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE instances_backups.id = ?`, e.AllURLsQuery())
}

func (e entityTypeInstanceBackup) IDFromURLQuery() string {
	return `
SELECT ?, instances_backups.id 
FROM instances_backups 
JOIN instances ON instances_backups.instance_id = instances.id 
JOIN projects ON instances.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND instances.name = ? 
	AND instances_backups.name = ?`
}

func (e entityTypeInstanceBackup) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_instance_backup_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON instances_backups
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
