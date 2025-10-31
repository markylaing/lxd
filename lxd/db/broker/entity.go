package broker

import (
	"context"
	"fmt"
	"net/http"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type projectEntity struct {
	id   int
	name string
}

func (p projectEntity) EntityType() entity.Type {
	return entity.TypeProject
}

func (p projectEntity) DatabaseID() int {
	return p.id
}

func (p projectEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "projects", p.name)
}

func (p projectEntity) Parent() auth.Entity {
	return serverEntity{}
}

type serverEntity struct{}

func (s serverEntity) EntityType() entity.Type {
	return entity.TypeServer
}

func (s serverEntity) DatabaseID() int {
	return 0
}

func (s serverEntity) Parent() auth.Entity {
	return nil
}

func (p serverEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion)
}

type instanceEntity struct {
	id          int
	name        string
	projectID   int
	projectName string
}

func (s instanceEntity) EntityType() entity.Type {
	return entity.TypeInstance
}

func (s instanceEntity) DatabaseID() int {
	return s.id
}

func (s instanceEntity) Parent() auth.Entity {
	return projectEntity{id: s.projectID, name: s.projectName}
}

func (s instanceEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "instances", s.name).Project(s.projectName)
}

type storageVolumeEntity struct {
	id          int
	name        string
	projectID   int
	projectName string
	volTypeName string
	poolID      int
	poolName    string
	nodeID      int
	nodeName    string
}

func (s storageVolumeEntity) EntityType() entity.Type {
	return entity.TypeInstance
}

func (s storageVolumeEntity) DatabaseID() int {
	return s.id
}

func (s storageVolumeEntity) Parent() auth.Entity {
	return projectEntity{id: s.projectID, name: s.projectName}
}

func (s storageVolumeEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "storage-pools", s.poolName, "volumes", s.volTypeName, s.name).Project(s.projectName).Target(s.nodeName)
}

func (m *Model) GetEntityByID(entityType entity.Type, entityID int) (auth.Entity, error) {
	var e auth.Entity
	var ok bool
	switch entityType {
	case entity.TypeServer:
		e = serverEntity{}
		ok = true
	case entity.TypeNetwork:
		e, ok = m.networks.networks[entityID]
	case entity.TypeProject:
		e, ok = m.projects.projects[entityID]
	case entity.TypeInstance:
		e, ok = m.instances.instances[entityID]
	default:
		return nil, api.StatusErrorf(http.StatusNotImplemented, "Model doesn't handle entities of type %q yet", entityType)
	}

	if !ok {
		return nil, fmt.Errorf("Entity of type %q with ID %d not loaded in cache", entityType, entityID)
	}

	return e, nil
}

func (m *Model) GetChildEntities(ctx context.Context, parentEntityType entity.Type, parentEntityID int, childEntityType entity.Type) ([]auth.Entity, error) {
	var entities []auth.Entity
	var err error
	switch parentEntityType {
	case entity.TypeServer:
		switch childEntityType {
		case entity.TypeCertificate:
		case entity.TypeIdentity:
		case entity.TypeIdentityProviderGroup:
		case entity.TypeAuthGroup:
		case entity.TypeStoragePool:
		case entity.TypeProject:
			if !m.projects.allLoaded {
				err = m.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
					return m.projects.loadAll(ctx, tx.Tx())
				})
				if err != nil {
					return nil, err
				}
			}

			entities = make([]auth.Entity, 0, len(m.projects.projects))
			for id := range m.projects.projects {
				entities = append(entities, projectEntity{id: id})
			}
		}
	case entity.TypeProject:
		switch childEntityType {
		case entity.TypeImage:
		case entity.TypeImageAlias:
		case entity.TypeInstance:
		case entity.TypeNetwork:
			if !m.networks.allLoadedByProject[parentEntityID] {
				err = m.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
					return m.networks.loadByProjectID(ctx, tx.Tx(), parentEntityID)
				})
				if err != nil {
					return nil, err
				}
			}

			entities = make([]auth.Entity, 0, len(m.networks.networks))
			for _, network := range m.networks.networks {
				if network.ProjectID == parentEntityID {
					entities = append(entities, network)
				}
			}
		case entity.TypeNetworkZone:
		case entity.TypeNetworkACL:
		case entity.TypePlacementGroup:
		case entity.TypeStorageVolume:
		case entity.TypeStorageBucket:
		case entity.TypeProfile:
		}
	case entity.TypeInstance:
		switch childEntityType {
		case entity.TypeInstanceSnapshot:
		case entity.TypeInstanceBackup:
		}
	case entity.TypeStorageVolume:
		switch childEntityType {
		case entity.TypeStorageVolumeSnapshot:
		case entity.TypeStorageVolumeBackup:
		}
	}

	if err != nil {
		return nil, err
	}

	if entities == nil {
		return nil, api.StatusErrorf(http.StatusNotImplemented, "Not implemented yet")
	}

	return entities, nil
}
