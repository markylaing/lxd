package cache

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type profiles struct {
	allLoaded bool
	profiles  map[int64]*Profile
	mu        sync.RWMutex
}

type Profile struct {
	ID          int64
	Name        string
	Description string
	ProjectID   int64

	ProjectName string
}

func (n Profile) DatabaseID() int64 {
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

func GetAllProfiles(ctx context.Context) ([]Profile, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.profiles.init()
	c.profiles.mu.RLock()
	if c.profiles.allLoaded {
		defer c.profiles.mu.RUnlock()
		return datastructures.MapToSlice(c.profiles.profiles, func(_ int64, v *Profile) (Profile, error) {
			return *v, nil
		})
	}

	c.profiles.mu.RUnlock()
	c.profiles.mu.Lock()
	defer c.profiles.mu.Unlock()
	var ids []Profile
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.profiles.loadAll(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func (p *profiles) init() {
	if p.profiles == nil {
		p.profiles = make(map[int64]*Profile)
	}
}

func (p *profiles) loadAll(ctx context.Context, tx *sql.Tx) ([]Profile, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *profiles) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]Profile, error) {
	p.init()

	q := `
SELECT 
	profiles.id,
	profiles.name,
	profiles.description,
	profiles.project_id,
	projects.name
FROM profiles
JOIN projects ON profiles.project_id = projects.id
` + sqlCondition

	var rows []Profile
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		profile := Profile{}
		err := scan(&profile.ID, &profile.Name, &profile.Description, &profile.ProjectID, &profile.ProjectName)
		if err != nil {
			return err
		}

		p.profiles[profile.ID] = &profile
		rows = append(rows, profile)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load profiles: %w", err)
	}

	return rows, nil
}
