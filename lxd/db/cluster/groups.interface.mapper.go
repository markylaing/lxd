//go:build linux && cgo && !agent

package cluster

import (
	"context"
	"database/sql"
)

// GroupGenerated is an interface of generated methods for Group.
type GroupGenerated interface {
	// GetGroups returns all available groups.
	// generator: group GetMany
	GetGroups(ctx context.Context, tx *sql.Tx, filters ...GroupFilter) ([]Group, error)

	// GetGroup returns the group with the given key.
	// generator: group GetOne
	GetGroup(ctx context.Context, tx *sql.Tx, name string) (*Group, error)

	// GetGroupID return the ID of the group with the given key.
	// generator: group ID
	GetGroupID(ctx context.Context, tx *sql.Tx, name string) (int64, error)

	// GroupExists checks if a group with the given key exists.
	// generator: group Exists
	GroupExists(ctx context.Context, tx *sql.Tx, name string) (bool, error)

	// CreateGroup adds a new group to the database.
	// generator: group Create
	CreateGroup(ctx context.Context, tx *sql.Tx, object Group) (int64, error)

	// DeleteGroup deletes the group matching the given key parameters.
	// generator: group DeleteOne-by-Name
	DeleteGroup(ctx context.Context, tx *sql.Tx, name string) error

	// UpdateGroup updates the group matching the given key parameters.
	// generator: group Update
	UpdateGroup(ctx context.Context, tx *sql.Tx, name string, object Group) error
}
