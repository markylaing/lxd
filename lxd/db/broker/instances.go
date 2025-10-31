package broker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/lxd/instance/instancetype"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/osarch"
	"github.com/canonical/lxd/shared/version"
)

type instances struct {
	allLoaded bool

	// Map of project ID to boolean indicating if all instances have been loaded for that project.
	allLoadedByProject map[int]bool

	// Map of instance ID to Instance
	instances map[int]*Instance

	// Instance configurations
	config *Configs

	// Map of instance ID to list of profile IDs
	profiles map[int][]InstanceProfile

	devices map[int][]InstanceDevice

	// DeviceConfigurations
	deviceConfig *Configs
}

type InstanceDevice struct {
	InstanceID int
	DeviceID   int
	Name       string
	Type       int
}

type InstanceProfile struct {
	InstanceID int
	ProfileID  int
	ApplyOrder int
}

type Instance struct {
	ID           int
	Name         string
	NodeID       int
	NodeName     string
	ProjectID    int
	ProjectName  string
	Architecture int
	Type         instancetype.Type
	Ephemeral    bool
	CreationDate time.Time
	Stateful     bool
	LastUseDate  sql.NullTime
	Description  string
	ExpiryDate   sql.NullTime
}

func (n Instance) DatabaseID() int {
	return n.ID
}

func (n Instance) EntityType() entity.Type {
	return entity.TypeInstance
}

func (n Instance) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n Instance) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "instances", n.Name).Project(n.ProjectName)
}

type InstanceFull struct {
	Instance
	Config   map[string]string
	Profiles []InstanceProfile
	Devices  map[string]map[string]string
}

func (p InstanceFull) ToAPI(statusMessage string, statusCode api.StatusCode, profiles map[int]ProfileFull) (*api.Instance, error) {
	archName, err := osarch.ArchitectureName(p.Architecture)
	if err != nil {
		return nil, err
	}

	// Ensure profiles are in apply order
	instanceProfiles := slices.SortedFunc(slices.Values(p.Profiles), func(e InstanceProfile, e2 InstanceProfile) int {
		return e.ApplyOrder - e2.ApplyOrder
	})

	expandedConfig := make(map[string]string)
	expandedDevices := make(map[string]map[string]string)
	profileNames := make([]string, 0, len(p.Profiles))
	for _, ip := range instanceProfiles {
		pf, ok := profiles[ip.ProfileID]
		if !ok {
			return nil, fmt.Errorf("Profile with ID %d not loaded for instance %q", ip.ProfileID)
		}

		profileNames = append(profileNames, pf.Name)

		for k, v := range pf.Config {
			expandedConfig[k] = v
		}

		for k1, m := range pf.Devices {
			_, ok := expandedDevices[k1]
			if !ok {
				expandedDevices[k1] = m
				continue
			}

			for k2, v := range m {
				expandedDevices[k1][k2] = v
			}
		}
	}

	return &api.Instance{
		WithEntitlements: api.WithEntitlements{},
		Name:             p.Name,
		Description:      p.Description,
		Status:           statusMessage,
		StatusCode:       statusCode,
		CreatedAt:        p.CreationDate,
		LastUsedAt:       p.LastUseDate.Time,
		Location:         p.NodeName,
		Type:             p.Type.String(),
		Project:          p.ProjectName,
		Architecture:     archName,
		Ephemeral:        p.Ephemeral,
		Stateful:         p.Stateful,
		Profiles:         profileNames,
		Config:           p.Config,
		Devices:          p.Devices,
		ExpandedConfig:   expandedConfig,
		ExpandedDevices:  expandedDevices,
	}, nil
}

func (g *Model) GetInstancesFullAllProjects(ctx context.Context) ([]InstanceFull, error) {
	if g.instances.allLoaded {
		return g.instances.getFromCacheFull(func(i int, instance *Instance) bool {
			return true
		})
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.instances.loadAllFull(ctx, tx.Tx())
	})
	if err != nil {
		return nil, err
	}

	return g.instances.getFromCacheFull(func(i int, instance *Instance) bool {
		return true
	})
}

func (g *Model) GetInstancesFullByProjectID(ctx context.Context, projectID int) ([]InstanceFull, error) {
	if g.instances.allLoadedByProject[projectID] {
		return g.instances.getFromCacheFull(func(i int, instance *Instance) bool {
			return instance.ProjectID == projectID
		})
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.instances.loadFullByProjectID(ctx, tx.Tx(), projectID)
	})
	if err != nil {
		return nil, err
	}

	return g.instances.getFromCacheFull(func(i int, instance *Instance) bool {
		return instance.ProjectID == projectID
	})
}

func (g *Model) GetInstanceByNameAndProjectID(ctx context.Context, name string, projectID int) (*Instance, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*Instance, error) {
		_, instance, err := shared.FilterMapOnceFunc(g.instances.instances, func(i int, instance *Instance) bool {
			return instance.Name == name && instance.ProjectID == projectID
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Instance not found")
			}

			return nil, nil
		}

		return instance, nil
	}

	instance, err := getFromCache(g.instances.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if instance != nil {
		return instance, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = g.instances.loadByName(ctx, tx.Tx(), projectID, name)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name, projectID)
}

func (g *Model) GetInstanceFullByNameAndProjectID(ctx context.Context, name string, projectID int) (*InstanceFull, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*InstanceFull, error) {
		cachedInstances, err := g.instances.getFromCacheFull(func(i int, instance *Instance) bool {
			return instance.Name == name && instance.ProjectID == projectID
		})
		if err != nil {
			return nil, err
		}

		if len(cachedInstances) == 0 {
			if expectLoaded {
				return nil, api.StatusErrorf(http.StatusNotFound, "Instance not found")
			}

			return nil, nil
		} else if len(cachedInstances) > 1 {
			return nil, fmt.Errorf("Found unexpected number of instances")
		}

		return &cachedInstances[0], nil
	}

	instance, err := getFromCache(g.instances.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if instance != nil {
		return instance, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.instances.loadFullByName(ctx, tx.Tx(), projectID, name)
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name, projectID)
}

func (p *instances) getFromCacheFull(filter func(int, *Instance) bool) ([]InstanceFull, error) {
	instances := shared.FilterMapFunc(p.instances, filter)

	result := make([]InstanceFull, 0, len(instances))
	for id, instance := range instances {
		config, ok := p.config.configs[id]
		if !ok {
			return nil, fmt.Errorf("Missing config for instance %q", instance.Name)
		}

		profiles, ok := p.profiles[id]
		if !ok {
			return nil, fmt.Errorf("Missing profiles for instance %q", instance.Name)
		}

		devices, ok := p.devices[id]
		if !ok {
			return nil, fmt.Errorf("Missing devices for instance %q", instance.Name)
		}

		deviceMap := make(map[string]map[string]string)
		for _, d := range devices {
			deviceConfig, ok := p.deviceConfig.configs[d.DeviceID]
			if !ok {
				return nil, fmt.Errorf("Missing device config for instance %q", instance.Name)
			}

			deviceMap[d.Name] = deviceConfig
		}

		result = append(result, InstanceFull{
			Instance: *instance,
			Config:   config,
			Profiles: profiles,
			Devices:  deviceMap,
		})
	}

	return result, nil
}

func (p *instances) initialiseIfNeeded() {
	if p.instances == nil {
		p.instances = make(map[int]*Instance)
	}

	if p.config == nil {
		p.config = &Configs{
			entityTable: "instances",
			configTable: "instances_config",
			foreignKey:  "instance_id",
		}
	}

	if p.allLoadedByProject == nil {
		p.allLoadedByProject = make(map[int]bool)
	}

	if p.profiles == nil {
		p.profiles = make(map[int][]InstanceProfile)
	}

	if p.deviceConfig == nil {
		p.deviceConfig = &Configs{
			configTable: "instances_devices_config",
			entityTable: "instances_devices",
			foreignKey:  "instance_device_id",
		}
	}
}

func (p *instances) loadAllFull(ctx context.Context, tx *sql.Tx) error {
	p.initialiseIfNeeded()
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	for _, n := range p.instances {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	err = p.config.load(ctx, tx)
	if err != nil {
		return err
	}

	err = p.loadInstanceProfiles(ctx, tx)
	if err != nil {
		return err
	}

	_, err = p.loadInstanceDevices(ctx, tx)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx)
	if err != nil {
		return err
	}

	return nil
}

func (p *instances) loadInstanceProfiles(ctx context.Context, tx *sql.Tx, instanceIDs ...int) error {
	q := `SELECT instance_id, profile_id, apply_order FROM instances_profiles`
	if len(instanceIDs) > 0 {
		q += " WHERE instance_id IN " + query.IntParams(instanceIDs...)
	}

	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var ip InstanceProfile
		var id int
		err := scan(&id, &ip.ProfileID, &ip.ApplyOrder)
		if err != nil {
			return err
		}

		p.profiles[id] = append(p.profiles[id], ip)
		return nil
	})
	if err != nil {
		return err
	}

	// Always keep profiles in apply order.
	for id, ip := range p.profiles {
		p.profiles[id] = slices.SortedFunc(slices.Values(ip), func(e InstanceProfile, e2 InstanceProfile) int {
			return e.ApplyOrder - e2.ApplyOrder
		})
	}

	return nil
}

func (p *instances) loadInstanceDevices(ctx context.Context, tx *sql.Tx, instanceIDs ...int) ([]int, error) {
	q := `
SELECT 
	instances_devices.id, 
	instances_devices.instance_id, 
	instances_devices.name, 
	instances_devices.type 
FROM instances_devices`

	if len(instanceIDs) > 0 {
		q += ` WHERE instances_devices.instance_id IN ` + query.IntParams(instanceIDs...)
	}

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var instanceDevice InstanceDevice
		err := scan(&instanceDevice.DeviceID, &instanceDevice.InstanceID, &instanceDevice.Name, &instanceDevice.Type)
		if err != nil {
			return err
		}

		ids = append(ids, instanceDevice.DeviceID)
		p.devices[instanceDevice.InstanceID] = append(p.devices[instanceDevice.InstanceID], instanceDevice)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func (p *instances) loadFullByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	p.initialiseIfNeeded()

	instanceIDs, err := p.loadBySQL(ctx, tx, "WHERE project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true

	err = p.config.load(ctx, tx, instanceIDs...)
	if err != nil {
		return err
	}

	err = p.loadInstanceProfiles(ctx, tx, instanceIDs...)
	if err != nil {
		return err
	}

	deviceIDs, err := p.loadInstanceDevices(ctx, tx, instanceIDs...)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx, deviceIDs...)
	if err != nil {
		return err
	}

	return nil
}

func (p *instances) loadFullByName(ctx context.Context, tx *sql.Tx, projectID int, instanceNames ...string) error {
	p.initialiseIfNeeded()
	args := make([]any, 0, len(instanceNames)+1)
	args = append(args, projectID)
	for name := range slices.Values(instanceNames) {
		args = append(args, name)
	}

	sqlCondition := `WHERE instances.project_id = ? AND instances.name IN ` + query.Params(len(instanceNames))
	instanceIDs, err := p.loadBySQL(ctx, tx, sqlCondition, args...)
	if err != nil {
		return err
	}

	err = p.config.load(ctx, tx, instanceIDs...)
	if err != nil {
		return err
	}

	err = p.loadInstanceProfiles(ctx, tx, instanceIDs...)
	if err != nil {
		return err
	}

	deviceIDs, err := p.loadInstanceDevices(ctx, tx, instanceIDs...)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx, deviceIDs...)
	if err != nil {
		return err
	}

	return nil
}

func (p *instances) loadAll(ctx context.Context, tx *sql.Tx) error {
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	for _, n := range p.instances {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	return nil
}

func (p *instances) loadByName(ctx context.Context, tx *sql.Tx, projectID int, instanceNames ...string) error {
	args := make([]any, 0, len(instanceNames)+1)
	args = append(args, projectID)
	for name := range slices.Values(instanceNames) {
		args = append(args, name)
	}

	_, err := p.loadBySQL(ctx, tx, "WHERE instances.project_id = ? AND instances.name IN "+query.Params(len(instanceNames)), args...)
	return err
}

func (p *instances) loadByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	_, err := p.loadBySQL(ctx, tx, "WHERE instances.project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true
	return nil
}

func (p *instances) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]int, error) {
	p.initialiseIfNeeded()

	q := `
SELECT 
	instances.id,
	instances.name,
	instances.node_id,
	nodes.name,
	instances.project_id,
	projects.name,
	instances.architecture, 
	instances.type,
	instances.ephemeral,
	instances.creation_date,
	instances.stateful,
	instances.last_use_date,
	instances.description,
	instances.expiry_date
FROM instances
` + sqlCondition + `
JOIN projects ON instances.project_id = projects.id
JOIN nodes ON instances.node_id = nodes.id`

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		instance := Instance{}
		err := scan(&instance.ID, &instance.Name, &instance.NodeID, &instance.NodeName, &instance.ProjectID, &instance.ProjectName, &instance.Architecture, &instance.Type, &instance.Ephemeral, &instance.CreationDate, &instance.Stateful, &instance.LastUseDate, &instance.Description, &instance.ExpiryDate)
		if err != nil {
			return err
		}

		ids = append(ids, instance.ID)
		p.instances[instance.ID] = &instance
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load instances: %w", err)
	}

	return ids, nil
}
