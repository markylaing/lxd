package types

import (
	"fmt"
)

// entityTypeClusterMember implements EntityTypeDB for a ClusterMember.
type entityTypeClusterMember struct{}

func (e entityTypeClusterMember) Code() int64 {
	return entityTypeCodeClusterMember
}

func (e entityTypeClusterMember) AllURLsQuery() string {
	return fmt.Sprintf(`SELECT %d, nodes.id, '', '', json_array(nodes.name) FROM nodes`, e.Code())
}

func (e entityTypeClusterMember) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeClusterMember) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE nodes.id = ?`, e.AllURLsQuery())
}

func (e entityTypeClusterMember) IDFromURLQuery() string {
	return `
SELECT ?, nodes.id 
FROM nodes 
WHERE '' = ? 
	AND '' = ? 
	AND nodes.name = ?`
}

func (e entityTypeClusterMember) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_node_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON nodes
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
