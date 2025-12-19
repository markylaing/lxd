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

type networkACLs struct {
	allLoaded   bool
	networkACLs map[int64]*NetworkACL
	mu          sync.RWMutex
}

type NetworkACL struct {
	ID          int64
	ProjectID   int64
	Name        string
	Description string
	Ingress     string
	Egress      string

	ProjectName string
}

func (n NetworkACL) DatabaseID() int64 {
	return n.ID
}

func (n NetworkACL) EntityType() entity.Type {
	return entity.TypeNetworkACL
}

func (n NetworkACL) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n NetworkACL) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "network-acls", n.Name).Project(n.ProjectName)
}

func GetAllNetworkACLs(ctx context.Context) ([]NetworkACL, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.networkACLs.init()
	c.networkACLs.mu.RLock()
	if c.networkACLs.allLoaded {
		defer c.networkACLs.mu.RUnlock()
		return datastructures.MapToSlice(c.networkACLs.networkACLs, func(_ int64, v *NetworkACL) (NetworkACL, error) {
			return *v, nil
		})
	}

	c.networkACLs.mu.RUnlock()
	c.networkACLs.mu.Lock()
	defer c.networkACLs.mu.Unlock()
	var ids []NetworkACL
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.networkACLs.loadAll(ctx, tx.Tx())
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

func (p *networkACLs) init() {
	if p.networkACLs == nil {
		p.networkACLs = make(map[int64]*NetworkACL)
	}
}

func (p *networkACLs) loadAll(ctx context.Context, tx *sql.Tx) ([]NetworkACL, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *networkACLs) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]NetworkACL, error) {
	p.init()

	q := `
SELECT 
	networks_acls.id,
	networks_acls.project_id,
	networks_acls.name,
	networks_acls.description,
	networks_acls.ingress,
	networks_acls.egress,
	projects.name
FROM networks_acls
JOIN projects ON networks_acls.project_id = projects.id
` + sqlCondition

	var rows []NetworkACL
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		networkACL := NetworkACL{}
		err := scan(&networkACL.ID, &networkACL.ProjectID, &networkACL.Name, &networkACL.Description, &networkACL.Ingress, &networkACL.Egress, &networkACL.ProjectName)
		if err != nil {
			return err
		}

		p.networkACLs[networkACL.ID] = &networkACL
		rows = append(rows, networkACL)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load networkACLs: %w", err)
	}

	return rows, nil
}
