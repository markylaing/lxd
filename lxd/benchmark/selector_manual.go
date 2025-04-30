package benchmark

import (
	"context"
	"github.com/canonical/lxd/lxd/db"
)

func RunManual(instanceListGetter InstanceListGetter) ([]int, error) {
	instChan := make(chan Instance)
	errChan := make(chan error)
	go func() {
		errChan <- clusterDB.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
			return instanceListGetter.GetInstances(ctx, tx.Tx(), lookupProject, instChan)
		})
	}()

	lookupKey := lookupConfig[0]
	lookupValue := lookupConfig[1]
	var ids []int
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return nil, err
			}

			return ids, err
		case inst := <-instChan:
			if inst.ExpandedConfig[lookupKey] == lookupValue {
				ids = append(ids, inst.ID)
			}
		}
	}
}
