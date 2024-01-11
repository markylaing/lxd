package cluster

import (
	"context"
	"database/sql"
	"strings"

	"github.com/canonical/lxd/shared/api"
)

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t groups.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e group objects
//go:generate mapper stmt -e group objects-by-ID
//go:generate mapper stmt -e group objects-by-Name
//go:generate mapper stmt -e group id
//go:generate mapper stmt -e group create
//go:generate mapper stmt -e group delete-by-Name
//go:generate mapper stmt -e group update
//
//go:generate mapper method -i -e group GetMany
//go:generate mapper method -i -e group GetOne
//go:generate mapper method -i -e group ID
//go:generate mapper method -i -e group Exists
//go:generate mapper method -i -e group Create
//go:generate mapper method -i -e group DeleteOne-by-Name
//go:generate mapper method -i -e group Update

type Group struct {
	ID          int
	Name        string `db:"primary=true"`
	Description string
}

type GroupFilter struct {
	ID   *int
	Name *string
}

func (g *Group) ToAPI(ctx context.Context, tx *sql.Tx) (*api.Group, error) {
	group := &api.Group{
		GroupPost: api.GroupPost{Name: g.Name},
		GroupPut:  api.GroupPut{Description: g.Description},
	}

	entitlements, err := GetGroupEntitlements(ctx, tx, g.ID)
	if err != nil {
		return nil, err
	}

	group.Entitlements = make(map[string][]string)
	for _, entitlement := range entitlements {
		object := strings.Join([]string{entitlement.ObjectType, entitlement.ObjectRef}, ":")
		group.Entitlements[object] = append(group.Entitlements[object], entitlement.Relation)
	}

	certificates, err := GetGroupCertificates(ctx, tx, g.ID)
	if err != nil {
		return nil, err
	}

	group.TLSUsers = make([]string, 0, len(certificates))
	for _, cert := range certificates {
		group.TLSUsers = append(group.TLSUsers, cert.Fingerprint)
	}

	group.IdPGroups = make([]string, 0)
	group.OIDCUsers = make([]string, 0)

	return group, nil
}
