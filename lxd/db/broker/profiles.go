package broker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type profiles struct {
	allLoaded bool

	// Map of project ID to boolean indicating if all profiles have been loaded for that project.
	allLoadedByProject map[int]bool

	// Map of profile ID to Profile
	profiles map[int]*Profile

	// Profile configurations
	config *Configs

	devices map[int][]ProfileDevice

	// DeviceConfigurations
	deviceConfig *Configs
}

type ProfileDevice struct {
	ProfileID int
	DeviceID  int
	Name      string
	Type      int
}

type Profile struct {
	ID          int
	Name        string
	ProjectID   int
	ProjectName string
	Description string
}

func (n Profile) DatabaseID() int {
	return n.ID
}

func (n Profile) EntityType() entity.Type {
	return entity.TypeProfile
}

func (n Profile) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n Profile) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "profiles", n.Name).Project(n.ProjectName)
}

type ProfileFull struct {
	Profile
	Config  map[string]string
	Devices map[string]map[string]string
}

func (p ProfileFull) ToAPI() api.Profile {
	return api.Profile{
		Name:        p.Name,
		Description: p.Description,
		Config:      p.Config,
		Devices:     p.Devices,
		Project:     p.ProjectName,
	}
}

func (g *Model) GetProfilesFullAllProjects(ctx context.Context) ([]ProfileFull, error) {
	if g.profiles.allLoaded {
		return g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
			return true
		})
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.profiles.loadAllFull(ctx, tx.Tx())
	})
	if err != nil {
		return nil, err
	}

	return g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
		return true
	})
}

func (g *Model) GetProfilesFullByProjectID(ctx context.Context, projectID int) ([]ProfileFull, error) {
	if g.profiles.allLoadedByProject[projectID] {
		return g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
			return profile.ProjectID == projectID
		})
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.profiles.loadFullByProjectID(ctx, tx.Tx(), projectID)
	})
	if err != nil {
		return nil, err
	}

	return g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
		return profile.ProjectID == projectID
	})
}

func (g *Model) GetProfilesFullByProfileID(ctx context.Context, profileIDs ...int) (map[int]ProfileFull, error) {
	profiles, err := g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
		return slices.Contains(profileIDs, profile.ID)
	})
	if err == nil && len(profiles) == len(profileIDs) {
		m := make(map[int]ProfileFull, len(profiles))
		for _, p := range profiles {
			m[p.ID] = p
		}

		return m, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.profiles.loadFullByID(ctx, tx.Tx(), profileIDs...)
	})
	if err != nil {
		return nil, err
	}

	profiles, err = g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
		return slices.Contains(profileIDs, profile.ID)
	})
	if err != nil {
		return nil, err
	}

	m := make(map[int]ProfileFull, len(profiles))
	for _, p := range profiles {
		m[p.ID] = p
	}

	return m, nil
}

func (g *Model) GetProfileByNameAndProjectID(ctx context.Context, name string, projectID int) (*Profile, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*Profile, error) {
		_, profile, err := shared.FilterMapOnceFunc(g.profiles.profiles, func(i int, profile *Profile) bool {
			return profile.Name == name && profile.ProjectID == projectID
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Profile not found")
			}

			return nil, nil
		}

		return profile, nil
	}

	profile, err := getFromCache(g.profiles.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if profile != nil {
		return profile, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = g.profiles.loadByName(ctx, tx.Tx(), projectID, name)
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

func (g *Model) GetProfileFullByNameAndProjectID(ctx context.Context, name string, projectID int) (*ProfileFull, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*ProfileFull, error) {
		cachedProfiles, err := g.profiles.getFromCacheFull(func(i int, profile *Profile) bool {
			return profile.Name == name && profile.ProjectID == projectID
		})
		if err != nil {
			return nil, err
		}

		if len(cachedProfiles) == 0 {
			if expectLoaded {
				return nil, api.StatusErrorf(http.StatusNotFound, "Profile not found")
			}

			return nil, nil
		} else if len(cachedProfiles) > 1 {
			return nil, fmt.Errorf("Found unexpected number of profiles")
		}

		return &cachedProfiles[0], nil
	}

	profile, err := getFromCache(g.profiles.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if profile != nil {
		return profile, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.profiles.loadFullByName(ctx, tx.Tx(), projectID, name)
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name, projectID)
}

func (p *profiles) getFromCacheFull(filter func(int, *Profile) bool) ([]ProfileFull, error) {
	profiles := shared.FilterMapFunc(p.profiles, filter)

	result := make([]ProfileFull, 0, len(profiles))
	for id, profile := range profiles {
		config, ok := p.config.configs[id]
		if !ok {
			return nil, fmt.Errorf("Missing config for profile %q", profile.Name)
		}

		devices, ok := p.devices[id]
		if !ok {
			return nil, fmt.Errorf("Missing devices for profile %q", profile.Name)
		}

		deviceMap := make(map[string]map[string]string)
		for _, d := range devices {
			deviceConfig, ok := p.deviceConfig.configs[d.DeviceID]
			if !ok {
				return nil, fmt.Errorf("Missing device config for profile %q", profile.Name)
			}

			deviceMap[d.Name] = deviceConfig
		}

		result = append(result, ProfileFull{
			Profile: *profile,
			Config:  config,
			Devices: deviceMap,
		})
	}

	return result, nil
}

func (p *profiles) initialiseIfNeeded() {
	if p.profiles == nil {
		p.profiles = make(map[int]*Profile)
	}

	if p.config == nil {
		p.config = &Configs{
			entityTable: "profiles",
			configTable: "profiles_config",
			foreignKey:  "profile_id",
		}
	}

	if p.allLoadedByProject == nil {
		p.allLoadedByProject = make(map[int]bool)
	}

	if p.deviceConfig == nil {
		p.deviceConfig = &Configs{
			configTable: "profiles_devices_config",
			entityTable: "profiles_devices",
			foreignKey:  "profile_device_id",
		}
	}
}

func (p *profiles) loadAllFull(ctx context.Context, tx *sql.Tx) error {
	p.initialiseIfNeeded()
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	for _, n := range p.profiles {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	err = p.config.load(ctx, tx)
	if err != nil {
		return err
	}

	_, err = p.loadProfileDevices(ctx, tx)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx)
	if err != nil {
		return err
	}

	return nil
}

func (p *profiles) loadProfileDevices(ctx context.Context, tx *sql.Tx, profileIDs ...int) ([]int, error) {
	q := `
SELECT 
	profiles_devices.id, 
	profiles_devices.profile_id, 
	profiles_devices.name, 
	profiles_devices.type 
FROM profiles_devices`

	if len(profileIDs) > 0 {
		q += ` WHERE profiles_devices.profile_id IN ` + query.IntParams(profileIDs...)
	}

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var profileDevice ProfileDevice
		err := scan(&profileDevice.DeviceID, &profileDevice.ProfileID, &profileDevice.Name, &profileDevice.Type)
		if err != nil {
			return err
		}

		ids = append(ids, profileDevice.DeviceID)
		p.devices[profileDevice.ProfileID] = append(p.devices[profileDevice.ProfileID], profileDevice)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func (p *profiles) loadFullByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	p.initialiseIfNeeded()

	profileIDs, err := p.loadBySQL(ctx, tx, "WHERE project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true

	err = p.config.load(ctx, tx, profileIDs...)
	if err != nil {
		return err
	}

	deviceIDs, err := p.loadProfileDevices(ctx, tx, profileIDs...)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx, deviceIDs...)
	if err != nil {
		return err
	}

	return nil
}

func (p *profiles) loadFullByID(ctx context.Context, tx *sql.Tx, profileIDs ...int) error {
	p.initialiseIfNeeded()

	_, err := p.loadBySQL(ctx, tx, "WHERE profiles.id IN "+query.IntParams(profileIDs...))
	if err != nil {
		return err
	}

	err = p.config.load(ctx, tx, profileIDs...)
	if err != nil {
		return err
	}

	deviceIDs, err := p.loadProfileDevices(ctx, tx, profileIDs...)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx, deviceIDs...)
	if err != nil {
		return err
	}

	return nil
}

func (p *profiles) loadFullByName(ctx context.Context, tx *sql.Tx, projectID int, profileNames ...string) error {
	p.initialiseIfNeeded()
	args := make([]any, 0, len(profileNames)+1)
	args = append(args, projectID)
	for name := range slices.Values(profileNames) {
		args = append(args, name)
	}

	sqlCondition := `WHERE profiles.project_id = ? AND profiles.name IN ` + query.Params(len(profileNames))
	profileIDs, err := p.loadBySQL(ctx, tx, sqlCondition, args...)
	if err != nil {
		return err
	}

	err = p.config.load(ctx, tx, profileIDs...)
	if err != nil {
		return err
	}

	deviceIDs, err := p.loadProfileDevices(ctx, tx, profileIDs...)
	if err != nil {
		return err
	}

	err = p.deviceConfig.load(ctx, tx, deviceIDs...)
	if err != nil {
		return err
	}

	return nil
}

func (p *profiles) loadAll(ctx context.Context, tx *sql.Tx) error {
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	for _, n := range p.profiles {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	return nil
}

func (p *profiles) loadByName(ctx context.Context, tx *sql.Tx, projectID int, profileNames ...string) error {
	args := make([]any, 0, len(profileNames)+1)
	args = append(args, projectID)
	for name := range slices.Values(profileNames) {
		args = append(args, name)
	}

	_, err := p.loadBySQL(ctx, tx, "WHERE profiles.project_id = ? AND profiles.name IN "+query.Params(len(profileNames)), args...)
	return err
}

func (p *profiles) loadByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	_, err := p.loadBySQL(ctx, tx, "WHERE profiles.project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true
	return nil
}

func (p *profiles) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]int, error) {
	p.initialiseIfNeeded()

	q := `
SELECT 
	profiles.id,
	profiles.name,
	profiles.project_id,
	projects.name,
	profiles.description
FROM profiles
` + sqlCondition + `
JOIN projects ON profiles.project_id = projects.id`

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		profile := Profile{}
		err := scan(&profile.ID, &profile.Name, &profile.ProjectID, &profile.ProjectName, &profile.Description)
		if err != nil {
			return err
		}

		ids = append(ids, profile.ID)
		p.profiles[profile.ID] = &profile
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load profiles: %w", err)
	}

	return ids, nil
}
