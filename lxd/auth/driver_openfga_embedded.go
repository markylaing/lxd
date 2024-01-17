package auth

import (
	"bytes"
	"context"
	"fmt"
	"github.com/canonical/lxd/lxd/entity"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/oklog/ulid/v2"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/server"
	"go.uber.org/zap"
	"net/http"

	"github.com/canonical/lxd/lxd/certificate"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
)

// embeddedOpenFGA implements Authorizer using an embedded OpenFGA server.
type embeddedOpenFGA struct {
	commonAuthorizer
	server           openfgav1.OpenFGAServiceServer
	certificateCache *certificate.Cache
}

// load sets up the authorizer.
func (e *embeddedOpenFGA) load(ctx context.Context, certificateCache *certificate.Cache, opts Opts) error {
	if certificateCache == nil {
		return fmt.Errorf("Must provide certificate cache")
	}

	e.certificateCache = certificateCache

	if opts.openfgaDatastore == nil {
		return fmt.Errorf("The OpenFGA datastore option must be set")
	}

	var err error
	e.server, err = server.NewServerWithOpts(server.WithDatastore(opts.openfgaDatastore), server.WithLogger(openfgaLogger{l: e.logger}))
	if err != nil {
		return err
	}

	var builtinAuthorizationModel openfgav1.WriteAuthorizationModelRequest
	pb := runtime.JSONPb{}
	err = pb.NewDecoder(bytes.NewReader([]byte(authModel))).Decode(&builtinAuthorizationModel)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal built in authorization model: %w", err)
	}

	builtinAuthorizationModel.StoreId = ulid.Make().String()
	_, err = e.server.WriteAuthorizationModel(ctx, &builtinAuthorizationModel)
	if err != nil {
		return err
	}

	return nil
}

// CheckPermission checks whether the user who sent the request has the given entitlement on the given object using the
// embedded OpenFGA server.
func (e *embeddedOpenFGA) CheckPermission(ctx context.Context, r *http.Request, relation entity.Entitlement, entityType entity.Type, projectName string, location string, pathArgs ...string) error {
	object := entityType.AuthObject(projectName, location, pathArgs...)
	logCtx := logger.Ctx{"object": object, "relation": relation, "url": r.URL.String(), "method": r.Method}
	//ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	//defer cancel()

	details, err := e.requestDetails(r)
	if err != nil {
		return api.StatusErrorf(http.StatusForbidden, "Failed to extract request details: %v", err)
	}

	if details.isInternalOrUnix() {
		return nil
	}

	username := details.username()
	protocol := details.authenticationProtocol()
	logCtx["username"] = username
	logCtx["protocol"] = protocol

	if protocol != api.AuthenticationMethodTLS {
		return api.StatusErrorf(http.StatusForbidden, "Only TLS supported")
	}

	certType, isPrivileged, projects, groups, err := certificateDetails(e.certificateCache, username)
	if err != nil {
		return err
	}

	if isPrivileged || (certType == certificate.TypeMetrics && relation == entity.EntitlementCanViewMetrics) {
		return nil
	}

	userURL, err := entity.TypeAuthUser.URL("", "", username)
	if err != nil {
		return err
	}

	objectUser := fmt.Sprintf("%s:%s", entity.Names[entity.TypeAuthUser], userURL)
	req := &openfgav1.CheckRequest{
		StoreId: ulid.Make().String(),
		TupleKey: &openfgav1.CheckRequestTupleKey{
			User:     objectUser,
			Relation: string(relation),
			Object:   object,
		},
		ContextualTuples: &openfgav1.ContextualTupleKeys{
			TupleKeys: []*openfgav1.TupleKey{},
		},
	}

	if isPrivileged {
		req.ContextualTuples.TupleKeys = []*openfgav1.TupleKey{
			{
				User:     objectUser,
				Relation: string(entity.EntitlementAdmin),
				Object:   entity.TypeServer.AuthObject("", ""),
			},
		}
	} else {
		for _, groupName := range groups {
			req.ContextualTuples.TupleKeys = append(req.ContextualTuples.TupleKeys, &openfgav1.TupleKey{
				User:     objectUser,
				Relation: string(entity.EntitlementMember),
				Object:   entity.TypeAuthGroup.AuthObject("", "", groupName),
			})
		}

		for _, projectName := range projects {
			req.ContextualTuples.TupleKeys = append(req.ContextualTuples.TupleKeys, &openfgav1.TupleKey{
				User:      objectUser,
				Relation:  string(entity.EntitlementOperator),
				Object:    entity.TypeProject.AuthObject("", "", projectName),
				Condition: nil,
			})
		}
	}

	e.logger.Debug("Checking OpenFGA relation", logCtx)
	resp, err := e.server.Check(ctx, req)
	if err != nil {
		return fmt.Errorf("Failed to check OpenFGA relation: %w", err)
	}

	if !resp.GetAllowed() {
		return api.StatusErrorf(http.StatusForbidden, "User does not have entitlement %q on object %q", relation, object)
	}

	return nil
}

// GetPermissionChecker returns a PermissionChecker using the embedded openfga server.
func (e *embeddedOpenFGA) GetPermissionChecker(ctx context.Context, r *http.Request, relation entity.Entitlement, entityType entity.Type) (PermissionChecker, error) {
	//ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	//defer cancel()

	allowFunc := func(b bool) func(string) bool {
		return func(string) bool {
			return b
		}
	}

	logCtx := logger.Ctx{"entity_type": entityType, "relation": relation, "url": r.URL.String(), "method": r.Method}
	details, err := e.requestDetails(r)
	if err != nil {
		return nil, api.StatusErrorf(http.StatusForbidden, "Failed to extract request details: %v", err)
	}

	if details.isInternalOrUnix() {
		return allowFunc(true), nil
	}

	username := details.username()
	protocol := details.authenticationProtocol()
	logCtx["username"] = username
	logCtx["protocol"] = protocol

	if protocol != api.AuthenticationMethodTLS {
		return nil, api.StatusErrorf(http.StatusForbidden, "Only TLS supported")
	}

	certType, isPrivileged, projects, groups, err := certificateDetails(e.certificateCache, username)
	if err != nil {
		return nil, err
	}

	if isPrivileged || (certType == certificate.TypeMetrics && relation == entity.EntitlementCanViewMetrics) {
		return allowFunc(true), nil
	}

	objectUser := entity.TypeAuthUser.AuthObject("", "", username)
	req := &openfgav1.ListObjectsRequest{
		StoreId:  ulid.Make().String(),
		Type:     entityType.String(),
		Relation: string(relation),
		User:     objectUser,
		ContextualTuples: &openfgav1.ContextualTupleKeys{
			TupleKeys: []*openfgav1.TupleKey{},
		},
	}

	if isPrivileged {
		req.ContextualTuples.TupleKeys = []*openfgav1.TupleKey{
			{
				User:     objectUser,
				Relation: "admin",
				Object:   fmt.Sprintf("%s:%s", entity.Names[entity.TypeServer], entity.TypeServer.AuthObject("", "")),
			},
		}
	} else {
		for _, groupName := range groups {
			req.ContextualTuples.TupleKeys = append(req.ContextualTuples.TupleKeys, &openfgav1.TupleKey{
				User:     objectUser,
				Relation: string(entity.EntitlementMember),
				Object:   entity.TypeAuthGroup.AuthObject("", "", groupName),
			})
		}

		for _, projectName := range projects {
			req.ContextualTuples.TupleKeys = append(req.ContextualTuples.TupleKeys, &openfgav1.TupleKey{
				User:      objectUser,
				Relation:  string(entity.EntitlementOperator),
				Object:    entity.TypeProject.AuthObject("", "", projectName),
				Condition: nil,
			})
		}
	}

	e.logger.Debug("Listing related objects for user", logCtx)
	resp, err := e.server.ListObjects(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Failed to OpenFGA objects of type %q with relation %q for user %q: %w", entityType.String(), relation, username, err)
	}

	objects := resp.GetObjects()

	return func(object string) bool {
		return shared.ValueInSlice(object, objects)
	}, nil
}

// openfgaLogger implements OpenFGA's logger.Logger interface but delegates to our logger.
type openfgaLogger struct {
	l logger.Logger
}

// Debug delegates to the authorizers logger.
func (o openfgaLogger) Debug(s string, field ...zap.Field) {
	o.l.Debug(s)
}

// Info delegates to the authorizers logger.
func (o openfgaLogger) Info(s string, field ...zap.Field) {
	o.l.Info(s)
}

// Warn delegates to the authorizers logger.
func (o openfgaLogger) Warn(s string, field ...zap.Field) {
	o.l.Warn(s)
}

// Error delegates to the authorizers logger.
func (o openfgaLogger) Error(s string, field ...zap.Field) {
	o.l.Error(s)
}

// Panic delegates to the authorizers logger.
func (o openfgaLogger) Panic(s string, field ...zap.Field) {
	o.l.Panic(s)
}

// Fatal delegates to the authorizers logger.
func (o openfgaLogger) Fatal(s string, field ...zap.Field) {
	o.l.Fatal(s)
}

// DebugWithContext delegates to the authorizers logger.
func (o openfgaLogger) DebugWithContext(ctx context.Context, s string, field ...zap.Field) {
	o.l.Debug(s)
}

// InfoWithContext delegates to the authorizers logger.
func (o openfgaLogger) InfoWithContext(ctx context.Context, s string, field ...zap.Field) {
	o.l.Info(s)
}

// WarnWithContext delegates to the authorizers logger.
func (o openfgaLogger) WarnWithContext(ctx context.Context, s string, field ...zap.Field) {
	o.l.Warn(s)
}

// ErrorWithContext delegates to the authorizers logger.
func (o openfgaLogger) ErrorWithContext(ctx context.Context, s string, field ...zap.Field) {
	o.l.Error(s)
}

// PanicWithContext delegates to the authorizers logger.
func (o openfgaLogger) PanicWithContext(ctx context.Context, s string, field ...zap.Field) {
	o.l.Panic(s)
}

// FatalWithContext delegates to the authorizers logger.
func (o openfgaLogger) FatalWithContext(ctx context.Context, s string, field ...zap.Field) {
	o.l.Fatal(s)
}
