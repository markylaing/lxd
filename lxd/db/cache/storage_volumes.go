package cache

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type StorageVolumeType string

func (s *StorageVolumeType) Scan(value any) error {
	return query.ScanValue(value, s, false)
}

func (s *StorageVolumeType) ScanInteger(code int64) error {
	t, err := cluster.StoragePoolVolumeTypeFromInt(int(code))
	if err != nil {
		return err
	}

	*s = StorageVolumeType(t.String())
	return nil
}

type storageVolumes struct {
	allLoaded      bool
	storageVolumes map[int64]*StorageVolume
	mu             sync.RWMutex
}

type StorageVolume struct {
	ID            int64
	Name          string
	StoragePoolID int64
	NodeID        int64
	Type          StorageVolumeType
	Description   string
	ProjectID     int64
	ContentType   int64
	CreationDate  time.Time

	ProjectName     string
	StoragePoolName string
	NodeName        string
}

func (n StorageVolume) DatabaseID() int64 {
	return n.ID
}

func (n StorageVolume) EntityType() entity.Type {
	return entity.TypeStorageVolume
}

func (n StorageVolume) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n StorageVolume) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "storage-pools", n.StoragePoolName, "volumes", string(n.Type), n.Name).Project(n.ProjectName).Target(n.NodeName)
}

func GetAllStorageVolumes(ctx context.Context) ([]StorageVolume, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.storageVolumes.init()
	c.storageVolumes.mu.RLock()
	if c.storageVolumes.allLoaded {
		defer c.storageVolumes.mu.RUnlock()
		return datastructures.MapToSlice(c.storageVolumes.storageVolumes, func(_ int64, v *StorageVolume) (StorageVolume, error) {
			return *v, nil
		})
	}

	c.storageVolumes.mu.RUnlock()
	c.storageVolumes.mu.Lock()
	defer c.storageVolumes.mu.Unlock()
	var ids []StorageVolume
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.storageVolumes.loadAll(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func (p *storageVolumes) init() {
	if p.storageVolumes == nil {
		p.storageVolumes = make(map[int64]*StorageVolume)
	}
}

func (p *storageVolumes) loadAll(ctx context.Context, tx *sql.Tx) ([]StorageVolume, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *storageVolumes) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]StorageVolume, error) {
	p.init()

	q := `
SELECT 
	storage_volumes.id,
	storage_volumes.name,
	storage_volumes.storage_pool_id,
	coalesce(storage_volumes.node_id, -1),
	storage_volumes.type,
	storage_volumes.description,
	storage_volumes.project_id,
	storage_volumes.content_type,
	storage_volumes.creation_date,
	projects.name,
	storage_pools.name,
	coalesce(nodes.name, '')
FROM storage_volumes
JOIN projects ON storage_volumes.project_id = projects.id
JOIN storage_pools ON storage_volumes.storage_pool_id = storage_pools.id
LEFT JOIN nodes ON storage_volumes.node_id = nodes.id
` + sqlCondition

	var rows []StorageVolume
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		storageVolume := StorageVolume{}
		err := scan(&storageVolume.ID, &storageVolume.Name, &storageVolume.StoragePoolID, &storageVolume.NodeID, &storageVolume.Type, &storageVolume.Description, &storageVolume.ProjectID, &storageVolume.ContentType, &storageVolume.CreationDate, &storageVolume.ProjectName, &storageVolume.StoragePoolName, &storageVolume.NodeName)
		if err != nil {
			return err
		}

		p.storageVolumes[storageVolume.ID] = &storageVolume
		rows = append(rows, storageVolume)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load storageVolumes: %w", err)
	}

	return rows, nil
}
