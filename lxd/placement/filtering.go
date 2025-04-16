package placement

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
)

// GetInstancesInPlacementGroup returns a map of instance ID to node ID for all instances that reference the given
// placement group either directly or indirectly via a profile.
func GetInstancesInPlacementGroup(ctx context.Context, tx *sql.Tx, project string, placementGroupName string) (map[int]int, error) {
	// Query notes:
	// 1. Union of profile config and instance config to perform single query. Since we plan to run this on the leader,
	//    this may not be necessary.
	// 2. The apply order must be a selected column, as we need to perform the ORDER BY to expand config correctly, and
	//    the ORDER BY can only be used after the UNION statement.
	// 3. The apply_order of 1000000 for instances is used to ensure that instance config is applied last.
	q := `
SELECT
	instances.id,
	instances.node_id,
	profiles_config.value,
	instances_profiles.apply_order AS apply_order
FROM instances 
	JOIN projects ON instances.project_id = projects.id
	JOIN instances_profiles ON instances.id = instances_profiles.instance_id 
	JOIN profiles ON instances_profiles.profile_id = profiles.id 
	JOIN profiles_config ON profiles.id = profiles_config.profile_id
WHERE projects.name = ? AND profiles_config.key = 'placement.group'
UNION
SELECT 
	instances.id,
	instances.node_id,
	instances_config.value,
	1000000 AS apply_order
FROM instances
	JOIN projects ON instances.project_id = projects.id
	JOIN instances_config ON instances.id = instances_config.instance_id 
WHERE projects.name = ? AND instances_config.key = 'placement.group'
	ORDER BY instances.id, apply_order`
	args := []any{project, project}

	// Keep a map of pointers so that each value is mutable.
	instIDToPlacementGroup := make(map[int]string)
	instIDToNodeID := make(map[int]int)
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var instID int
		var nodeID int
		var placementGroup string
		var applyOrder int
		err := scan(&instID, &nodeID, &placementGroup, &applyOrder)
		if err != nil {
			return err
		}

		instIDToNodeID[instID] = nodeID
		instIDToPlacementGroup[instID] = placementGroup
		return nil
	}, args...)
	if err != nil {
		return nil, err
	}

	result := make(map[int]int, len(instIDToPlacementGroup))
	for id, group := range instIDToPlacementGroup {
		if group == placementGroupName {
			result[id] = instIDToNodeID[id]
		}
	}

	return result, nil
}

// Filter filters the candidates using the placement group. If the runningInstanceID argument is passed, that instance
// is ignored (this is used to check if a running instance satisfies its placement group policy).
func Filter(ctx context.Context, tx *sql.Tx, candidates []db.NodeInfo, runningInstanceID *int, placementGroup cluster.PlacementGroup) ([]db.NodeInfo, error) {
	instToNode, err := GetInstancesInPlacementGroup(ctx, tx, placementGroup.Project, placementGroup.Name)
	if err != nil {
		return nil, err
	}

	// Omit current instance from rule application.
	if runningInstanceID != nil {
		delete(instToNode, *runningInstanceID)
	}

	nodeToAZInClusterGroup := make(map[int]int)
	nodeToAZQuery := `
SELECT 
	nodes.id, 
	coalesce(nodes.failure_domain_id, 0) AS failure_domain 
FROM nodes 
	JOIN nodes_cluster_groups ON nodes.id = nodes_cluster_groups.node_id 
	JOIN cluster_groups ON nodes_cluster_groups.group_id = cluster_groups.id
WHERE cluster_groups.name = ?`
	err = query.Scan(ctx, tx, nodeToAZQuery, func(scan func(dest ...any) error) error {
		var nodeID int
		var azID int
		err := scan(&nodeID, &azID)
		if err != nil {
			return err
		}

		nodeToAZInClusterGroup[nodeID] = azID
		return nil
	}, placementGroup.ClusterGroup)
	if err != nil {
		return nil, err
	}

	azToInst := make(map[int][]int, len(instToNode))
	nodeToInst := make(map[int][]int, len(instToNode))
	for instID, nodeID := range instToNode {
		nodeToInst[nodeID] = append(nodeToInst[nodeID], instID)
		az, ok := nodeToAZInClusterGroup[nodeID]
		if !ok {
			// TODO: This means an instance using this placement group is not in the correct cluster group. Can opportunistically create a warning.
			continue
		}

		azToInst[az] = append(azToInst[az], instID)
	}

	// First, filter candidates not in the cluster group
	candidatesInClusterGroup := make([]db.NodeInfo, 0, len(candidates))
	for _, candidate := range candidates {
		_, ok := nodeToAZInClusterGroup[int(candidate.ID)]
		if !ok {
			continue
		}

		candidatesInClusterGroup = append(candidatesInClusterGroup, candidate)
	}

	policy := api.PlacementPolicy(placementGroup.Policy)
	applyGroupRules := func(scope api.PlacementScope) ([]db.NodeInfo, error) {
		compliantCandidates := make([]db.NodeInfo, 0, len(candidatesInClusterGroup))
		for _, candidate := range candidatesInClusterGroup {
			switch {
			case policy == api.PlacementPolicyDistribute && scope == api.PlacementScopeClusterMember:
				// Filter candidates that already have instances.
				_, hasInst := nodeToInst[int(candidate.ID)]
				if hasInst {
					continue
				}

				compliantCandidates = append(compliantCandidates, candidate)
			case policy == api.PlacementPolicyDistribute && scope == api.PlacementScopeAvailabilityZone:
				// Filter candidates that are in an AZ that already has one or more instances.
				if len(azToInst[nodeToAZInClusterGroup[int(candidate.ID)]]) > 0 {
					continue
				}

				compliantCandidates = append(compliantCandidates, candidate)
			case policy == api.PlacementPolicyCompact && scope == api.PlacementScopeClusterMember:
				// If there are already instances using the placement group, filter candidates that do not already have
				// one or more instances.
				if len(instToNode) > 0 && len(nodeToInst[int(candidate.ID)]) == 0 {
					continue
				}

				compliantCandidates = append(compliantCandidates, candidate)
			case policy == api.PlacementPolicyCompact && scope == api.PlacementScopeAvailabilityZone:
				// If there are already instances using the placement group, filter candidates that are in an AZ that does
				// not have one or more instances.
				if len(instToNode) > 0 && len(azToInst[nodeToAZInClusterGroup[int(candidate.ID)]]) == 0 {
					continue
				}

				compliantCandidates = append(compliantCandidates, candidate)
			default:
				return nil, fmt.Errorf("Failed to schedule instance: Invalid placement group %q", placementGroup.Name)
			}
		}

		return compliantCandidates, nil
	}

	scope := api.PlacementScope(placementGroup.Scope)
	compliantCandidates, err := applyGroupRules(scope)
	if err != nil {
		return nil, err
	}

	switch api.PlacementRigor(placementGroup.Rigor) {
	case api.PlacementRigorStrict:
		// If rigor is strict, only return compliant candidates (might be empty).
		return compliantCandidates, nil
	case api.PlacementRigorPermissive:
		// With permissive rigor, return compliant candidates if there are any.
		if len(compliantCandidates) > 0 {
			return compliantCandidates, nil
		}

		// If the policy is distribute and the scope is availability zone, attempt to narrow the scope so that the
		// workload can still be distributed across cluster members.
		if policy == api.PlacementPolicyDistribute && scope == api.PlacementScopeAvailabilityZone {
			compliantCandidates, err = applyGroupRules(api.PlacementScopeClusterMember)
			if err != nil {
				return nil, err
			}

			if len(compliantCandidates) > 0 {
				return compliantCandidates, nil
			}
		}

		// If the policy is compact and the scope is cluster member, attempt to widen the scope so that the workload
		// can still be co-located within the AZ.
		if policy == api.PlacementPolicyCompact && scope == api.PlacementScopeClusterMember {
			compliantCandidates, err = applyGroupRules(api.PlacementScopeAvailabilityZone)
			if err != nil {
				return nil, err
			}

			if len(compliantCandidates) > 0 {
				return compliantCandidates, nil
			}
		}

		// Otherwise just return all members of the cluster group.
		return candidatesInClusterGroup, nil
	}

	return nil, fmt.Errorf("Failed to schedule instance: Invalid placement group %q", placementGroup.Name)
}
