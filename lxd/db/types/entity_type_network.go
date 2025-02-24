package types

import (
	"fmt"
)

// entityTypeNetwork implements EntityTypeDB for a Network.
type entityTypeNetwork struct{}

func (e entityTypeNetwork) Code() int64 {
	return entityTypeCodeNetwork
}

func (e entityTypeNetwork) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, networks.id, projects.name, '', json_array(networks.name) 
FROM networks 
JOIN projects ON networks.project_id = projects.id`, e.Code())
}

func (e entityTypeNetwork) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeNetwork) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE networks.id = ?`, e.AllURLsQuery())
}

func (e entityTypeNetwork) IDFromURLQuery() string {
	return `
SELECT ?, networks.id 
FROM networks 
JOIN projects ON networks.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND networks.name = ?`
}

func (e entityTypeNetwork) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_network_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON networks
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
