package cluster

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t entitlements.mapper.go
//go:generate mapper reset -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e entitlement objects
//go:generate mapper stmt -e entitlement objects-by-ID
//go:generate mapper stmt -e entitlement objects-by-EntityType
//go:generate mapper stmt -e entitlement objects-by-EntityType-and-EntityID
//go:generate mapper stmt -e entitlement objects-by-EntityType-and-EntityID-and-Relation
//
//go:generate mapper method -i -e entitlement GetMany

type Entitlement struct {
	ID         int
	Relation   string `db:"primary=true"`
	EntityType int    `db:"primary=true"`
	EntityID   int    `db:"primary=true"`
}

type EntitlementFilter struct {
	ID         *int
	Relation   *string
	EntityType *int
	EntityID   *int
}

//func GetEntitlement(ctx context.Context, tx *sql.Tx, relation string, entityType int, entityID int) (*Entitlement, error) {
//	// Check if entitlement already exists.
//	var entitlements []Entitlement
//	var err error
//
//	entitlements, err = GetEntitlements(ctx, tx, EntitlementFilter{
//		Relation:   &relation,
//		EntityType: &entityType,
//		EntityID:   &entityID,
//	})
//
//	if err != nil {
//		return nil, err
//	}
//
//	if len(entitlements) == 0 {
//		return nil, api.StatusErrorf(http.StatusNotFound, "Entitlement not found")
//	} else if len(entitlements) == 1 {
//		return &entitlements[0], nil
//	}
//
//	return nil, fmt.Errorf("Multiple entitlements for the same object and relation")
//}
//
//func CreateEntitlement(ctx context.Context, tx *sql.Tx, entitlement Entitlement) (int, error) {
//
//	q := `INSERT INTO entitlements (relation, entity_type, entity_id) VALUES (?, ?, ?)`
//	args := []any{entity.Entitlement, entitlement.EntityType, entitlement.EntityID}
//
//	res, err := tx.ExecContext(ctx, q, args...)
//	if err != nil {
//		var dqliteErr dqliteDriver.Error
//		// Detect SQLITE_CONSTRAINT_UNIQUE (2067) errors.
//		if errors.As(err, &dqliteErr) && dqliteErr.Code == 2067 {
//			return 0, api.StatusErrorf(http.StatusConflict, "Entitlement already exists")
//		}
//
//		return 0, err
//	}
//
//	id, err := res.LastInsertId()
//	if err != nil {
//		return 0, err
//	}
//
//	return int(id), nil
//}
