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

type networks struct {
	allLoaded bool
	networks  map[int64]*Network
	mu        sync.RWMutex
}

type Network struct {
	ID          int64
	ProjectID   int64
	Name        string
	Description string
	State       int64
	Type        int64

	ProjectName string
}

func (n Network) DatabaseID() int64 {
	return n.ID
}

func (n Network) EntityType() entity.Type {
	return entity.TypeNetwork
}

func (n Network) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n Network) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "networks", n.Name).Project(n.ProjectName)
}

func GetAllNetworks(ctx context.Context) ([]Network, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.networks.init()
	c.networks.mu.RLock()
	if c.networks.allLoaded {
		defer c.networks.mu.RUnlock()
		return datastructures.MapToSlice(c.networks.networks, func(_ int64, v *Network) (Network, error) {
			return *v, nil
		})
	}

	c.networks.mu.RUnlock()
	c.networks.mu.Lock()
	defer c.networks.mu.Unlock()
	var ids []Network
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.networks.loadAll(ctx, tx.Tx())
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

func (p *networks) init() {
	if p.networks == nil {
		p.networks = make(map[int64]*Network)
	}
}

func (p *networks) loadAll(ctx context.Context, tx *sql.Tx) ([]Network, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *networks) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]Network, error) {
	p.init()

	q := `
SELECT 
	networks.id,
	networks.project_id,
	networks.name,
	networks.description,
	networks.state,
	networks.type,
	projects.name
FROM networks
JOIN projects ON networks.project_id = projects.id
` + sqlCondition

	var rows []Network
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		network := Network{}
		err := scan(&network.ID, &network.ProjectID, &network.Name, &network.Description, &network.State, &network.Type, &network.ProjectName)
		if err != nil {
			return err
		}

		p.networks[network.ID] = &network
		rows = append(rows, network)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load networks: %w", err)
	}

	return rows, nil
}
