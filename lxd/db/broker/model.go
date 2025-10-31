package broker

import (
	"context"
	"net/http"

	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/request"
)

const ctxBrokerModel request.CtxKey = "broker_model"

func InitialiseModel(r *http.Request, cluster *db.Cluster) *Model {
	model := &Model{
		cluster: cluster,
	}

	request.SetContextValue(r, ctxBrokerModel, model)
	return model
}

func GetModelFromContext(ctx context.Context) (*Model, error) {
	return request.GetContextValue[*Model](ctx, ctxBrokerModel)
}

type Model struct {
	cluster    *db.Cluster
	projects   projects
	networks   networks
	authGroups authGroups
	identities identities
	instances  instances
	profiles   profiles
}

func (g *Model) transaction(ctx context.Context, f func(ctx context.Context, tx *db.ClusterTx) error) error {
	tx, err := request.GetContextValue[*db.ClusterTx](ctx, request.CtxClusterTx)
	if err == nil {
		return f(ctx, tx)
	}

	return g.cluster.Transaction(ctx, f)
}
