package types

import (
	"fmt"
)

// entityTypeImageAlias implements EntityTypeDB for an ImageAlias.
type entityTypeImageAlias struct{}

func (e entityTypeImageAlias) Code() int64 {
	return entityTypeCodeImageAlias
}

func (e entityTypeImageAlias) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, images_aliases.id, projects.name, '', json_array(images_aliases.name) 
FROM images_aliases 
JOIN projects ON images_aliases.project_id = projects.id`, e.Code())
}

func (e entityTypeImageAlias) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeImageAlias) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE images_aliases.id = ?`, e.AllURLsQuery())
}

func (e entityTypeImageAlias) IDFromURLQuery() string {
	return `
SELECT ?, images_aliases.id 
FROM images_aliases 
JOIN projects ON images_aliases.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND images_aliases.name = ? `
}

func (e entityTypeImageAlias) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_image_alias_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON images_aliases
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
