package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

type identities struct {
	allLoaded  bool
	identities map[int64]*Identity
}

type Identity struct {
	ID            int64
	AuthMethod    cluster.AuthMethod
	Type          cluster.IdentityType
	Identifier    string
	Name          string
	Metadata      []byte
	FirstSeenDate time.Time
	LastSeenDate  time.Time
	UpdatedDate   time.Time
	GroupIDs      []int64
	ProjectIDs    []int64
}

func (i Identity) ToAPI(groups map[int64]AuthGroup) (*api.Identity, error) {
	var certificate string
	if i.AuthMethod == api.AuthenticationMethodTLS {
		var metadata cluster.CertificateMetadata
		err := json.Unmarshal(i.Metadata, &metadata)
		if err != nil {
			return nil, fmt.Errorf("Failed to unmarshal identity metadata: %w", err)
		}

		certificate = metadata.Certificate
	}

	groupNames, err := datastructures.SliceToSlice(i.GroupIDs, func(_ int, e int64) (string, error) {
		group, ok := groups[e]
		if !ok {
			return "", fmt.Errorf("Failed to get identity group names: No group with ID %d", e)
		}

		return group.Name, nil
	})
	if err != nil {
		return nil, err
	}

	return &api.Identity{
		AuthenticationMethod: string(i.AuthMethod),
		Type:                 string(i.Type),
		Identifier:           i.Identifier,
		Name:                 i.Name,
		Groups:               groupNames,
		TLSCertificate:       certificate,
	}, nil
}

func (n Identity) DatabaseID() int64 {
	return n.ID
}

func (n Identity) EntityType() entity.Type {
	return entity.TypeIdentity
}

func (n Identity) Parent() auth.Entity {
	return serverEntity{}
}

func (n Identity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "auth", "identities", string(n.AuthMethod), n.Identifier)
}

func GetIdentityByAuthenticationMethodAndIdentifier(ctx context.Context, method cluster.AuthMethod, identifier string) (*Identity, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.identities.init()

	if len(c.identities.identities) > 0 {
		cachedIdentities, _ := datastructures.MapToSliceFilter(c.identities.identities, func(k int64, v *Identity) (bool, error) {
			return v.Identifier == identifier && v.AuthMethod == method, nil
		}, func(k int64, v *Identity) (Identity, error) {
			return *v, nil
		})
		if len(cachedIdentities) > 0 {
			return &cachedIdentities[0], nil
		}
	}

	var id *Identity
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		id, err = c.identities.loadByMethodAndIdentifier(ctx, tx.Tx(), method, identifier)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return id, nil
}

func (p *identities) init() {
	if p.identities == nil {
		p.identities = make(map[int64]*Identity)
	}
}

func (p *identities) loadByMethodAndIdentifier(ctx context.Context, tx *sql.Tx, method cluster.AuthMethod, identifier string) (*Identity, error) {
	p.init()

	rows, err := p.loadBySQL(ctx, tx, "WHERE identities.auth_method = ? AND identities.identifier = ?", method, identifier)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, api.StatusErrorf(http.StatusNotFound, "Identity not found")
	} else if len(rows) > 1 {
		return nil, errors.New("Multiple identities found with same identifier and authentication method")
	}

	return &rows[0], nil
}

func (p *identities) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]Identity, error) {
	p.init()

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

	var rows []Identity
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		identity := Identity{}
		var groupsJSON, projectsJSON []byte
		err := scan(&identity.ID, &identity.AuthMethod, &identity.Type, &identity.Identifier, &identity.Name, &identity.Metadata, &identity.FirstSeenDate, &identity.LastSeenDate, &identity.UpdatedDate, &groupsJSON, &projectsJSON)
		if err != nil {
			return err
		}

		var groupIDs, projectIDs []int64
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

		identity.GroupIDs = groupIDs
		identity.ProjectIDs = projectIDs
		p.identities[identity.ID] = &identity
		rows = append(rows, identity)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load identities: %w", err)
	}

	return rows, nil
}
