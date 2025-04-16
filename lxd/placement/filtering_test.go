package placement

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/instance/instancetype"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
)

type filteringSuite struct {
	suite.Suite
}

func TestFilteringSuite(t *testing.T) {
	suite.Run(t, new(filteringSuite))
}

func (s *filteringSuite) TestFilter() {
	testCluster, cleanup := db.NewTestCluster(s.T())
	defer cleanup()

	nodeNameToAZ := map[string]string{
		"member01": "A",
		"member02": "B",
		"member03": "B",
		"member04": "C",
		"member05": "C",
	}

	candidates := make([]db.NodeInfo, 0, len(nodeNameToAZ))
	i := 0
	for nodeName := range nodeNameToAZ {
		candidates = append(candidates, db.NodeInfo{Name: nodeName, Address: fmt.Sprintf("192.0.2.%d", i)})
		i++
	}

	candidatesWithout := func(members ...string) []db.NodeInfo {
		filteredCandidates := make([]db.NodeInfo, 0, len(candidates))
		for _, candidate := range candidates {
			if !shared.ValueInSlice(candidate.Name, members) {
				filteredCandidates = append(filteredCandidates, candidate)
			}
		}

		return filteredCandidates
	}

	err := testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
		for i, node := range candidates {
			id, err := tx.CreateNode(node.Name, node.Address)
			candidates[i].ID = id
			s.Require().NoError(err)

			err = tx.UpdateNodeFailureDomain(ctx, id, nodeNameToAZ[node.Name])
			s.Require().NoError(err)
		}

		return nil
	})
	s.Require().NoError(err)

	type args struct {
		candidates     []db.NodeInfo
		project        string
		placementGroup cluster.PlacementGroup
	}

	tests := []struct {
		name         string
		args         args
		caseSetup    func()
		caseTearDown func()
		want         []db.NodeInfo
		wantErr      error
	}{
		{
			name: "distribute over cluster members within cluster group g1 (initial)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					_, err := cluster.CreateClusterGroup(ctx, tx.Tx(), cluster.ClusterGroup{Name: "g1"})
					s.Require().NoError(err)

					for _, node := range candidatesWithout("member01", "member02") {
						err = tx.AddNodeToClusterGroup(ctx, "g1", node.Name)
						s.Require().NoError(err)
					}

					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeClusterMember),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02"),
		},
		{
			name: "distribute over cluster members within cluster group g1 (second, strict)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c1",
						Node:    "member03",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeClusterMember),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02", "member03"),
		},
		{
			name: "distribute over cluster members within cluster group g1 (all members occupied, strict)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c2",
						Node:    "member04",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					instanceID, err = cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c3",
						Node:    "member05",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeClusterMember),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: []db.NodeInfo{},
		},
		{
			name: "distribute over cluster members within cluster group g1 (all members occupied, permissive)",
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeClusterMember),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorPermissive),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02"),
		},
		{
			name: "distribute over availability zones within cluster group g1 (initial)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					for _, name := range []string{"c1", "c2", "c3"} {
						err := cluster.DeleteInstance(ctx, tx.Tx(), "default", name)
						s.Require().NoError(err)
					}

					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02"),
		},
		{
			name: "distribute over availability zones within cluster group g1 (second, strict)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c1",
						Node:    "member05",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02", "member04", "member05"),
		},
		{
			name: "distribute over availability zones within cluster group g1 (all AZs occupied, strict)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c2",
						Node:    "member03",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: []db.NodeInfo{},
		},
		{
			name: "distribute over availability zones within cluster group g1 (all AZs occupied, permissive)",
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorPermissive),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02", "member03", "member05"),
		},
		{
			name: "distribute over availability zones within cluster group g1 (all cluster members occupied, permissive)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c3",
						Node:    "member04",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyDistribute),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorPermissive),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02"),
		},
		{
			name: "compact to cluster member within cluster group g1 (initial)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					for _, name := range []string{"c1", "c2", "c3"} {
						err := cluster.DeleteInstance(ctx, tx.Tx(), "default", name)
						s.Require().NoError(err)
					}

					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyCompact),
					Scope:        cluster.PlacementScope(api.PlacementScopeClusterMember),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02"),
		},
		{
			name: "compact to cluster member within cluster group g1 (second)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c1",
						Node:    "member03",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyCompact),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02", "member04", "member05"),
		},
		{
			name: "compact to availability zone within cluster group g1 (initial)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					err := cluster.DeleteInstance(ctx, tx.Tx(), "default", "c1")
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyCompact),
					Scope:        cluster.PlacementScope(api.PlacementScopeClusterMember),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02"),
		},
		{
			name: "compact to availability zone within cluster group g1 (second)",
			caseSetup: func() {
				_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
					instanceID, err := cluster.CreateInstance(ctx, tx.Tx(), cluster.Instance{
						Name:    "c1",
						Node:    "member04",
						Project: "default",
						Type:    instancetype.Container,
					})
					s.Require().NoError(err)
					err = cluster.CreateInstanceConfig(ctx, tx.Tx(), instanceID, map[string]string{
						"placement.group": "pg1",
					})
					s.Require().NoError(err)
					return nil
				})
			},
			args: args{
				candidates: candidates,
				project:    "default",
				placementGroup: cluster.PlacementGroup{
					Project:      "default",
					Name:         "pg1",
					Description:  "Test placement group 1",
					Policy:       cluster.PlacementPolicy(api.PlacementPolicyCompact),
					Scope:        cluster.PlacementScope(api.PlacementScopeAvailabilityZone),
					Rigor:        cluster.PlacementRigor(api.PlacementRigorStrict),
					ClusterGroup: "g1",
				},
			},
			want: candidatesWithout("member01", "member02", "member03"),
		},
	}

	for i, tt := range tests {
		s.T().Logf("Case %d: %s", i, tt.name)
		if tt.caseSetup != nil {
			tt.caseSetup()
		}

		_ = testCluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
			got, err := Filter(ctx, tx.Tx(), tt.args.candidates, nil, tt.args.placementGroup)
			if tt.wantErr != nil {
				s.Equal(tt.wantErr, err)
				return nil
			}

			s.Require().NoError(err)
			s.ElementsMatch(tt.want, got)
			return nil
		})

		if tt.caseTearDown != nil {
			tt.caseTearDown()
		}
	}
}
