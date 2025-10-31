//go:build linux && cgo && !agent

package drivers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db/broker"
	"github.com/canonical/lxd/lxd/identity"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/logger"
)

const (
	// DriverTLS is used at start up to allow communication between cluster members and initialise the cluster database.
	DriverTLS string = "tls"
)

func init() {
	authorizers[DriverTLS] = func() authorizer { return &tls{} }
}

type tls struct {
	commonAuthorizer
}

func (t *tls) load(ctx context.Context, identityCache *identity.Cache, opts Opts) error {
	return nil
}

// GetViewableProjects is not implemented for the TLS authorizer.
func (t *tls) GetViewableProjects(ctx context.Context, permissions []api.Permission) ([]string, error) {
	return nil, api.NewGenericStatusError(http.StatusNotImplemented)
}

// CheckPermission returns an error if the user does not have the given Entitlement on the given Object.
func (t *tls) CheckPermission(ctx context.Context, entitlement auth.Entitlement, entityType entity.Type, entityID int) error {
	requestor, err := request.GetRequestor(ctx)
	if err != nil {
		return err
	}

	// Untrusted requests are denied.
	if !requestor.IsTrusted() {
		return api.NewGenericStatusError(http.StatusForbidden)
	}

	// Cluster or unix socket requests have admin permission.
	if requestor.IsAdmin() {
		return nil
	}

	id := requestor.CallerIdentity()
	if id == nil {
		return errors.New("No identity is set in the request details")
	}

	if id.IdentityType == api.IdentityTypeCertificateMetricsUnrestricted && entitlement == auth.EntitlementCanViewMetrics {
		return nil
	}

	projectSpecific, err := entityType.RequiresProject()
	if err != nil {
		return fmt.Errorf("Failed to check project specificity of entity type %q: %w", entityType, err)
	}

	model, err := broker.GetModelFromContext(ctx)
	if err != nil {
		return err
	}

	identity, err := model.GetIdentityFullByAuthenticationMethodAndIdentifier(ctx, requestor.CallerProtocol(), requestor.CallerUsername())
	if err != nil {
		return err
	}

	// Check non- project-specific entity types.
	if !projectSpecific {
		if t.allowProjectUnspecificEntityType(entityType, entityID, entitlement, *identity) {
			return nil
		}

		return api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	}

	e, err := model.GetEntityByID(entityType, entityID)
	if err != nil {
		return err
	}

	projectEntity := auth.GetParentEntityOfType(e, entity.TypeProject)
	if projectEntity == nil {
		return fmt.Errorf("Entity of type %q and ID %d has no parent project entity", entityType, entityID)
	}

	// Check project level permissions against the certificates project list.
	if !slices.Contains(identity.Projects, projectEntity.DatabaseID()) {
		return api.NewStatusError(http.StatusForbidden, "User does not have permission for this project")
	}

	return nil
}

// GetPermissionChecker returns a function that can be used to check whether a user has the required entitlement on an authorization object.
func (t *tls) GetPermissionChecker(ctx context.Context, entitlement auth.Entitlement, entityType entity.Type) (auth.PermissionChecker, error) {
	allowFunc := func(b bool) func(int) bool {
		return func(_ int) bool {
			return b
		}
	}

	requestor, err := request.GetRequestor(ctx)
	if err != nil {
		return nil, err
	}

	// Untrusted requests are denied.
	if !requestor.IsTrusted() {
		return allowFunc(false), nil
	}

	// Cluster or unix socket requests have admin permission.
	if requestor.IsAdmin() {
		return allowFunc(true), nil
	}

	id := requestor.CallerIdentity()
	if id == nil {
		return nil, errors.New("No identity is set in the request details")
	}

	if id.IdentityType == api.IdentityTypeCertificateMetricsUnrestricted && entitlement == auth.EntitlementCanViewMetrics {
		return allowFunc(true), nil
	}

	projectSpecific, err := entityType.RequiresProject()
	if err != nil {
		return nil, fmt.Errorf("Failed to check project specificity of entity type %q: %w", entityType, err)
	}

	model, err := broker.GetModelFromContext(ctx)
	if err != nil {
		return nil, err
	}

	identity, err := model.GetIdentityFullByAuthenticationMethodAndIdentifier(ctx, requestor.CallerProtocol(), requestor.CallerUsername())
	if err != nil {
		return nil, err
	}

	// Filter objects by project.
	return func(entityID int) bool {
		// Check non- project-specific entity types.
		if !projectSpecific {
			return t.allowProjectUnspecificEntityType(entityType, entityID, entitlement, *identity)
		}

		e, err := model.GetEntityByID(entityType, entityID)
		if err != nil {
			logger.Error("Couldn't get entity", logger.Ctx{"entity_type": entityType, "entity_id": entityID, "err": err})
			return false
		}

		projectEntity := auth.GetParentEntityOfType(e, entity.TypeProject)
		if projectEntity == nil {
			logger.Error("Encountered entity with no parent project entity", logger.Ctx{"entity_type": entityType, "entity_id": entityID})
		}

		// Otherwise, check if the project is in the list of allowed projects for the entity.
		return slices.Contains(identity.Projects, projectEntity.DatabaseID())
	}, nil
}

func (t *tls) allowProjectUnspecificEntityType(entityType entity.Type, entityID int, entitlement auth.Entitlement, identity broker.IdentityFull) bool {
	switch entityType {
	case entity.TypeServer:
		// Restricted TLS certificates have the following entitlements on server.
		//
		// Note: We have to keep EntitlementCanViewMetrics here for backwards compatibility with older versions of LXD.
		// Historically when viewing the metrics endpoint for a specific project with a restricted certificate also the
		// internal server metrics get returned.
		return slices.Contains([]auth.Entitlement{auth.EntitlementCanViewResources, auth.EntitlementCanViewMetrics, auth.EntitlementCanViewUnmanagedNetworks}, entitlement)
	case entity.TypeIdentity, entity.TypeCertificate:

		// If the entity URL refers to the identity that made the request, then the second path argument of the URL is
		// the identifier of the identity. This line allows the caller to view their own identity and no one else's.
		return entitlement == auth.EntitlementCanView && entityID == identity.ID
	case entity.TypeProject:
		// If the project is in the list of projects that the identity is restricted to, then they have the following
		// entitlements.
		return slices.Contains(identity.Projects, entityID) && slices.Contains([]auth.Entitlement{auth.EntitlementCanView, auth.EntitlementCanCreateImages, auth.EntitlementCanCreateImageAliases, auth.EntitlementCanCreateInstances, auth.EntitlementCanCreateNetworks, auth.EntitlementCanCreateNetworkACLs, auth.EntitlementCanCreateNetworkZones, auth.EntitlementCanCreateProfiles, auth.EntitlementCanCreateStorageVolumes, auth.EntitlementCanCreateStorageBuckets, auth.EntitlementCanViewEvents, auth.EntitlementCanViewOperations, auth.EntitlementCanViewMetrics}, entitlement)

	default:
		return false
	}
}
