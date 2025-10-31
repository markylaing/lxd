package broker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
)

type identities struct {
	identities map[int]*Identity
	groups     map[int][]int
	projects   map[int][]int
}

type Identity struct {
	ID            int
	AuthMethod    cluster.AuthMethod
	Type          cluster.IdentityType
	Identifier    string
	Name          string
	Metadata      string
	FirstSeenDate time.Time
	LastSeenDate  time.Time
	UpdatedDate   time.Time
}

type IdentityFull struct {
	Identity
	Groups   []int
	Projects []int
}

func (n Identity) DatabaseID() int {
	return n.ID
}

func (n Identity) EntityType() entity.Type {
	return entity.TypeIdentity
}

func (n Identity) Parent() auth.Entity {
	return serverEntity{}
}

func (g *Model) GetIdentityFullByAuthenticationMethodAndIdentifier(ctx context.Context, method string, identifier string) (*IdentityFull, error) {
	g.identities.initialiseIfNeeded()

	authMethod := cluster.AuthMethod(method)
	getFromCache := func(expectLoaded bool) (*IdentityFull, error) {
		_, identity, err := shared.FilterMapOnceFunc(g.identities.identities, func(i int, identity *Identity) bool {
			return identity.Identifier == identifier && identity.AuthMethod == authMethod
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Group not found")
			}

			return nil, nil
		}

		return &IdentityFull{
			Identity: *identity,
			Groups:   g.identities.groups[identity.ID],
			Projects: g.identities.projects[identity.ID],
		}, nil
	}

	identity, err := getFromCache(false)
	if err != nil {
		return nil, err
	}

	if identity != nil {
		return identity, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = g.identities.loadByMethodAndIdentifierFull(ctx, tx.Tx(), authMethod, identifier)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true)
}

func (p *identities) initialiseIfNeeded() {
	if p.groups == nil {
		p.groups = make(map[int][]int)
	}

	if p.projects == nil {
		p.projects = make(map[int][]int)
	}
}

func (p *identities) loadByMethodAndIdentifierFull(ctx context.Context, tx *sql.Tx, method cluster.AuthMethod, identifier string) error {
	return p.loadAllFullBySQL(ctx, tx, "WHERE identities.auth_method = ? AND identities.identifier = ?", method, identifier)
}

func (p *identities) loadAllFullBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) error {
	p.initialiseIfNeeded()

	q := `
SELECT 
	identities.id,
	identities.auth_method,
	identities.type,
	identities.identifier,
	identities.name,
	identities.metadata,
	identities.first_seen_date,
	identities.last_seen_date,
	identities.updated_date,
	json_group_array(coalesce(identities_auth_groups.auth_group_id, -1)) AS groups,
	json_group_array(coalesce(identities_projects.project_id, -1)) AS projects 
FROM identities 
	LEFT JOIN identities_auth_groups ON identities.id = identities_auth_groups.identity_id 
	LEFT JOIN identities_projects ON identities.id = identities_projects.identity_id
` + sqlCondition + `
GROUP BY identities.id
`
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		identity := Identity{}
		var groupsJSON, projectsJSON []byte
		err := scan(&identity.ID, &identity.AuthMethod, &identity.Type, &identity.Identifier, &identity.Name, &identity.Metadata, &identity.FirstSeenDate, &identity.LastSeenDate, &identity.UpdatedDate, &groupsJSON, &projectsJSON)
		if err != nil {
			return err
		}

		var groupIDs, projectIDs []int
		err = json.Unmarshal(groupsJSON, &groupIDs)
		if err != nil {
			return err
		}

		if len(groupIDs) == 1 && groupIDs[0] == -1 {
			groupIDs = nil
		}

		err = json.Unmarshal(projectsJSON, &projectIDs)
		if err != nil {
			return err
		}

		if len(projectIDs) == 1 && projectIDs[0] == -1 {
			projectIDs = nil
		}

		p.identities[identity.ID] = &identity
		p.groups[identity.ID] = groupIDs
		p.projects[identity.ID] = projectIDs
		return nil
	}, args...)
	if err != nil {
		return fmt.Errorf("Failed to load identities: %w", err)
	}

	return nil
}
