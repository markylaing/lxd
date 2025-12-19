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

type placementGroups struct {
	allLoaded       bool
	placementGroups map[int64]*PlacementGroup
	mu              sync.RWMutex
}

type PlacementGroup struct {
	ID          int64
	Name        string
	Description string
	ProjectID   int64

	ProjectName string
}

func (n PlacementGroup) DatabaseID() int64 {
	return n.ID
}

func (n PlacementGroup) EntityType() entity.Type {
	return entity.TypePlacementGroup
}

func (n PlacementGroup) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n PlacementGroup) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "placement-groups", n.Name).Project(n.ProjectName)
}

func GetAllPlacementGroups(ctx context.Context) ([]PlacementGroup, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.placementGroups.init()
	c.placementGroups.mu.RLock()
	if c.placementGroups.allLoaded {
		defer c.placementGroups.mu.RUnlock()
		return datastructures.MapToSlice(c.placementGroups.placementGroups, func(_ int64, v *PlacementGroup) (PlacementGroup, error) {
			return *v, nil
		})
	}

	c.placementGroups.mu.RUnlock()
	c.placementGroups.mu.Lock()
	defer c.placementGroups.mu.Unlock()
	var ids []PlacementGroup
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.placementGroups.loadAll(ctx, tx.Tx())
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

func (p *placementGroups) init() {
	if p.placementGroups == nil {
		p.placementGroups = make(map[int64]*PlacementGroup)
	}
}

func (p *placementGroups) loadAll(ctx context.Context, tx *sql.Tx) ([]PlacementGroup, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *placementGroups) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]PlacementGroup, error) {
	p.init()

	q := `
SELECT 
	placement_groups.id,
	placement_groups.name,
	placement_groups.description,
	placement_groups.project_id,
	projects.name
FROM placement_groups
JOIN projects ON placement_groups.project_id = projects.id
` + sqlCondition

	var rows []PlacementGroup
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		placementGroup := PlacementGroup{}
		err := scan(&placementGroup.ID, &placementGroup.Name, &placementGroup.Description, &placementGroup.ProjectID, &placementGroup.ProjectName)
		if err != nil {
			return err
		}

		p.placementGroups[placementGroup.ID] = &placementGroup
		rows = append(rows, placementGroup)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load placementGroups: %w", err)
	}

	return rows, nil
}
