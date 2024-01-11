package auth

import (
	"context"
	"errors"
	"github.com/canonical/lxd/shared/entitlement"
	"net/http"

	"github.com/canonical/lxd/lxd/certificate"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
)

type tls struct {
	commonAuthorizer
	certificates *certificate.Cache
}

func (t *tls) load(ctx context.Context, certificateCache *certificate.Cache, opts Opts) error {
	if certificateCache == nil {
		return errors.New("TLS authorization driver requires a certificate cache")
	}

	t.certificates = certificateCache
	return nil
}

// CheckPermission returns an error if the user does not have the given Relation on the given Object.
func (t *tls) CheckPermission(ctx context.Context, r *http.Request, object entitlement.Object, relation entitlement.Relation) error {
	details, err := t.requestDetails(r)
	if err != nil {
		return api.StatusErrorf(http.StatusForbidden, "Failed to extract request details: %v", err)
	}

	if details.isInternalOrUnix() {
		return nil
	}

	authenticationProtocol := details.authenticationProtocol()
	if authenticationProtocol != api.AuthenticationMethodTLS {
		t.logger.Warn("Authentication protocol is not compatible with authorization driver", logger.Ctx{"protocol": authenticationProtocol})
		// Return nil. If the server has been configured with an authentication method but no associated authorization driver,
		// the default is to give these authenticated users admin privileges.
		return nil
	}

	certType, isNotRestricted, projectNames, _, err := t.certificateDetails(details.username())
	if err != nil {
		return err
	}

	if isNotRestricted || (certType == certificate.TypeMetrics && relation == entitlement.RelationCanViewMetrics) {
		return nil
	}

	if details.isAllProjectsRequest {
		// Only admins (users with non-restricted certs) can use the all-projects parameter.
		return api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	}

	// Check server level object types
	switch object.Type() {
	case entitlement.ObjectTypeServer:
		if relation == entitlement.RelationCanView || relation == entitlement.RelationCanViewResources || relation == entitlement.RelationCanViewMetrics {
			return nil
		}

		return api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	case entitlement.ObjectTypeStoragePool, entitlement.ObjectTypeCertificate:
		if relation == entitlement.RelationCanView {
			return nil
		}

		return api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	}

	// Check project level permissions against the certificates project list.
	projectName := object.Project()
	if !shared.ValueInSlice(projectName, projectNames) {
		return api.StatusErrorf(http.StatusForbidden, "User does not have permission for project %q", projectName)
	}

	return nil
}

// GetPermissionChecker returns a function that can be used to check whether a user has the required entitlement on an authorization object.
func (t *tls) GetPermissionChecker(ctx context.Context, r *http.Request, relation entitlement.Relation, objectType entitlement.ObjectType) (PermissionChecker, error) {
	allowFunc := func(b bool) func(entitlement.Object) bool {
		return func(entitlement.Object) bool {
			return b
		}
	}

	details, err := t.requestDetails(r)
	if err != nil {
		return nil, api.StatusErrorf(http.StatusForbidden, "Failed to extract request details: %v", err)
	}

	if details.isInternalOrUnix() {
		return allowFunc(true), nil
	}

	authenticationProtocol := details.authenticationProtocol()
	if authenticationProtocol != api.AuthenticationMethodTLS {
		t.logger.Warn("Authentication protocol is not compatible with authorization driver", logger.Ctx{"protocol": authenticationProtocol})
		// Allow all. If the server has been configured with an authentication method but no associated authorization driver,
		// the default is to give these authenticated users admin privileges.
		return allowFunc(true), nil
	}

	certType, isNotRestricted, projectNames, _, err := t.certificateDetails(details.username())
	if err != nil {
		return nil, err
	}

	if isNotRestricted || (certType == certificate.TypeMetrics && relation == entitlement.RelationCanViewMetrics) {
		return allowFunc(true), nil
	}

	if details.isAllProjectsRequest {
		// Only admins (users with non-restricted certs) can use the all-projects parameter.
		return nil, api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	}

	// Check server level object types
	switch objectType {
	case entitlement.ObjectTypeServer:
		if relation == entitlement.RelationCanView || relation == entitlement.RelationCanViewResources || relation == entitlement.RelationCanViewMetrics {
			return allowFunc(true), nil
		}

		return nil, api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	case entitlement.ObjectTypeStoragePool, entitlement.ObjectTypeCertificate:
		if relation == entitlement.RelationCanView {
			return allowFunc(true), nil
		}

		return nil, api.StatusErrorf(http.StatusForbidden, "Certificate is restricted")
	}

	// Error if user does not have access to the project (unless we're getting projects, where we want to filter the results).
	if !shared.ValueInSlice(details.projectName, projectNames) && objectType != entitlement.ObjectTypeProject {
		return nil, api.StatusErrorf(http.StatusForbidden, "User does not have permissions for project %q", details.projectName)
	}

	// Filter objects by project.
	return func(object entitlement.Object) bool {
		return shared.ValueInSlice(object.Project(), projectNames)
	}, nil
}

// certificateDetails returns the certificate type, a boolean indicating if the certificate is *not* restricted, a slice of
// project names for this certificate, or an error if the certificate could not be found.
func (t *tls) certificateDetails(fingerprint string) (certificate.Type, bool, []string, []string, error) {
	return certificateDetails(t.certificates, fingerprint)
}

func certificateDetails(cache *certificate.Cache, fingerprint string) (certificate.Type, bool, []string, []string, error) {
	certs, projects, groups := cache.GetCertificatesProjectsAndGroups()
	clientCerts := certs[certificate.TypeClient]
	_, ok := clientCerts[fingerprint]
	if ok {
		projectNames, hasProjects := projects[fingerprint]
		groupNames, hasGroups := groups[fingerprint]
		if !hasProjects && !hasGroups {
			// Certificate is not restricted.
			return certificate.TypeClient, true, nil, nil, nil
		}

		return certificate.TypeClient, false, projectNames, groupNames, nil
	}

	// If not a client cert, could be a metrics cert. Only need to check one entitlement.
	metricCerts := certs[certificate.TypeMetrics]
	_, ok = metricCerts[fingerprint]
	if ok {
		return certificate.TypeMetrics, false, nil, nil, nil
	}

	// If we're in a CA environment, it's possible for a certificate to be trusted despite not being present in the trust store.
	// We rely on the validation of the certificate (and its potential revocation) having been done in CheckTrustState.
	if shared.PathExists(shared.VarPath("server.ca")) {
		return certificate.TypeClient, true, nil, nil, nil
	}

	return -1, false, nil, nil, api.StatusErrorf(http.StatusForbidden, "Client certificate not found")
}
