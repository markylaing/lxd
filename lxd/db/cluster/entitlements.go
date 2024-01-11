package cluster

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	dqliteDriver "github.com/canonical/go-dqlite/driver"
	"github.com/canonical/lxd/shared/api"
)

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t entitlements.mapper.go
//go:generate mapper reset -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e entitlement objects
//go:generate mapper stmt -e entitlement objects-by-ID
//go:generate mapper stmt -e entitlement objects-by-ObjectType
//go:generate mapper stmt -e entitlement objects-by-ObjectType-and-ObjectRef
//go:generate mapper stmt -e entitlement objects-by-ObjectType-and-EntityID
//go:generate mapper stmt -e entitlement objects-by-ObjectType-and-ObjectRef-and-Relation
//go:generate mapper stmt -e entitlement objects-by-ObjectType-and-EntityID-and-Relation
//
//go:generate mapper method -i -e entitlement GetMany

type Entitlement struct {
	ID         int
	Relation   string `db:"primary=true"`
	ObjectType string `db:"primary=true"`
	EntityID   *int
	ObjectRef  string `db:"primary=true"`
}

type EntitlementFilter struct {
	ID         *int
	Relation   *string
	ObjectType *string
	EntityID   *int
	ObjectRef  *string
}

func GetEntitlement(ctx context.Context, tx *sql.Tx, relation string, objectType string, objectRef string, entityID *int) (*Entitlement, error) {
	// Check if entitlement already exists.
	var entitlements []Entitlement
	var err error
	if entityID == nil {
		entitlements, err = GetEntitlements(ctx, tx, EntitlementFilter{
			Relation:   &relation,
			ObjectType: &objectType,
			ObjectRef:  &objectRef,
		})
	} else {
		entitlements, err = GetEntitlements(ctx, tx, EntitlementFilter{
			Relation:   &relation,
			ObjectType: &objectType,
			EntityID:   entityID,
		})
	}

	if err != nil {
		return nil, err
	}

	if len(entitlements) == 0 {
		return nil, api.StatusErrorf(http.StatusNotFound, "Entitlement not found")
	} else if len(entitlements) == 1 {
		return &entitlements[0], nil
	}

	return nil, fmt.Errorf("Multiple entitlements for the same object and relation")
}

func CreateEntitlement(ctx context.Context, tx *sql.Tx, entitlement Entitlement) (int, error) {
	var q string
	var args []any
	if entitlement.EntityID == nil {
		q = `INSERT INTO entitlements (relation, object_type, object_ref) VALUES (?, ?, ?)`
		args = []any{entitlement.Relation, entitlement.ObjectType, entitlement.ObjectRef}
	} else {
		q = `INSERT INTO entitlements (relation, object_type, object_ref, entity_id) VALUES (?, ?, ?, ?)`
		args = []any{entitlement.Relation, entitlement.ObjectType, entitlement.ObjectRef, *entitlement.EntityID}
	}

	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		var dqliteErr dqliteDriver.Error
		// Detect SQLITE_CONSTRAINT_UNIQUE (2067) errors.
		if errors.As(err, &dqliteErr) && dqliteErr.Code == 2067 {
			return 0, api.StatusErrorf(http.StatusConflict, "Entitlement already exists")
		}

		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}
