package benchmark

import (
	"context"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
)

func RunQuery() ([]int, error) {
	var ids []int
	var err error
	err = clusterDB.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
		ids, err = cluster.RunSelector(ctx, tx.Tx(), cluster.Selector{
			EntityType: cluster.EntityType(entity.TypeInstance),
			Matchers: []api.SelectorMatcher{
				{
					Property: "project",
					Values:   []string{lookupProject},
				},
				{
					Property: "config." + lookupConfig[0],
					Values:   []string{lookupConfig[1]},
				},
			},
		})
		return err
	})
	return ids, err
}
