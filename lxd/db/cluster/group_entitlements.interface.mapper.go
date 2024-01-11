//go:build linux && cgo && !agent

package cluster

import (
	"context"
	"database/sql"
)

// GroupEntitlementGenerated is an interface of generated methods for GroupEntitlement.
type GroupEntitlementGenerated interface {
	// GetGroupEntitlements returns all available Entitlements for the Group.
	// generator: group_entitlement GetMany
	GetGroupEntitlements(ctx context.Context, tx *sql.Tx, groupID int) ([]Entitlement, error)

	// DeleteGroupEntitlements deletes the group_entitlement matching the given key parameters.
	// generator: group_entitlement DeleteMany
	DeleteGroupEntitlements(ctx context.Context, tx *sql.Tx, groupID int) error

	// CreateGroupEntitlements adds a new group_entitlement to the database.
	// generator: group_entitlement Create
	CreateGroupEntitlements(ctx context.Context, tx *sql.Tx, objects []GroupEntitlement) error
}
