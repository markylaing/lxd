package broker

import (
	"context"
	"database/sql"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/entity"
)

type authGroups struct {
	allLoaded   bool
	groups      map[int]*AuthGroup
	permissions map[int][]AuthGroupPermission
}

type AuthGroup struct {
	ID          int
	Name        string
	Description string
}

func (n AuthGroup) DatabaseID() int {
	return n.ID
}

func (n AuthGroup) EntityType() entity.Type {
	return entity.TypeAuthGroup
}

func (n AuthGroup) Parent() auth.Entity {
	return serverEntity{}
}

type AuthGroupFull struct {
	AuthGroup
	Permissions []AuthGroupPermission
}

type AuthGroupPermission struct {
	GroupID     int
	EntityType  cluster.EntityType
	EntityID    int
	Entitlement string
}

func (g *Model) GetAuthGroupsFull(ctx context.Context) ([]AuthGroupFull, error) {
	getFromCache := func() []AuthGroupFull {
		projects := make([]AuthGroupFull, 0, len(g.projects.projects))
		for id, group := range g.authGroups.groups {
			projects = append(projects, AuthGroupFull{
				AuthGroup:   *group,
				Permissions: g.authGroups.permissions[id],
			})
		}

		return projects
	}

	if g.authGroups.allLoaded {
		return getFromCache(), nil
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.authGroups.loadAllFull(ctx, tx.Tx())
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(), nil
}

func (p *authGroups) initialiseIfNeeded() {
	if p.groups == nil {
		p.groups = make(map[int]*AuthGroup)
	}

	if p.permissions == nil {
		p.permissions = make(map[int][]AuthGroupPermission)
	}
}

func (p *authGroups) loadAllFull(ctx context.Context, tx *sql.Tx) error {
	err := p.loadBySQLFull(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	return nil
}

func (p *authGroups) loadBySQLFull(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) error {
	p.initialiseIfNeeded()
	if p.allLoaded {
		return nil
	}

	q := `
SELECT 
	auth_groups.id, 
	auth_groups.name, 
	auth_groups.description, 
	coalesce(auth_groups_permissions.entity_type, -1), 
	coalesce(auth_groups_permissions.entity_id, -1), 
	coalesce(auth_groups_permissions.entitlement, '') 
FROM auth_groups 
	LEFT JOIN auth_groups_permissions ON auth_groups.id = auth_groups_permissions.auth_group_id` +
		sqlCondition + `
GROUP BY auth_groups.id`

	currentGroup := AuthGroupFull{AuthGroup: AuthGroup{ID: -1}}
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		group := AuthGroup{}
		permission := AuthGroupPermission{}
		err := scan(&group.ID, &group.Name, &group.Description, &permission.EntityType, &permission.EntityID, &permission.Entitlement)
		if err != nil {
			return err
		}

		permissionValid := permission.Entitlement != ""

		permission.GroupID = group.ID
		if currentGroup.ID == -1 {
			currentGroup.AuthGroup = group
			if permissionValid {
				currentGroup.Permissions = append(currentGroup.Permissions, permission)
			}
		} else if currentGroup.ID != group.ID {
			p.groups[currentGroup.ID] = &currentGroup.AuthGroup
			p.permissions[currentGroup.ID] = currentGroup.Permissions
		} else if permissionValid {
			currentGroup.Permissions = append(currentGroup.Permissions, permission)
		}

		return nil
	}, args)
	if err != nil {
		return err
	}

	p.groups[currentGroup.ID] = &currentGroup.AuthGroup
	p.permissions[currentGroup.ID] = currentGroup.Permissions
	return nil
}
