package types

import (
	"fmt"
)

// entityTypeImage implements EntityTypeDB for an Image.
type entityTypeImage struct{}

func (e entityTypeImage) Code() int64 {
	return entityTypeCodeImage
}

func (e entityTypeImage) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, images.id, projects.name, '', json_array(images.fingerprint) 
FROM images 
JOIN projects ON images.project_id = projects.id`, e.Code())
}

func (e entityTypeImage) URLsByProjectQuery() string {
	return fmt.Sprintf("%s WHERE projects.name = ?", e.AllURLsQuery())
}

func (e entityTypeImage) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE images.id = ?`, e.AllURLsQuery())
}

func (e entityTypeImage) IDFromURLQuery() string {
	return `
SELECT ?, images.id 
FROM images 
JOIN projects ON images.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND images.fingerprint = ?`
}

func (e entityTypeImage) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_image_delete"
	return name, fmt.Sprintf(`CREATE TRIGGER %s
	AFTER DELETE ON images
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
