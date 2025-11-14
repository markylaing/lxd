package broker

import (
	"context"
	"net/http"

	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

const ctxBrokerModel request.CtxKey = "broker_model"

func Initialize(r *http.Request, cluster *db.Cluster) *Model {
	model := &Model{
		cluster: cluster,
	}

	request.SetContextValue(r, ctxBrokerModel, model)
	return model
}

func getModelFromContext(ctx context.Context) (*Model, error) {
	return request.GetContextValue[*Model](ctx, ctxBrokerModel)
}

type Model struct {
	cluster  *db.Cluster
	projects projects
}

type Reference interface {
	DatabaseID() int
	EntityType() entity.Type
	Parent() Reference
	URL() *api.URL
}

type serverReference struct{}

func (serverReference) DatabaseID() int {
	return 0
}

func (serverReference) EntityType() entity.Type {
	return entity.TypeServer
}

func (serverReference) Parent() Reference {
	return nil
}

func (serverReference) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion)
}
