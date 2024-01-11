package cluster

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t group_entitlements.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e group_entitlement objects
//go:generate mapper stmt -e group_entitlement objects-by-GroupID
//go:generate mapper stmt -e group_entitlement create struct=GroupEntitlement
//go:generate mapper stmt -e group_entitlement delete-by-GroupID
//
//go:generate mapper method -i -e group_entitlement GetMany struct=Group
//go:generate mapper method -i -e group_entitlement DeleteMany struct=Group
//go:generate mapper method -i -e group_entitlement Create struct=Group

// GroupEntitlement is an association table struct that associates
// Groups to Entitlements.
type GroupEntitlement struct {
	GroupID       int `db:"primary=yes"`
	EntitlementID int
}

// GroupEntitlementFilter specifies potential query parameter fields.
type GroupEntitlementFilter struct {
	GroupID       *int
	EntitlementID *int
}
