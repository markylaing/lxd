package types

import (
	"fmt"
)

// entityTypeNetworkZone implements EntityTypeDB for a NetworkZone.
type entityTypeNetworkZone struct{}

func (e entityTypeNetworkZone) Code() int64 {
	return entityTypeCodeNetworkZone
}

func (e entityTypeNetworkZone) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, networks_zones.id, projects.name, '', json_array(networks_zones.name) 
FROM networks_zones 
JOIN projects ON networks_zones.project_id = projects.id`, e.Code())
}

func (e entityTypeNetworkZone) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeNetworkZone) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE networks_zones.id = ?`, e.AllURLsQuery())
}

func (e entityTypeNetworkZone) IDFromURLQuery() string {
	return `
SELECT ?, networks_zones.id 
FROM networks_zones 
JOIN projects ON networks_zones.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND networks_zones.name = ?`
}

func (e entityTypeNetworkZone) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_network_zone_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON networks_zones
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
