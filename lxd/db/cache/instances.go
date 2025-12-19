package cache

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type instances struct {
	allLoaded bool
	instances map[int64]*Instance
	mu        sync.RWMutex
}

type Instance struct {
	ID           int64
	NodeID       int64
	Name         string
	Architecture int64
	Type         int64
	Ephemeral    bool
	CreationDate time.Time
	Stateful     bool
	LastUseDate  sql.NullTime
	Description  string
	ProjectID    int64
	ExpiryDate   sql.NullTime

	ProjectName string
}

func (n Instance) DatabaseID() int64 {
	return n.ID
}

func (n Instance) EntityType() entity.Type {
	return entity.TypeInstance
}

func (n Instance) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n Instance) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "instances", n.Name).Project(n.ProjectName)
}

func GetAllInstances(ctx context.Context) ([]Instance, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.instances.init()
	c.instances.mu.RLock()
	if c.instances.allLoaded {
		defer c.instances.mu.RUnlock()
		return datastructures.MapToSlice(c.instances.instances, func(_ int64, v *Instance) (Instance, error) {
			return *v, nil
		})
	}

	c.instances.mu.RUnlock()
	c.instances.mu.Lock()
	defer c.instances.mu.Unlock()
	var ids []Instance
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.instances.loadAll(ctx, tx.Tx())
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

func (p *instances) init() {
	if p.instances == nil {
		p.instances = make(map[int64]*Instance)
	}
}

func (p *instances) loadAll(ctx context.Context, tx *sql.Tx) ([]Instance, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *instances) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]Instance, error) {
	p.init()

	q := `
SELECT 
	instances.id,
	instances.node_id,
	instances.name,
	instances.architecture,
	instances.type,
	instances.ephemeral,
	instances.creation_date,
	instances.stateful,
	instances.last_use_date,
	instances.description,
	instances.project_id,
	instances.expiry_date,
	projects.name
FROM instances
JOIN projects ON instances.project_id = projects.id
` + sqlCondition

	var rows []Instance
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		instance := Instance{}
		err := scan(&instance.ID, &instance.NodeID, &instance.Name, &instance.Architecture, &instance.Type, &instance.Ephemeral, &instance.CreationDate, &instance.Stateful, &instance.LastUseDate, &instance.Description, &instance.ProjectID, &instance.ExpiryDate, &instance.ProjectName)
		if err != nil {
			return err
		}

		p.instances[instance.ID] = &instance
		rows = append(rows, instance)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load instances: %w", err)
	}

	return rows, nil
}
