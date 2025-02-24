package types

import (
	"fmt"
)

// entityTypeNetworkACL implements EntityTypeDB for a NetworkACL.
type entityTypeNetworkACL struct{}

func (e entityTypeNetworkACL) Code() int64 {
	return entityTypeCodeNetworkACL
}

func (e entityTypeNetworkACL) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, networks_acls.id, projects.name, '', json_array(networks_acls.name) 
FROM networks_acls 
JOIN projects ON networks_acls.project_id = projects.id`, e.Code())
}

func (e entityTypeNetworkACL) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeNetworkACL) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE networks_acls.id = ?`, e.AllURLsQuery())
}

func (e entityTypeNetworkACL) IDFromURLQuery() string {
	return `
SELECT ?, networks_acls.id 
FROM networks_acls 
JOIN projects ON networks_acls.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND networks_acls.name = ?`
}

func (e entityTypeNetworkACL) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_network_acl_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON networks_acls
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
