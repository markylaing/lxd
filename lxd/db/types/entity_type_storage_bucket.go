package types

import (
	"fmt"
)

// entityTypeStorageBucket implements EntityTypeDB for a StorageBucket.
type entityTypeStorageBucket struct{}

func (e entityTypeStorageBucket) Code() int64 {
	return entityTypeCodeStorageBucket
}

func (e entityTypeStorageBucket) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, storage_buckets.id, projects.name, replace(coalesce(nodes.name, ''), 'none', ''), json_array(storage_pools.name, storage_buckets.name)
FROM storage_buckets
	JOIN projects ON storage_buckets.project_id = projects.id
	JOIN storage_pools ON storage_buckets.storage_pool_id = storage_pools.id
	LEFT JOIN nodes ON storage_buckets.node_id = nodes.id
`, e.Code(),
	)
}

func (e entityTypeStorageBucket) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeStorageBucket) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE storage_buckets.id = ?`, e.AllURLsQuery())
}

func (e entityTypeStorageBucket) IDFromURLQuery() string {
	return `
SELECT ?, storage_buckets.id 
FROM storage_buckets
JOIN projects ON storage_buckets.project_id = projects.id
JOIN storage_pools ON storage_buckets.storage_pool_id = storage_pools.id
LEFT JOIN nodes ON storage_buckets.node_id = nodes.id
WHERE projects.name = ? 
	AND replace(coalesce(nodes.name, ''), 'none', '') = ? 
	AND storage_pools.name = ? 
	AND storage_buckets.name = ?
`
}

func (e entityTypeStorageBucket) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_storage_bucket_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON storage_buckets
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
