package broker

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"strconv"
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

type images struct {
	allLoaded bool

	// Map of project ID to boolean indicating if all images have been loaded for that project.
	allLoadedByProject map[int]bool

	// Map of image ID to Image
	images map[int]*Image

	// Map of image ID to list of properties
	properties map[int][]ImageProperty

	nodes map[int][]int

	profiles map[int][]int

	aliases map[int][]int

	sources map[int]*ImageSource
}

type ImageSource struct {
	ImageID     int
	Server      string
	Protocol    string
	Certificate string
	Alias       string
}

type ImageProperty struct {
	Type  int
	Key   string
	Value sql.NullString
}

type Image struct {
	ID           int
	Fingerprint  string
	ProjectID    int
	ProjectName  string
	Filename     string
	Size         int
	Public       bool
	Architecture int
	CreationDate sql.NullTime
	ExpiryDate   sql.NullTime
	UploadDate   time.Time
	Cached       bool
	LastUseDate  sql.NullTime
	AutoUpdate   bool
	Type         instancetype.Type
}

func (n Image) DatabaseID() int {
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

type ImageFull struct {
	Image
	Properties []ImageProperty
	Nodes      []int
	Profiles   []int
	Source     *ImageSource
	Aliases    []int
}

type ImageAlias struct {
	ID          int
	Name        string
	ProjectID   int
	ProjectName string
	ImageID     int
	Description string
}

func (p ImageFull) ToAPI(aliases map[int]ImageAlias, profiles map[int]string) (*api.Image, error) {
	apiAliases := make([]api.ImageAlias, 0, len(p.Aliases))
	for _, aliasID := range p.Aliases {
		alias, ok := aliases[aliasID]
		if !ok {
			return nil, fmt.Errorf("Missing aliases for image %q", p.Fingerprint)
		}

		apiAliases = append(apiAliases, api.ImageAlias{
			Name:        alias.Name,
			Description: alias.Description,
		})
	}

	archName, err := osarch.ArchitectureName(p.Architecture)
	if err != nil {
		return nil, err
	}

	profileNames := make([]string, 0, len(p.Profiles))
	for _, profileID := range p.Profiles {
		profile, ok := profiles[profileID]
		if !ok {
			return nil, fmt.Errorf("Missing profile for image %q", p.Fingerprint)
		}

		profileNames = append(profileNames, profile)
	}

	var updateSource *api.ImageSource
	if p.Source != nil {
		updateSource = &api.ImageSource{
			Alias:       p.Source.Alias,
			Certificate: p.Source.Certificate,
			Protocol:    p.Source.Protocol,
			Server:      p.Source.Server,
			ImageType:   p.Type.String(),
		}
	}

	properties := make(map[string]string, len(p.Properties))
	for _, property := range p.Properties {
		properties[property.Key] = property.Value.String
	}

	return &api.Image{
		Aliases:      apiAliases,
		Architecture: archName,
		Cached:       p.Cached,
		Public:       p.Public,
		Filename:     p.Filename,
		Fingerprint:  p.Fingerprint,
		Size:         int64(p.Size),
		UpdateSource: updateSource,
		AutoUpdate:   p.AutoUpdate,
		Type:         p.Type.String(),
		CreatedAt:    p.CreationDate.Time,
		ExpiresAt:    p.ExpiryDate.Time,
		LastUsedAt:   p.LastUseDate.Time,
		UploadedAt:   p.UploadDate,
		Properties:   properties,
		Profiles:     profileNames,
		Project:      p.ProjectName,
	}, nil
}

func (g *Model) GetImagesFullAllProjects(ctx context.Context) ([]ImageFull, error) {
	getFromCache := func() []ImageFull {
		images := make([]ImageFull, 0, len(g.images.images))
		for id, image := range g.images.images {
			images = append(images, ImageFull{
				Image:  *image,
				Config: g.images.config.configs[id],
				Nodes:  g.images.imageNodes[id],
			})
		}

		return images
	}

	if g.images.allLoaded {
		return getFromCache(), nil
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.images.loadAllFull(ctx, tx.Tx())
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(), nil
}

func (g *Model) GetImagesFullByProjectID(ctx context.Context, projectID int) ([]ImageFull, error) {
	getFromCache := func() []ImageFull {
		images := make([]ImageFull, 0, len(g.images.images))
		for id, image := range g.images.images {
			if image.ProjectID != projectID {
				continue
			}

			images = append(images, ImageFull{
				Image:  *image,
				Config: g.images.config.configs[id],
				Nodes:  g.images.imageNodes[id],
			})
		}

		return images
	}

	if g.images.allLoadedByProject[projectID] {
		return getFromCache(), nil
	}

	err := g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.images.loadFullByProjectID(ctx, tx.Tx(), projectID)
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(), nil
}

func (g *Model) GetImageByNameAndProjectID(ctx context.Context, name string, projectID int) (*Image, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*Image, error) {
		_, image, err := shared.FilterMapOnceFunc(g.images.images, func(i int, image *Image) bool {
			return image.Name == name && image.ProjectID == projectID
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Project not found")
			}

			return nil, nil
		}

		return image, nil
	}

	project, err := getFromCache(g.images.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if project != nil {
		return project, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		var err error
		err = g.images.loadByName(ctx, tx.Tx(), projectID, name)
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

func (g *Model) GetImageFullByNameAndProjectID(ctx context.Context, name string, projectID int) (*ImageFull, error) {
	getFromCache := func(expectLoaded bool, name string, projectID int) (*ImageFull, error) {
		_, image, err := shared.FilterMapOnceFunc(g.images.images, func(i int, image *Image) bool {
			return image.Name == name && image.ProjectID == projectID
		})
		if err != nil {
			if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			if expectLoaded {
				return nil, api.NewStatusError(http.StatusNotFound, "Image not found")
			}

			return nil, nil
		}

		config, ok := g.images.config.configs[image.ID]
		if !ok {
			return nil, nil
		}

		nodes, ok := g.images.imageNodes[image.ID]
		if !ok {
			return nil, nil
		}

		return &ImageFull{
			Image:  *image,
			Config: config,
			Nodes:  nodes,
		}, nil
	}

	image, err := getFromCache(g.images.allLoadedByProject[projectID], name, projectID)
	if err != nil {
		return nil, err
	}

	if image != nil {
		return image, nil
	}

	err = g.transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return g.images.loadFullByName(ctx, tx.Tx(), projectID, name)
	})
	if err != nil {
		return nil, err
	}

	return getFromCache(true, name, projectID)
}

func (p *images) initialiseIfNeeded() {
	if p.images == nil {
		p.images = make(map[int]*Image)
	}

	if p.config == nil {
		p.config = &Configs{
			entityTable: "images",
			configTable: "images_config",
			foreignKey:  "image_id",
		}
	}

	if p.allLoadedByProject == nil {
		p.allLoadedByProject = make(map[int]bool)
	}

	if p.imageNodes == nil {
		p.imageNodes = make(map[int][]ImageNode)
	}
}

func (p *images) loadAllFull(ctx context.Context, tx *sql.Tx) error {
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	p.allLoaded = true
	for _, n := range p.images {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	err = p.config.load(ctx, tx)
	if err != nil {
		return err
	}

	err = p.loadImageNodes(ctx, tx)
	if err != nil {
		return err
	}

	return nil
}

func (p *images) loadImageNodes(ctx context.Context, tx *sql.Tx, imageIDs ...int) error {
	q := `SELECT image_id, node_id, state FROM images_nodes`

	if len(imageIDs) > 0 {
		q += " WHERE image_id IN " + query.IntParams(imageIDs...)
	}

	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var nn ImageNode
		err := scan(&nn.ImageID, &nn.NodeID, &nn.State)
		if err != nil {
			return err
		}

		p.imageNodes[nn.ImageID] = append(p.imageNodes[nn.ImageID], nn)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *images) loadFullByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	ids, err := p.loadBySQL(ctx, tx, "WHERE images.project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true
	err = p.config.load(ctx, tx, ids...)
	if err != nil {
		return err
	}

	err = p.loadImageNodes(ctx, tx, ids...)
	if err != nil {
		return err
	}

	return nil
}

func (p *images) loadFullByFingerprintPrefix(ctx context.Context, tx *sql.Tx, projectID int, imageFingerprintPrefix string) error {
	ids, err := p.loadByFingerprintPrefix(ctx, tx, projectID, imageFingerprintPrefix)

	return nil
}

func (p *images) loadAll(ctx context.Context, tx *sql.Tx) error {
	_, err := p.loadBySQL(ctx, tx, "")
	if err != nil {
		return err
	}

	for _, n := range p.images {
		_, ok := p.allLoadedByProject[n.ProjectID]
		if !ok {
			p.allLoadedByProject[n.ProjectID] = true
		}
	}

	return nil
}

func (p *images) loadByFingerprintPrefix(ctx context.Context, tx *sql.Tx, projectID int, imageFingerprintPrefix string) ([]int, error) {
	return p.loadBySQL(ctx, tx, "WHERE images.project_id = ? AND images.fingerprint LIKE ? ", projectID, imageFingerprintPrefix+"%")
}

func (p *images) loadByProjectID(ctx context.Context, tx *sql.Tx, projectID int) error {
	_, err := p.loadBySQL(ctx, tx, "WHERE images.project_id = ?", projectID)
	if err != nil {
		return err
	}

	p.allLoadedByProject[projectID] = true
	return nil
}

func (p *images) loadBySQL(ctx context.Context, tx *sql.Tx, sqlCondition string, args ...any) ([]int, error) {
	p.initialiseIfNeeded()

	q := `
SELECT 
	images.id,
	images.fingerprint,
	images.project_id,
	projects.name,
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
	images.type
FROM images
` + sqlCondition + `
JOIN projects ON images.project_id = projects.id`

	var ids []int
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		image := Image{}
		err := scan(&image.ID, &image.Fingerprint, &image.ProjectID, &image.ProjectName, &image.Filename, &image.Size, &image.Public, &image.Architecture, &image.CreationDate, &image.ExpiryDate, &image.UploadDate, &image.Cached, &image.LastUseDate, &image.AutoUpdate, &image.Type)
		if err != nil {
			return err
		}

		ids = append(ids, image.ID)
		p.images[image.ID] = &image
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to load images: %w", err)
	}

	return ids, nil
}
