package cache

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/version"
)

type images struct {
	allLoaded bool
	images    map[int64]*Image
	mu        sync.RWMutex
}

type Image struct {
	ID           int64
	Fingerprint  string
	Filename     string
	Size         int64
	Public       bool
	Architecture int64
	CreationDate sql.NullTime
	ExpiryDate   sql.NullTime
	UploadDate   time.Time
	Cached       bool
	LastUseDate  sql.NullTime
	AutoUpdate   bool
	ProjectID    int64
	Type         int64
	ProjectName  string
}

func (n Image) DatabaseID() int64 {
	return n.ID
}

func (n Image) EntityType() entity.Type {
	return entity.TypeImage
}

func (n Image) Parent() auth.Entity {
	return projectEntity{id: n.ProjectID, name: n.ProjectName}
}

func (n Image) URL() *api.URL {
	return api.NewURL().Path(version.APIVersion, "images", n.Fingerprint).Project(n.ProjectName)
}

func GetAllImages(ctx context.Context) ([]Image, error) {
	c, err := cacheFromContext(ctx)
	if err != nil {
		return nil, err
	}

	c.images.init()
	c.images.mu.RLock()
	if c.images.allLoaded {
		defer c.images.mu.RUnlock()
		return datastructures.MapToSlice(c.images.images, func(_ int64, v *Image) (Image, error) {
			return *v, nil
		})
	}

	c.images.mu.RUnlock()
	c.images.mu.Lock()
	defer c.images.mu.Unlock()
	var ids []Image
	err = c.cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		ids, err = c.images.loadAll(ctx, tx.Tx())
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

func (p *images) init() {
	if p.images == nil {
		p.images = make(map[int64]*Image)
	}
}

func (p *images) loadAll(ctx context.Context, tx *sql.Tx) ([]Image, error) {
	p.init()
	result, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return nil, err
	}

	p.allLoaded = true
	return result, nil
}

func (p *images) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]Image, error) {
	p.init()

	q := `
SELECT 
	images.id,
	images.fingerprint,
	images.filename,
	images.size,
	images.public,
	images.architecture,
	images.creation_date,
	images.expiry_date,
	images.upload_date,
	images.cached,
	images.last_use_date,
	images.auto_update,
	images.project_id,
	images.type,
	projects.name
FROM images
JOIN projects ON images.project_id = projects.id
` + sqlCondition

	var rows []Image
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		image := Image{}
		err := scan(&image.ID, &image.Fingerprint, &image.Filename, &image.Size, &image.Public, &image.Architecture, &image.CreationDate, &image.ExpiryDate, &image.UploadDate, &image.Cached, &image.LastUseDate, &image.AutoUpdate, &image.ProjectID, &image.Type, &image.ProjectName)
		if err != nil {
			return err
		}

		p.images[image.ID] = &image
		rows = append(rows, image)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load images: %w", err)
	}

	return rows, nil
}
