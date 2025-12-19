package auth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
)

const ctxIDMode request.CtxKey = "id_mode"

func IsIDMode(ctx context.Context) bool {
	return ctx.Value(ctxIDMode) != nil
}

func SetIDMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxIDMode, struct{}{})
}

type Entity interface {
	DatabaseID() int64
	URL() *api.URL
	EntityType() entity.Type
	Parent() Entity
}

// PermissionChecker is a type alias for a function that returns whether a user has required permissions on an object.
// It is returned by Authorizer.GetPermissionChecker.
type PermissionChecker func(entityURL *api.URL) bool
type IDPermissionChecker func(id int64) bool

// EntitlementReporter is an interface for adding entitlements to an entity.
type EntitlementReporter interface {
	// ReportEntitlements adds entitlements to the entity.
	// Note: this needs to be a list of string because the implementations of this method will be for the API types.
	ReportEntitlements([]string)
}

// Authorizer is the primary external API for this package.
type Authorizer interface {
	// Driver returns the driver name.
	Driver() string

	// CheckPermission checks if the caller has the given entitlement on the entity found at the given URL.
	//
	// Note: When a project does not have a feature enabled, the given URL should contain the request project, and the
	// effective project for the entity should be set on the request.Info in the given context.
	CheckPermission(ctx context.Context, entityURL *api.URL, entitlement Entitlement) error

	// GetPermissionChecker returns a PermissionChecker for a particular entity.Type.
	//
	// Note: As with CheckPermission, arguments to the returned PermissionChecker should contain the request project for
	// the entity. The effective project for the entity must be set on the request.Info in the given context before
	// calling the PermissionChecker.
	GetPermissionChecker(ctx context.Context, entitlement Entitlement, entityType entity.Type) (PermissionChecker, error)

	// CheckPermissionByID checks if the caller has the given entitlement on the entity with the given type and database ID.
	CheckPermissionByID(ctx context.Context, entityType entity.Type, entityID int64, entitlement Entitlement) error

	// GetIDPermissionChecker returns an IDPermissionChecker for a particular entity.Type.
	GetIDPermissionChecker(ctx context.Context, entityType entity.Type, entitlement Entitlement) (IDPermissionChecker, error)

	// CheckPermissionWithoutEffectiveProject checks a permission, but does not replace the project in the entity URL
	// with the effective project stored in the context.
	//
	// Warn: You almost never need this function. You should use CheckPermission instead.
	CheckPermissionWithoutEffectiveProject(ctx context.Context, entityURL *api.URL, entitlement Entitlement) error

	// GetPermissionCheckerWithoutEffectiveProject returns a PermissionChecker does not replace the project in the entity URL
	// with the effective project stored in the context.
	//
	// Warn: You almost never need this function. You should use GetPermissionChecker instead.
	GetPermissionCheckerWithoutEffectiveProject(ctx context.Context, entitlement Entitlement, entityType entity.Type) (PermissionChecker, error)

	// GetViewableProjects accepts a list of permissions and returns a list of projects that a member of a group with these permissions is able to view.
	GetViewableProjects(ctx context.Context, permissions []api.Permission) ([]string, error)
}

// IsDeniedError returns true if the error is not found or forbidden. This is because the CheckPermission method on
// Authorizer will return a not found error if the requestor does not have access to view the resource. If a requestor
// has view access, but not edit access a forbidden error is returned.
func IsDeniedError(err error) bool {
	return api.StatusErrorCheck(err, http.StatusNotFound, http.StatusForbidden)
}

func OpenFGAObject(p entity.Type, id int64) string {
	return string(p) + ":" + strconv.FormatInt(id, 10)
}

func GetParentEntityOfType(e Entity, t entity.Type) (Entity, error) {
	if e.EntityType() == t {
		return e, nil
	}

	parent := e.Parent()
	if parent == nil {
		return nil, fmt.Errorf("Entity %q does not have a parent of type %q", e.URL().String(), t)
	}

	return GetParentEntityOfType(parent, t)
}

func ParseOpenFGAObject(obj string) (entity.Type, int64, error) {
	e, i, ok := strings.Cut(obj, ":")
	if !ok {
		return "", -1, fmt.Errorf("Failed to split object %q on colon separator", obj)
	}

	eType := entity.Type(e)
	err := eType.Validate()
	if err != nil {
		return "", -1, fmt.Errorf("Failed to get entity type of object %q: %w", obj, err)
	}

	id, err := strconv.ParseInt(i, 10, 64)
	if err != nil {
		return "", -1, fmt.Errorf("Failed to get entity ID of object %q: %w", obj, err)
	}

	return eType, id, nil
}
