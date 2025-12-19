package cache

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/shared/datastructures"
)

const ctxClusterCache request.CtxKey = "cluster_cache"

func Initialize(r *http.Request, cluster *db.Cluster) *Cache {
	model := &Cache{
		cluster: cluster,
		projects: projects{
			allLoaded: atomic.Bool{},
			projects:  datastructures.SyncMap[int64, *Project]{},
			config: config{
				configTable: "projects_config",
				entityTable: "projects",
				foreignKey:  "project_id",
			},
		},
	}

	request.SetContextValue(r, ctxClusterCache, model)
	return model
}

func cacheFromContext(ctx context.Context) (*Cache, error) {
	c, err := request.GetContextValue[*Cache](ctx, ctxClusterCache)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type Cache struct {
	cluster         *db.Cluster
	projects        projects
	authGroups      authGroups
	idpGroups       idpGroups
	identities      identities
	instances       instances
	profiles        profiles
	images          images
	networks        networks
	networkACLs     networkACLs
	placementGroups placementGroups
	storageVolumes  storageVolumes
	storageBuckets  storageBuckets
}
