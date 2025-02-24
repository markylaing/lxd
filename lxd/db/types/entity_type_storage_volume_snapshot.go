package types

import (
	"fmt"
	"strconv"
)

// entityTypeStorageVolumeSnapshot implements EntityTypeDB for a StorageVolumeSnapshot.
type entityTypeStorageVolumeSnapshot struct{}

func (e entityTypeStorageVolumeSnapshot) Code() int64 {
	return entityTypeCodeStorageVolumeSnapshot
}

func (e entityTypeStorageVolumeSnapshot) AllURLsQuery() string {
	return `
SELECT 
	` + strconv.FormatInt(e.Code(), 10) + `, 
	storage_volumes_snapshots.id, 
	projects.name, 
	replace(coalesce(nodes.name, ''), 'none', ''), 
	json_array(
		storage_pools.name,
		CASE storage_volumes.type
			WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeContainer, 10) + ` THEN '` + StoragePoolVolumeTypeNameContainer + `'
			WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeImage, 10) + ` THEN '` + StoragePoolVolumeTypeNameImage + `'
			WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeCustom, 10) + ` THEN '` + StoragePoolVolumeTypeNameCustom + `'
			WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeVM, 10) + ` THEN '` + StoragePoolVolumeTypeNameVM + `'
		END,
		storage_volumes.name,
		storage_volumes_snapshots.name
	)
FROM storage_volumes_snapshots
	JOIN storage_volumes ON storage_volumes_snapshots.storage_volume_id = storage_volumes.id
	JOIN projects ON storage_volumes.project_id = projects.id
	JOIN storage_pools ON storage_volumes.storage_pool_id = storage_pools.id
	LEFT JOIN nodes ON storage_volumes.node_id = nodes.id
`
}

func (e entityTypeStorageVolumeSnapshot) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeStorageVolumeSnapshot) URLByIDQuery() string {
	return fmt.Sprintf(`%s WHERE storage_volumes_snapshots.id = ?`, e.AllURLsQuery())
}

func (e entityTypeStorageVolumeSnapshot) IDFromURLQuery() string {
	return `
SELECT ?, storage_volumes_snapshots.id 
FROM storage_volumes_snapshots
JOIN storage_volumes ON storage_volumes_snapshots.storage_volume_id = storage_volumes.id
JOIN projects ON storage_volumes.project_id = projects.id
JOIN storage_pools ON storage_volumes.storage_pool_id = storage_pools.id
LEFT JOIN nodes ON storage_volumes.node_id = nodes.id
WHERE projects.name = ? 
	AND replace(coalesce(nodes.name, ''), 'none', '') = ? 
	AND storage_pools.name = ? 
	AND CASE storage_volumes.type
		WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeContainer, 10) + ` THEN '` + StoragePoolVolumeTypeNameContainer + `' 
		WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeImage, 10) + ` THEN '` + StoragePoolVolumeTypeNameImage + `'
		WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeCustom, 10) + ` THEN '` + StoragePoolVolumeTypeNameCustom + `'
		WHEN ` + strconv.FormatInt(StoragePoolVolumeTypeVM, 10) + ` THEN '` + StoragePoolVolumeTypeNameVM + `'
	END = ? 
	AND storage_volumes.name = ? 
	AND storage_volumes_snapshots.name = ?
`
}

func (e entityTypeStorageVolumeSnapshot) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_storage_volume_snapshot_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON storage_volumes_snapshots
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
