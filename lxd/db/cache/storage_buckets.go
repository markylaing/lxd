package cache

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type storageBuckets struct {
	allLoaded      bool
	storageBuckets map[int64]*StorageBucket
	mu             sync.RWMutex
}

type StorageBucket struct {
	ID            int64
	Name          string
	StoragePoolID int64
	NodeID        int64
	Description   string
	ProjectID     int64

	ProjectName     string
	StoragePoolName string
	NodeName        string
}

func (n StorageBucket) DatabaseID() int64 {
	return n.ID
}

func (n StorageBucket) EntityType() entity.Type {
	return entity.TypeStorageBucket
}

func (n StorageBucket) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n StorageBucket) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "storage-pools", n.StoragePoolName, "buckets", n.Name).Project(n.ProjectName).Target(n.NodeName)
}

func GetAllStorageBuckets(ctx context.Context) ([]StorageBucket, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.storageBuckets.init()
	c.storageBuckets.mu.RLock()
	if c.storageBuckets.allLoaded {
		defer c.storageBuckets.mu.RUnlock()
		return datastructures.MapToSlice(c.storageBuckets.storageBuckets, func(_ int64, v *StorageBucket) (StorageBucket, error) {
			return *v, nil
		})
	}

	c.storageBuckets.mu.RUnlock()
	c.storageBuckets.mu.Lock()
	defer c.storageBuckets.mu.Unlock()
	var ids []StorageBucket
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.storageBuckets.loadAll(ctx, tx.Tx())
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

func (p *storageBuckets) init() {
	if p.storageBuckets == nil {
		p.storageBuckets = make(map[int64]*StorageBucket)
	}
}

func (p *storageBuckets) loadAll(ctx context.Context, tx *sql.Tx) ([]StorageBucket, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *storageBuckets) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]StorageBucket, error) {
	p.init()

	q := `
SELECT 
	storage_buckets.id,
	storage_buckets.name,
	storage_buckets.storage_pool_id,
	coalesce(storage_buckets.node_id, -1),
	storage_buckets.description,
	storage_buckets.project_id,
	projects.name,
	storage_pools.name,
	nodes.name
FROM storage_buckets
JOIN projects ON storage_buckets.project_id = projects.id
JOIN storage_pools ON storage_buckets.storage_pool_id = storage_pools.id
LEFT JOIN nodes ON storage_buckets.node_id = nodes.id
` + sqlCondition

	var rows []StorageBucket
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		storageBucket := StorageBucket{}
		err := scan(&storageBucket.ID, &storageBucket.Name, &storageBucket.StoragePoolID, &storageBucket.NodeID, &storageBucket.Description, &storageBucket.ProjectID, &storageBucket.ProjectName, &storageBucket.StoragePoolName, &storageBucket.NodeName)
		if err != nil {
			return err
		}

		p.storageBuckets[storageBucket.ID] = &storageBucket
		rows = append(rows, storageBucket)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load storageBuckets: %w", err)
	}

	return rows, nil
}
