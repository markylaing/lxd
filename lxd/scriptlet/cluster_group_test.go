package scriptlet

import (
	"context"
	"github.com/canonical/lxd/shared/api"
	"github.com/stretchr/testify/suite"
	"testing"
)

type ClusterGroupMembershipTesterSuite struct {
	suite.Suite
}

func TestClusterGroupMembershipSuite(t *testing.T) {
	suite.Run(t, new(ClusterGroupMembershipTesterSuite))
}

func (s *ClusterGroupMembershipTesterSuite) TestFilterMembers() {
	type args struct {
		groupName string
		isMember  string
		members   []ClusterMember
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "all",
			args: args{
				groupName: "test",
				isMember:  "return True",
				members: []ClusterMember{
					{
						Name: "m1",
					},
					{
						Name: "m2",
					},
					{
						Name: "m3",
					},
				},
			},
			want: []string{"m1", "m2", "m3"},
		},
		{
			name: "m1 & m2",
			args: args{
				groupName: "test",
				isMember:  `return member.name == "m1" or member.name == "m2"`,
				members: []ClusterMember{
					{
						Name: "m1",
					},
					{
						Name: "m2",
					},
					{
						Name: "m3",
					},
				},
			},
			want: []string{"m1", "m2"},
		},
		{
			name: "has gpu",
			args: args{
				groupName: "test",
				isMember:  `return member.resources.gpu.total > 0`,
				members: []ClusterMember{
					{
						Name: "m1",
						Resources: api.Resources{
							GPU: api.ResourcesGPU{
								Total: 1,
							},
						},
					},
					{
						Name: "m2",
					},
					{
						Name: "m3",
					},
				},
			},
			want: []string{"m1"},
		},
		{
			name: "has nvidia gpu",
			args: args{
				groupName: "test",
				isMember: `for card in member.resources.gpu.cards:
	if card.driver == "nvidia":
		return True`,
				members: []ClusterMember{
					{
						Name: "m1",
						Resources: api.Resources{
							GPU: api.ResourcesGPU{
								Cards: []api.ResourcesGPUCard{
									{
										Driver: "nvidia",
									},
								},
							},
						},
					},
					{
						Name: "m2",
					},
					{
						Name: "m3",
					},
				},
			},
			want: []string{"m1"},
		},
	}
	for _, tt := range tests {
		ctx := context.Background()
		tester, err := NewClusterGroupMembershipTester(ctx, tt.args.groupName, tt.args.isMember)
		s.Require().NoError(err)

		got, err := tester.FilterMembers(ctx, tt.args.members)
		s.Require().NoError(err)

		s.Equal(tt.want, got)
	}
}
