package api

// PlacementPolicy determines how instances are scheduled for the PlacementGroup.
type PlacementPolicy string

const (
	// PlacementPolicyDistribute distributes instances across the PlacementScope.
	PlacementPolicyDistribute PlacementPolicy = "distribute"

	// PlacementPolicyCompact places all instances within the same PlacementScope.
	PlacementPolicyCompact PlacementPolicy = "compact"
)

// PlacementScope represents the scope for which the PlacementPolicy applies. For example, to place instances across
// availability zones, use PlacementPolicyDistribute and PlacementScopeAvailabilityZone.
type PlacementScope string

const (
	// PlacementScopeAvailabilityZone is the availability zone scope.
	PlacementScopeAvailabilityZone PlacementScope = "availability-zone"

	// PlacementScopeClusterMember is the cluster member scope.
	PlacementScopeClusterMember PlacementScope = "cluster-member"
)

// PlacementRigor determines whether LXD should fail to place an instance if no suitable cluster members can be found
// that match the PlacementPolicy and PlacementScope.
type PlacementRigor string

const (
	// PlacementRigorStrict causes LXD to fail to place instances if no cluster member can be found that matches the
	// conditions of the PlacementGroup.
	PlacementRigorStrict PlacementRigor = "strict"

	// PlacementRigorPermissive tells LXD to take a best-effort approach when placing instances. Instances will not fail
	// to place, but may not strictly adhere to the PlacementPolicy. For example, if using PlacementPolicyDistribute,
	// there may be more than one instance per PlacementScope.
	PlacementRigorPermissive PlacementRigor = "permissive"
)

// PlacementGroup defines how an instance should be placed.
//
// swagger:model
//
// API extension: placement_groups.
type PlacementGroup struct {
	WithEntitlements `yaml:",inline"`

	// Name is the name of the placement group.
	//
	// Example: foo
	Name string `json:"name" yaml:"name"`

	// Description is a freetext description of the group.
	//
	// Example: My group.
	Description string `json:"description" yaml:"description"`

	// Policy expresses the PlacementPolicy of the group.
	//
	// Example: distribute
	Policy PlacementPolicy `json:"policy" yaml:"policy"`

	// Scope expresses the PlacementScope for which the Policy is applied.
	//
	// Example: availability-zone
	Scope PlacementScope `json:"scope" yaml:"scope"`

	// Rigor determines whether the LXD server should fail if it cannot strictly meet the given Policy. For example,
	// when set to PlacementRigorStrict, if the Policy is PlacementPolicyDistribute, placement will fail if there is
	// already one instance within each Scope.
	//
	// Example: strict
	Rigor PlacementRigor `json:"rigor" yaml:"rigor"`

	// Project is the project containing the placement group.
	//
	// Example: default
	Project string `json:"project" yaml:"project"`

	// ClusterGroup is the cluster group against which the PlacementGroup is defined.
	//
	// Example: default
	ClusterGroup string `json:"cluster_group" yaml:"cluster_group"`

	// UsedBy is a list of resource URLs with soft references to this placement group.
	UsedBy []string `json:"used_by" yaml:"used_by"`
}

// Writable returns the editable fields of a PlacementGroup as PlacementGroupPut.
func (p PlacementGroup) Writable() PlacementGroupPut {
	return PlacementGroupPut{
		Description:  p.Description,
		Policy:       p.Policy,
		Scope:        p.Scope,
		Rigor:        p.Rigor,
		ClusterGroup: p.ClusterGroup,
	}
}

// PlacementGroupsPost contains the fields used to create a PlacementGroup.
//
// swagger:model
//
// API extension: placement_groups.
type PlacementGroupsPost struct {
	// Name is the name of the placement group.
	//
	// Example: foo
	Name string `json:"name" yaml:"name"`

	PlacementGroupPut `yaml:",inline"`
}

// PlacementGroupPut contains the updatable fields of a PlacementGroup.
//
// swagger:model
//
// API extension: placement_groups.
type PlacementGroupPut struct {
	// Description is a freetext description of the group.
	//
	// Example: My group.
	Description string `json:"description" yaml:"description"`

	// Policy expresses the PlacementPolicy of the group.
	//
	// Example: distribute
	Policy PlacementPolicy `json:"policy" yaml:"policy"`

	// Scope expresses the PlacementScope for which the Policy is applied.
	//
	// Example: availability-zone
	Scope PlacementScope `json:"scope" yaml:"scope"`

	// Rigor determines whether the LXD server should fail if it cannot strictly meet the given Policy. For example,
	// when set to PlacementRigorStrict, if the Policy is PlacementPolicyDistribute, placement will fail if there is
	// already one instance within each Scope.
	//
	// Example: strict
	Rigor PlacementRigor `json:"rigor" yaml:"rigor"`

	// ClusterGroup is the cluster group against which the PlacementGroup is defined.
	//
	// Example: default
	ClusterGroup string `json:"cluster_group" yaml:"cluster_group"`
}

// PlacementGroupPost is used to rename a PlacementGroup.
//
// swagger:model
//
// API extension: placement_groups.
type PlacementGroupPost struct {
	// Name is the new name of the placement group.
	//
	// Example: bar
	Name string `json:"name" yaml:"name"`
}
