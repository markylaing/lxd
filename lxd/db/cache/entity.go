package cache

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type projectEntity struct {
	id   int64
	name string
}

func (p projectEntity) EntityType() entity.Type {
	return entity.TypeProject
}

func (p projectEntity) DatabaseID() int64 {
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

func (s serverEntity) DatabaseID() int64 {
	return 0
}

func (s serverEntity) Parent() auth.Entity {
	return nil
}

func (p serverEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion)
}

type instanceEntity struct {
	id          int64
	name        string
	projectID   int64
	projectName string
}

func (s instanceEntity) EntityType() entity.Type {
	return entity.TypeInstance
}

func (s instanceEntity) DatabaseID() int64 {
	return s.id
}

func (s instanceEntity) Parent() auth.Entity {
	return projectEntity{id: s.projectID, name: s.projectName}
}

func (s instanceEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "instances", s.name).Project(s.projectName)
}

type storageVolumeEntity struct {
	id          int64
	name        string
	projectID   int64
	projectName string
	volTypeName string
	poolName    string
	nodeName    string
}

func (s storageVolumeEntity) EntityType() entity.Type {
	return entity.TypeInstance
}

func (s storageVolumeEntity) DatabaseID() int64 {
	return s.id
}

func (s storageVolumeEntity) Parent() auth.Entity {
	return projectEntity{id: s.projectID, name: s.projectName}
}

func (s storageVolumeEntity) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "storage-pools", s.poolName, "volumes", s.volTypeName, s.name).Project(s.projectName).Target(s.nodeName)
}

func getEntityFromMap[V auth.Entity](mu *sync.RWMutex, id int64, m map[int64]V) (auth.Entity, bool) {
	mu.RLock()
	defer mu.RUnlock()
	e, ok := m[id]
	return e, ok
}

func getEntityFromSyncMap[V auth.Entity](id int64, m *datastructures.SyncMap[int64, V]) (auth.Entity, bool) {
	return m.Get(true, id)
}

func GetEntityByID(ctx context.Context, entityType entity.Type, entityID int64) (auth.Entity, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var e auth.Entity
	var ok bool
	switch entityType {
	case entity.TypeServer:
		return serverEntity{}, nil
	case entity.TypeAuthGroup:
		e, ok = getEntityFromSyncMap(entityID, &c.authGroups.groups)
	case entity.TypeProject:
		e, ok = getEntityFromMap(&c.projects.mu, entityID, c.projects.projects)
	case entity.TypeInstance:
		e, ok = getEntityFromMap(&c.instances.mu, entityID, c.instances.instances)
	case entity.TypeImage:
		e, ok = getEntityFromMap(&c.images.mu, entityID, c.images.images)
	case entity.TypeNetwork:
		e, ok = getEntityFromMap(&c.networks.mu, entityID, c.networks.networks)
	case entity.TypeNetworkACL:
		e, ok = getEntityFromMap(&c.networkACLs.mu, entityID, c.networkACLs.networkACLs)
	case entity.TypePlacementGroup:
		e, ok = getEntityFromMap(&c.placementGroups.mu, entityID, c.placementGroups.placementGroups)
	case entity.TypeProfile:
		e, ok = getEntityFromMap(&c.profiles.mu, entityID, c.profiles.profiles)
	case entity.TypeStorageBucket:
		e, ok = getEntityFromMap(&c.storageBuckets.mu, entityID, c.storageBuckets.storageBuckets)
	case entity.TypeStorageVolume:
		e, ok = getEntityFromMap(&c.storageVolumes.mu, entityID, c.storageVolumes.storageVolumes)
	default:
		return nil, api.StatusErrorf(http.StatusNotImplemented, "Cache doesn't handle entities of type %q yet", entityType)
	}

	if !ok {
		return nil, fmt.Errorf("Entity of type %q with ID %d not loaded in cache", entityType, entityID)
	}

	return e, nil
}

var count = 0

func GetChildEntities(ctx context.Context, parentEntityType entity.Type, parentEntityID int64, childEntityType entity.Type) ([]auth.Entity, error) {
	count++
	switch parentEntityType {
	case entity.TypeServer:
		switch childEntityType {
		case entity.TypeProject:
			projects, err := GetAllProjects(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSlice(projects, func(i int, e Project) (auth.Entity, error) {
				return e, nil
			})
		}
	case entity.TypeProject:
		switch childEntityType {
		case entity.TypeInstance:
			instances, err := GetAllInstances(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(instances, func(i int, e Instance) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e Instance) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypeProfile:
			profiles, err := GetAllProfiles(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(profiles, func(i int, e Profile) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e Profile) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypeImage:
			images, err := GetAllImages(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(images, func(i int, e Image) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e Image) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypeStorageVolume:
			storageVolumes, err := GetAllStorageVolumes(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(storageVolumes, func(i int, e StorageVolume) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e StorageVolume) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypeNetwork:
			networks, err := GetAllNetworks(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(networks, func(i int, e Network) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e Network) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypeNetworkACL:
			networkACLs, err := GetAllNetworkACLs(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(networkACLs, func(i int, e NetworkACL) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e NetworkACL) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypeStorageBucket:
			storageBuckets, err := GetAllStorageBuckets(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(storageBuckets, func(i int, e StorageBucket) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e StorageBucket) (auth.Entity, error) {
				return e, nil
			})
		case entity.TypePlacementGroup:
			placementGroups, err := GetAllPlacementGroups(ctx)
			if err != nil {
				return nil, err
			}

			return datastructures.SliceToSliceFilter(placementGroups, func(i int, e PlacementGroup) (bool, error) {
				return e.ProjectID == parentEntityID, nil
			}, func(i int, e PlacementGroup) (auth.Entity, error) {
				return e, nil
			})
		}
	}

	return nil, api.StatusErrorf(http.StatusNotImplemented, "Not implemented yet")
}
