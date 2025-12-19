package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type idpGroups struct {
	allLoaded bool
	groups    map[int64]*IdPGroup
}

type IdPGroup struct {
	ID           int64
	Name         string
	AuthGroupIDs []int64
}

func (n IdPGroup) DatabaseID() int64 {
	return n.ID
}

func (n IdPGroup) EntityType() entity.Type {
	return entity.TypeIdentityProviderGroup
}

func (n IdPGroup) Parent() auth.Entity {
	return serverEntity{}
}

func (n IdPGroup) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "auth", "identity-provider-groups", n.Name)
}

func GetIdentityProviderGroupsByNames(ctx context.Context, names ...string) ([]IdPGroup, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.idpGroups.init()

	if len(c.idpGroups.groups) > 0 {
		cachedIdPGroups, _ := datastructures.MapToSliceFilter(c.idpGroups.groups, func(k int64, v *IdPGroup) (bool, error) {
			return slices.Contains(names, v.Name), nil
		}, func(k int64, v *IdPGroup) (IdPGroup, error) {
			return *v, nil
		})
		if len(cachedIdPGroups) == len(names) {
			return cachedIdPGroups, nil
		}
	}

	var dbIdPGroups []IdPGroup
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		dbIdPGroups, err = c.idpGroups.loadByNames(ctx, tx.Tx(), names...)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return dbIdPGroups, nil
}

func (p *idpGroups) init() {
	if p.groups == nil {
		p.groups = make(map[int64]*IdPGroup)
	}
}

func (p *idpGroups) loadByNames(ctx context.Context, tx *sql.Tx, names ...string) ([]IdPGroup, error) {
	p.init()

	args, _ := datastructures.SliceToSlice(names, func(i int, e string) (any, error) {
		return e, nil
	})

	rows, err := p.loadBySQL(ctx, tx, "WHERE identity_provider_groups.name IN "+query.Params(len(names)), args...)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (p *idpGroups) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]IdPGroup, error) {
	p.init()

	q := `
SELECT 
	identity_provider_groups.id,
	identity_provider_groups.name,
	json_group_array(coalesce(auth_groups_identity_provider_groups.auth_group_id, -1)) AS groups
JOIN auth_groups_identity_provider_groups ON identity_provider_groups.id = auth_groups_identity_provider_groups.identity_provider_group_id
FROM identity_provider_groups
` + sqlCondition + `
GROUP BY identity_provider_groups.id
`

	var rows []IdPGroup
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		idpGroup := IdPGroup{}
		var authGroupsJSON []byte
		err := scan(&idpGroup.ID, &idpGroup.Name, &authGroupsJSON)
		if err != nil {
			return err
		}

		var authGroupIDs []int64
		err = json.Unmarshal(authGroupsJSON, &authGroupIDs)
		if err != nil {
			return err
		}

		if len(authGroupIDs) == 1 && authGroupIDs[0] == -1 {
			authGroupIDs = nil
		}

		idpGroup.AuthGroupIDs = authGroupIDs
		p.groups[idpGroup.ID] = &idpGroup
		rows = append(rows, idpGroup)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load idpGroups: %w", err)
	}

	return rows, nil
}
