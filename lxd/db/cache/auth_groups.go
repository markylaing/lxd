package cache

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"sync/atomic"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type authGroups struct {
	groups               datastructures.SyncMap[int64, *AuthGroup]
	allPermissionsLoaded atomic.Bool
	permissions          datastructures.SyncMap[int64, []Permission]
}

type AuthGroup struct {
	ID          int64
	Name        string
	Description string
}

type Permission struct {
	AuthGroupID int64
	EntityType  cluster.EntityType
	EntityID    int64
	Entitlement auth.Entitlement
}

func (n AuthGroup) DatabaseID() int64 {
	return n.ID
}

func (n AuthGroup) EntityType() entity.Type {
	return entity.TypeAuthGroup
}

func (n AuthGroup) Parent() auth.Entity {
	return serverEntity{}
}

func (n AuthGroup) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "auth", "groups", n.Name)
}

func GetAuthGroupsByID(ctx context.Context, authGroupIDs ...int64) ([]AuthGroup, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	getCached := func(lock bool) ([]AuthGroup, error) {
		cachedAuthGroups, _ := datastructures.SyncMapToSliceFilter(&c.authGroups.groups, lock, func(k int64, _ *AuthGroup) (bool, error) {
			return slices.Contains(authGroupIDs, k), nil
		}, func(_ int64, v *AuthGroup) (AuthGroup, error) {
			return *v, nil
		})
		if len(cachedAuthGroups) == len(authGroupIDs) {
			return cachedAuthGroups, nil
		}

		return nil, api.NewStatusError(http.StatusNotFound, "One or more authorization groups were not found")
	}

	// Try to get from cache.
	groups, err := getCached(true)
	if err == nil {
		return groups, nil
	}

	// Missed.
	// Obtain write lock.
	c.authGroups.groups.Lock()
	defer c.authGroups.groups.Unlock()

	// Values may have been loaded while we were waiting to acquire the write lock.
	groups, err = getCached(false)
	if err == nil {
		return groups, nil
	}

	// Otherwise, load values and return.
	var ids []AuthGroup
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.authGroups.loadByIDs(ctx, tx.Tx(), authGroupIDs...)
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

func GetAllPermissions(ctx context.Context) ([]Permission, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	getCached := func(lock bool) ([]Permission, error) {
		if !c.authGroups.allPermissionsLoaded.Load() {
			return nil, api.NewStatusError(http.StatusNotFound, "Permissions not yet loaded")
		}

		var permissions []Permission
		_ = c.authGroups.permissions.Range(lock, func(k int64, v []Permission) error {
			permissions = append(permissions, v...)
			return nil
		})

		return permissions, nil
	}

	// Try to get from cache.
	permissions, err := getCached(true)
	if err == nil {
		return permissions, nil
	}

	// Missed.
	// Obtain write lock.
	c.authGroups.permissions.Lock()
	defer c.authGroups.permissions.Unlock()

	// Values may have been loaded while we were waiting to acquire the write lock.
	permissions, err = getCached(false)
	if err == nil {
		return permissions, nil
	}

	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		permissions, err = c.authGroups.loadAllPermissions(ctx, tx.Tx())
		return err
	})
	if err != nil {
		return nil, err
	}

	return permissions, nil
}

func (p *authGroups) loadByIDs(ctx context.Context, tx *sql.Tx, authGroupIDs ...int64) ([]AuthGroup, error) {
	return p.loadBySQL(ctx, tx, "WHERE auth_groups.id IN "+query.IntParams(authGroupIDs...))
}

func (p *authGroups) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]AuthGroup, error) {
	q := `
SELECT 
	auth_groups.id,
	auth_groups.name,
	auth_groups.description
FROM auth_groups
` + sqlCondition

	var rows []AuthGroup
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		authGroup := AuthGroup{}
		err := scan(&authGroup.ID, &authGroup.Name, &authGroup.Description)
		if err != nil {
			return err
		}

		p.groups.Set(false, authGroup.ID, &authGroup)
		rows = append(rows, authGroup)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load authorization roups: %w", err)
	}

	return rows, nil
}

func (p *authGroups) loadAllPermissions(ctx context.Context, tx *sql.Tx) ([]Permission, error) {
	q := `
SELECT 
	auth_groups_permissions.auth_group_id,
	auth_groups_permissions.entity_id,
	auth_groups_permissions.entity_type,
	auth_groups_permissions.entitlement
FROM auth_groups_permissions
`

	var rows []Permission
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		permission := Permission{}
		err := scan(&permission.AuthGroupID, &permission.EntityID, &permission.EntityType, &permission.Entitlement)
		if err != nil {
			return err
		}

		rows = append(rows, permission)

		permissions, _ := p.permissions.Get(false, permission.AuthGroupID)
		p.permissions.Set(false, permission.AuthGroupID, append(permissions, permission))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to load authorization roups: %w", err)
	}

	p.allPermissionsLoaded.Store(true)
	return rows, nil
}
