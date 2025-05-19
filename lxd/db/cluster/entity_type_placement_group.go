package cluster

import (
	"fmt"
	"strconv"
)

// entityTypePlacementGroup implements entityTypeDBInfo for a PlacementGroup.
type entityTypePlacementGroup struct{}

func (e entityTypePlacementGroup) code() int64 {
	return entityTypeCodePlacementGroup
}

func (e entityTypePlacementGroup) allURLsQuery() string {
	return `SELECT ` + strconv.FormatInt(e.code(), 10) + `, placement_groups.id, projects.name, '', json_array(placement_groups.name) FROM placement_groups JOIN projects ON placement_groups.project_id = projects.id`
}

func (e entityTypePlacementGroup) urlsByProjectQuery() string {
	return e.allURLsQuery() + " WHERE projects.name = ?"
}

func (e entityTypePlacementGroup) urlByIDQuery() string {
	return e.allURLsQuery() + " WHERE placement_groups.id = ?"
}

func (e entityTypePlacementGroup) idFromURLQuery() string {
	return `
SELECT ?, placement_groups.id 
FROM placements_groups
JOIN projects ON placement_groups.project_id = projects.id
WHERE projects.name = ? 
	AND '' = ? 
	AND placement_groups.name = ?`
}

func (e entityTypePlacementGroup) onDeleteTriggerSQL() (name string, sql string) {
	name = "on_placement_group_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON placement_groups
	BEGIN
	DELETE FROM auth_groups_permissions 
		WHERE entity_type = %d 
		AND entity_id = OLD.id;
	DELETE FROM warnings
		WHERE entity_type_code = %d
		AND entity_id = OLD.id;
	END
`, name, e.code(), e.code())
}
