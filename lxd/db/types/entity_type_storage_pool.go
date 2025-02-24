package types

import (
	"fmt"
)

// entityTypeStoragePool implements EntityTypeDB for a StoragePool.
type entityTypeStoragePool struct{}

func (e entityTypeStoragePool) Code() int64 {
	return entityTypeCodeStoragePool
}

func (e entityTypeStoragePool) AllURLsQuery() string {
	return fmt.Sprintf(`SELECT %d, storage_pools.id, '', '', json_array(storage_pools.name) FROM storage_pools`, e.Code())
}

func (e entityTypeStoragePool) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeStoragePool) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE storage_pools.id = ?`, e.AllURLsQuery())
}

func (e entityTypeStoragePool) IDFromURLQuery() string {
	return `
SELECT ?, storage_pools.id 
FROM storage_pools 
WHERE '' = ? 
	AND '' = ? 
	AND storage_pools.name = ?`
}

func (e entityTypeStoragePool) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_storage_pool_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON storage_pools
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
