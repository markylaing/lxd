package auth

import (
	"context"
	"fmt"
	"github.com/canonical/lxd/lxd/entity"
	"net/http"

	"github.com/openfga/openfga/pkg/storage"

	"github.com/canonical/lxd/lxd/certificate"
	"github.com/canonical/lxd/shared/logger"
)

const (
	// DriverTLS is the default TLS authorization driver. It is not compatible with OIDC or Candid authentication.
	DriverTLS string = "tls"

	// DriverRBAC is role-based authorization. It is not compatible with TLS authentication.
	DriverRBAC string = "rbac"

	// DriverOpenFGA provides fine-grained authorization. It is compatible with any authentication method.
	DriverOpenFGA string = "openfga"

	// DriverOpenFGAEmbedded provides fine-grained authorization built in to LXD. It is compatible with any authentication method.
	DriverOpenFGAEmbedded string = "openfga-embedded"
)

// ErrUnknownDriver is the "Unknown driver" error.
var ErrUnknownDriver = fmt.Errorf("Unknown driver")

var authorizers = map[string]func() authorizer{
	DriverTLS: func() authorizer { return &tls{} },
	DriverRBAC: func() authorizer {
		return &rbac{
			resources:   map[string]string{},
			permissions: map[string]map[string][]rbacPermission{},
		}
	},
	DriverOpenFGAEmbedded: func() authorizer {
		return &embeddedOpenFGA{}
	},
}

type authorizer interface {
	Authorizer

	init(driverName string, logger logger.Logger) error
	load(ctx context.Context, certificateCache *certificate.Cache, opts Opts) error
}

// PermissionChecker is a type alias for a function that returns whether a user has required permissions on an object.
// It is returned by Authorizer.GetPermissionChecker.
type PermissionChecker func(object string) bool

// Authorizer is the primary external API for this package.
type Authorizer interface {
	Driver() string
	StopService(ctx context.Context) error

	CheckPermission(ctx context.Context, req *http.Request, relation entity.Entitlement, entityType entity.Type, projectName string, location string, pathArgs ...string) error
	GetPermissionChecker(ctx context.Context, r *http.Request, relation entity.Entitlement, entityType entity.Type) (PermissionChecker, error)

	AddProject(ctx context.Context, projectID int64, projectName string) error
	DeleteProject(ctx context.Context, projectID int64, projectName string) error
	RenameProject(ctx context.Context, projectID int64, oldName string, newName string) error

	AddCertificate(ctx context.Context, fingerprint string) error
	DeleteCertificate(ctx context.Context, fingerprint string) error

	AddStoragePool(ctx context.Context, storagePoolName string) error
	DeleteStoragePool(ctx context.Context, storagePoolName string) error

	AddImage(ctx context.Context, projectName string, fingerprint string) error
	DeleteImage(ctx context.Context, projectName string, fingerprint string) error

	AddImageAlias(ctx context.Context, projectName string, imageAliasName string) error
	DeleteImageAlias(ctx context.Context, projectName string, imageAliasName string) error
	RenameImageAlias(ctx context.Context, projectName string, oldAliasName string, newAliasName string) error

	AddInstance(ctx context.Context, projectName string, instanceName string) error
	DeleteInstance(ctx context.Context, projectName string, instanceName string) error
	RenameInstance(ctx context.Context, projectName string, oldInstanceName string, newInstanceName string) error

	AddNetwork(ctx context.Context, projectName string, networkName string) error
	DeleteNetwork(ctx context.Context, projectName string, networkName string) error
	RenameNetwork(ctx context.Context, projectName string, oldNetworkName string, newNetworkName string) error

	AddNetworkZone(ctx context.Context, projectName string, networkZoneName string) error
	DeleteNetworkZone(ctx context.Context, projectName string, networkZoneName string) error

	AddNetworkACL(ctx context.Context, projectName string, networkACLName string) error
	DeleteNetworkACL(ctx context.Context, projectName string, networkACLName string) error
	RenameNetworkACL(ctx context.Context, projectName string, oldNetworkACLName string, newNetworkACLName string) error

	AddProfile(ctx context.Context, projectName string, profileName string) error
	DeleteProfile(ctx context.Context, projectName string, profileName string) error
	RenameProfile(ctx context.Context, projectName string, oldProfileName string, newProfileName string) error

	AddStoragePoolVolume(ctx context.Context, projectName string, storagePoolName string, storageVolumeType string, storageVolumeName string, storageVolumeLocation string) error
	DeleteStoragePoolVolume(ctx context.Context, projectName string, storagePoolName string, storageVolumeType string, storageVolumeName string, storageVolumeLocation string) error
	RenameStoragePoolVolume(ctx context.Context, projectName string, storagePoolName string, storageVolumeType string, oldStorageVolumeName string, newStorageVolumeName string, storageVolumeLocation string) error

	AddStorageBucket(ctx context.Context, projectName string, storagePoolName string, storageBucketName string, storageBucketLocation string) error
	DeleteStorageBucket(ctx context.Context, projectName string, storagePoolName string, storageBucketName string, storageBucketLocation string) error
}

// Opts is used as part of the LoadAuthorizer function so that only the relevant configuration fields are passed into a
// particular driver.
type Opts struct {
	config           map[string]any
	projectsGetFunc  func(ctx context.Context) (map[int64]string, error)
	openfgaDatastore storage.OpenFGADatastore
}

// WithConfig can be passed into LoadAuthorizer to pass in driver specific configuration.
func WithConfig(c map[string]any) func(*Opts) {
	return func(o *Opts) {
		o.config = c
	}
}

// WithProjectsGetFunc should be passed into LoadAuthorizer when DriverRBAC is used.
func WithProjectsGetFunc(f func(ctx context.Context) (map[int64]string, error)) func(*Opts) {
	return func(o *Opts) {
		o.projectsGetFunc = f
	}
}

// WithOpenFGADatastore should be passed into LoadAuthorizer when DriverOpenFGAEmbedded is used.
func WithOpenFGADatastore(s storage.OpenFGADatastore) func(opts *Opts) {
	return func(o *Opts) {
		o.openfgaDatastore = s
	}
}

// LoadAuthorizer instantiates, configures, and initialises an Authorizer.
func LoadAuthorizer(ctx context.Context, driver string, logger logger.Logger, certificateCache *certificate.Cache, options ...func(opts *Opts)) (Authorizer, error) {
	opts := &Opts{}
	for _, o := range options {
		o(opts)
	}

	driverFunc, ok := authorizers[driver]
	if !ok {
		return nil, ErrUnknownDriver
	}

	d := driverFunc()
	err := d.init(driver, logger)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize authorizer: %w", err)
	}

	err = d.load(ctx, certificateCache, *opts)
	if err != nil {
		return nil, fmt.Errorf("Failed to load authorizer: %w", err)
	}

	return d, nil
}
