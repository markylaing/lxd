package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	dqliteDriver "github.com/canonical/go-dqlite/driver"
	"github.com/canonical/lxd/lxd/certificate"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entitlement"
	"github.com/canonical/lxd/shared/version"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var groupsCmd = APIEndpoint{
	Name: "groups",
	Path: "groups",
	Get: APIEndpointAction{
		Handler:        getGroups,
		AllowUntrusted: true,
	},
	Post: APIEndpointAction{
		Handler:        createGroup,
		AllowUntrusted: true,
	},
}

var groupCmd = APIEndpoint{
	Name: "group",
	Path: "groups/{groupName}",
	Get: APIEndpointAction{
		Handler:        getGroup,
		AllowUntrusted: true,
	},
	Put: APIEndpointAction{
		Handler:        updateGroup,
		AllowUntrusted: true,
	},
	Post: APIEndpointAction{
		Handler:        renameGroup,
		AllowUntrusted: true,
	},
	Delete: APIEndpointAction{
		Handler:        deleteGroup,
		AllowUntrusted: true,
	},
	Patch: APIEndpointAction{
		Handler:        patchGroup,
		AllowUntrusted: true,
	},
}

var groupUserCmd = APIEndpoint{
	Name: "group_user",
	Path: "groups/{groupName}/users/{authMethod}/{userNameOrID}",
	Post: APIEndpointAction{
		Handler:        groupAddUser,
		AllowUntrusted: true,
	},
	Delete: APIEndpointAction{
		Handler:        groupRemoveUser,
		AllowUntrusted: true,
	},
}

var groupEntitlementCmd = APIEndpoint{
	Name: "group_entitlement",
	Path: "groups/{groupName}/entitlements/{object}/{relation}",
	Post: APIEndpointAction{
		Handler:        groupGrantEntitlement,
		AllowUntrusted: true,
	},
	Delete: APIEndpointAction{
		Handler:        groupRevokeEntitlement,
		AllowUntrusted: true,
	},
}

func validateGroupName(name string) error {
	if name == "" {
		return api.StatusErrorf(http.StatusBadRequest, "Group name cannot be empty")
	}

	if strings.Contains(name, "/") {
		return api.StatusErrorf(http.StatusBadRequest, "Group name cannot contain a forward slash")
	}

	if strings.Contains(name, ":") {
		return api.StatusErrorf(http.StatusBadRequest, "Group name cannot contain a colon")
	}

	return nil
}

func getGroups(d *Daemon, r *http.Request) response.Response {
	recursion := request.QueryParam(r, "recursion")
	var apiGroups []api.Group
	var groupURLs []string
	err := d.db.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		groups, err := cluster.GetGroups(ctx, tx.Tx())
		if err != nil {
			return err
		}

		if recursion == "1" {
			apiGroups = make([]api.Group, 0, len(groups))
			for _, group := range groups {
				apiGroup, err := group.ToAPI(ctx, tx.Tx())
				if err != nil {
					return err
				}

				apiGroups = append(apiGroups, *apiGroup)
			}
		} else {
			groupURLs = make([]string, 0, len(groups))
			for _, group := range groups {
				groupURLs = append(groupURLs, api.NewURL().Path(version.APIVersion, "groups", group.Name).String())
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	if recursion == "1" {
		return response.SyncResponse(true, apiGroups)
	}

	return response.SyncResponse(true, groupURLs)
}

func createGroup(d *Daemon, r *http.Request) response.Response {
	var group api.Group
	err := json.NewDecoder(r.Body).Decode(&group)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Invalid request body: %w", err))
	}

	err = validateGroupName(group.Name)
	if err != nil {
		return response.SmartError(err)
	}

	// Validate entitlements against model.
	for object, relations := range group.Entitlements {
		authObject, err := entitlement.ObjectFromString(object)
		if err != nil {
			return response.BadRequest(err)
		}

		objectType := authObject.Type()
		for _, relation := range relations {
			err = objectType.ValidateRelation(entitlement.Relation(relation))
			if err != nil {
				return response.BadRequest(err)
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	s := d.State()
	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		groupID, err := cluster.CreateGroup(ctx, tx.Tx(), cluster.Group{
			Name:        group.Name,
			Description: group.Description,
		})
		if err != nil {
			return err
		}

		entitlementIDs, err := upsertGroupEntitlements(ctx, tx, group.Entitlements)
		if err != nil {
			return err
		}

		groupEntitlements := make([]cluster.GroupEntitlement, 0, len(entitlementIDs))
		for _, entitlementID := range entitlementIDs {
			groupEntitlements = append(groupEntitlements, cluster.GroupEntitlement{
				GroupID:       int(groupID),
				EntitlementID: entitlementID,
			})
		}

		err = cluster.CreateGroupEntitlements(ctx, tx.Tx(), groupEntitlements)
		if err != nil {
			return err
		}

		var groupsCertificates []cluster.GroupCertificate
		for _, fingerprint := range group.TLSUsers {
			cert, err := cluster.GetCertificateByFingerprintPrefix(ctx, tx.Tx(), fingerprint)
			if err != nil {
				return err
			}

			groupsCertificates = append(groupsCertificates, cluster.GroupCertificate{
				CertificateID: cert.ID,
				GroupID:       int(groupID),
			})
		}

		err = cluster.CreateGroupCertificates(ctx, tx.Tx(), groupsCertificates)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.SyncResponseLocation(true, nil, api.NewURL().Path(version.APIVersion, "groups", group.Name).String())
}

func getGroup(d *Daemon, r *http.Request) response.Response {
	groupName, err := url.PathUnescape(mux.Vars(r)["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var apiGroup *api.Group
	s := d.State()
	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		group, err := cluster.GetGroup(ctx, tx.Tx(), groupName)
		if err != nil {
			return err
		}

		apiGroup, err = group.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponseETag(true, *apiGroup, *apiGroup)
}

func updateGroup(d *Daemon, r *http.Request) response.Response {
	groupName, err := url.PathUnescape(mux.Vars(r)["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	var groupPut api.GroupPut
	err = json.NewDecoder(r.Body).Decode(&groupPut)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Invalid request body: %w", err))
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	s := d.State()
	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		group, err := cluster.GetGroup(ctx, tx.Tx(), groupName)
		if err != nil {
			return err
		}

		apiGroup, err := group.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		err = util.EtagCheck(r, *apiGroup)
		if err != nil {
			return err
		}

		err = cluster.UpdateGroup(ctx, tx.Tx(), groupName, cluster.Group{
			Name:        groupName,
			Description: groupPut.Description,
		})
		if err != nil {
			return err
		}

		err = cluster.DeleteGroupEntitlements(ctx, tx.Tx(), group.ID)
		if err != nil {
			return err
		}

		newEntitlementIDs, err := upsertGroupEntitlements(ctx, tx, groupPut.Entitlements)
		if err != nil {
			return err
		}

		groupEntitlements := make([]cluster.GroupEntitlement, 0, len(newEntitlementIDs))
		for _, entitlementID := range newEntitlementIDs {
			groupEntitlements = append(groupEntitlements, cluster.GroupEntitlement{
				GroupID:       group.ID,
				EntitlementID: entitlementID,
			})
		}

		err = cluster.CreateGroupEntitlements(ctx, tx.Tx(), groupEntitlements)
		if err != nil {
			return err
		}

		err = cluster.DeleteGroupCertificates(ctx, tx.Tx(), group.ID)
		if err != nil {
			return err
		}

		var groupsCertificates []cluster.GroupCertificate
		for _, certNameOrFingerprintPrefix := range groupPut.TLSUsers {
			cert, err := getUniqueCertByNameOrFingerprintPrefix(ctx, tx.Tx(), certNameOrFingerprintPrefix)
			if err != nil {
				return err
			}

			groupsCertificates = append(groupsCertificates, cluster.GroupCertificate{
				CertificateID: cert.ID,
				GroupID:       group.ID,
			})
		}

		err = cluster.CreateGroupCertificates(ctx, tx.Tx(), groupsCertificates)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.EmptySyncResponse
}

func patchGroup(d *Daemon, r *http.Request) response.Response {
	groupName, err := url.PathUnescape(mux.Vars(r)["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	var groupPut api.GroupPut
	err = json.NewDecoder(r.Body).Decode(&groupPut)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Invalid request body: %w", err))
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	s := d.State()
	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		group, err := cluster.GetGroup(ctx, tx.Tx(), groupName)
		if err != nil {
			return err
		}

		apiGroup, err := group.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		err = util.EtagCheck(r, *apiGroup)
		if err != nil {
			return err
		}

		if groupPut.Description != "" {
			err = cluster.UpdateGroup(ctx, tx.Tx(), groupName, cluster.Group{
				Name:        groupName,
				Description: groupPut.Description,
			})
			if err != nil {
				return err
			}
		}

		newEntitlementIDs, err := upsertGroupEntitlements(ctx, tx, groupPut.Entitlements)
		if err != nil {
			return err
		}

		groupEntitlements := make([]cluster.GroupEntitlement, 0, len(newEntitlementIDs))
		for _, entitlementID := range newEntitlementIDs {
			groupEntitlements = append(groupEntitlements, cluster.GroupEntitlement{
				GroupID:       group.ID,
				EntitlementID: entitlementID,
			})
		}

		err = cluster.CreateGroupEntitlements(ctx, tx.Tx(), groupEntitlements)
		if err != nil {
			return err
		}

		var groupsCertificates []cluster.GroupCertificate
		for _, certNameOrFingerprintPrefix := range groupPut.TLSUsers {
			cert, err := getUniqueCertByNameOrFingerprintPrefix(ctx, tx.Tx(), certNameOrFingerprintPrefix)
			if err != nil {
				return err
			}

			groupsCertificates = append(groupsCertificates, cluster.GroupCertificate{
				CertificateID: cert.ID,
				GroupID:       group.ID,
			})
		}

		err = cluster.CreateGroupCertificates(ctx, tx.Tx(), groupsCertificates)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.EmptySyncResponse
}

func renameGroup(d *Daemon, r *http.Request) response.Response {
	groupName, err := url.PathUnescape(mux.Vars(r)["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	var groupPost api.GroupPost
	err = json.NewDecoder(r.Body).Decode(&groupPost)
	if err != nil {
		return response.BadRequest(fmt.Errorf("Invalid request body: %w", err))
	}

	err = validateGroupName(groupPost.Name)
	if err != nil {
		return response.SmartError(err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	s := d.State()
	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		err = cluster.UpdateGroup(ctx, tx.Tx(), groupName, cluster.Group{
			Name: groupPost.Name,
		})
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.SyncResponseLocation(true, nil, api.NewURL().Path(version.APIVersion, "groups", groupPost.Name).String())
}

func deleteGroup(d *Daemon, r *http.Request) response.Response {
	groupName, err := url.PathUnescape(mux.Vars(r)["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	s := d.State()
	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return cluster.DeleteGroup(ctx, tx.Tx(), groupName)
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.EmptySyncResponse
}

func upsertGroupEntitlements(ctx context.Context, tx *db.ClusterTx, entitlements map[string][]string) ([]int, error) {
	var entitlementIDs []int
	for object, relations := range entitlements {
		authObject, _ := entitlement.ObjectFromString(object)
		objectType := authObject.Type()
		objectRef := authObject.Ref()
		var entityID *int

		if objectType != entitlement.ObjectTypeServer {
			entityIDInt, err := tx.GetAuthObjectEntityID(ctx, authObject)
			if err != nil {
				return nil, err
			}

			entityID = &entityIDInt
		}

		for _, relation := range relations {
			existingEntitlement, err := cluster.GetEntitlement(ctx, tx.Tx(), relation, string(objectType), objectRef, entityID)
			if err == nil {
				entitlementIDs = append(entitlementIDs, existingEntitlement.ID)
				continue
			} else if !api.StatusErrorCheck(err, http.StatusNotFound) {
				return nil, err
			}

			entitlement := cluster.Entitlement{
				Relation:   relation,
				ObjectType: string(objectType),
				ObjectRef:  objectRef,
				EntityID:   entityID,
			}

			entitlementID, err := cluster.CreateEntitlement(ctx, tx.Tx(), entitlement)
			if err != nil {
				return nil, err
			}

			entitlementIDs = append(entitlementIDs, entitlementID)
		}
	}

	return entitlementIDs, nil
}

func groupAddUser(d *Daemon, r *http.Request) response.Response {
	muxVars := mux.Vars(r)
	groupName, err := url.PathUnescape(muxVars["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	userNameOrID, err := url.PathUnescape(muxVars["userNameOrID"])
	if err != nil {
		return response.SmartError(err)
	}

	authMethod, err := url.PathUnescape(muxVars["authMethod"])
	if err != nil {
		return response.SmartError(err)
	}

	s := d.State()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return manageGroupMembership(ctx, tx, groupName, authMethod, userNameOrID, false)
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.EmptySyncResponse
}

func groupRemoveUser(d *Daemon, r *http.Request) response.Response {
	muxVars := mux.Vars(r)
	groupName, err := url.PathUnescape(muxVars["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	userNameOrID, err := url.PathUnescape(muxVars["userNameOrID"])
	if err != nil {
		return response.SmartError(err)
	}

	authMethod, err := url.PathUnescape(muxVars["authMethod"])
	if err != nil {
		return response.SmartError(err)
	}

	s := d.State()

	//ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	//defer cancel()

	err = s.DB.Cluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
		return manageGroupMembership(ctx, tx, groupName, authMethod, userNameOrID, true)
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.UpdateCertificateCache()

	return response.EmptySyncResponse
}

func manageGroupMembership(ctx context.Context, tx *db.ClusterTx, groupName string, authMethod string, userNameOrID string, remove bool) error {
	switch authMethod {
	case api.AuthenticationMethodCandid, api.AuthenticationMethodOIDC:
		return api.StatusErrorf(http.StatusNotImplemented, "Authentication method %q not supported", authMethod)
	case api.AuthenticationMethodTLS:
	default:
		return api.StatusErrorf(http.StatusBadRequest, "Invalid authentication method %q", authMethod)
	}

	if authMethod != api.AuthenticationMethodTLS {
		return api.StatusErrorf(http.StatusNotImplemented, "Authentication method %q not supported", authMethod)
	}

	cert, err := getUniqueCertByNameOrFingerprintPrefix(ctx, tx.Tx(), userNameOrID)

	group, err := cluster.GetGroup(ctx, tx.Tx(), groupName)
	if err != nil {
		return err
	}

	groupCertificate := cluster.GroupCertificate{
		GroupID:       group.ID,
		CertificateID: cert.ID,
	}

	if remove {
		res, err := tx.Tx().ExecContext(ctx, `DELETE FROM certificates_groups WHERE certificate_id = ? AND group_id = ?`, cert.ID, group.ID)
		if err != nil {
			return err
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if rowsAffected == 0 {
			return api.StatusErrorf(http.StatusNotFound, "User %q is not a member of group %q", userNameOrID, groupName)
		}
	} else {
		err = cluster.CreateGroupCertificates(ctx, tx.Tx(), []cluster.GroupCertificate{groupCertificate})
		if err != nil {
			var dqliteErr dqliteDriver.Error
			// Detect SQLITE_CONSTRAINT_UNIQUE (2067) errors.
			if errors.As(err, &dqliteErr) && dqliteErr.Code == 2067 {
				return api.StatusErrorf(http.StatusConflict, "User %q is already a member of group %q", userNameOrID, groupName)
			}

			return err
		}
	}

	return nil
}

func getUniqueCertByNameOrFingerprintPrefix(ctx context.Context, tx *sql.Tx, nameOrFingerprintPrefix string) (*cluster.Certificate, error) {
	// Try to get by certificate fingerprint first
	cert, err := cluster.GetCertificateByFingerprintPrefix(ctx, tx, nameOrFingerprintPrefix)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return nil, err
	} else if err == nil {
		return cert, nil
	}

	// Try to get by certificate name.
	certType := certificate.TypeClient
	certs, err := cluster.GetCertificates(ctx, tx, cluster.CertificateFilter{
		Name: &nameOrFingerprintPrefix,
		Type: &certType,
	})
	if err != nil {
		return nil, err
	}

	if len(certs) == 0 {
		return nil, api.StatusErrorf(http.StatusNotFound, "Could not find a TLS certificate for the given fingerprint or name", nameOrFingerprintPrefix)
	} else if len(certs) > 1 {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Multiple client certificates have name %q, requires fingerprint", nameOrFingerprintPrefix)
	}

	return &certs[0], nil
}

func groupGrantEntitlement(d *Daemon, r *http.Request) response.Response {
	muxVars := mux.Vars(r)
	groupName, err := url.PathUnescape(muxVars["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	objectStr, err := url.PathUnescape(muxVars["object"])
	if err != nil {
		return response.SmartError(err)
	}

	relation, err := url.PathUnescape(muxVars["relation"])
	if err != nil {
		return response.SmartError(err)
	}

	object, err := entitlement.ObjectFromString(objectStr)
	if err != nil {
		return response.SmartError(err)
	}

	err = object.Type().ValidateRelation(entitlement.Relation(relation))
	if err != nil {
		return response.BadRequest(err)
	}

	s := d.State()

	//ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	//defer cancel()

	err = s.DB.Cluster.Transaction(context.Background(), func(ctx context.Context, tx *db.ClusterTx) error {
		return manageGroupEntitlement(ctx, tx, groupName, object, relation, false)
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func groupRevokeEntitlement(d *Daemon, r *http.Request) response.Response {
	muxVars := mux.Vars(r)
	groupName, err := url.PathUnescape(muxVars["groupName"])
	if err != nil {
		return response.SmartError(err)
	}

	objectStr, err := url.PathUnescape(muxVars["object"])
	if err != nil {
		return response.SmartError(err)
	}

	relation, err := url.PathUnescape(muxVars["relation"])
	if err != nil {
		return response.SmartError(err)
	}

	object, err := entitlement.ObjectFromString(objectStr)
	if err != nil {
		return response.SmartError(err)
	}

	err = object.Type().ValidateRelation(entitlement.Relation(relation))
	if err != nil {
		return response.BadRequest(err)
	}

	s := d.State()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err = s.DB.Cluster.Transaction(ctx, func(ctx context.Context, tx *db.ClusterTx) error {
		return manageGroupEntitlement(ctx, tx, groupName, object, relation, true)
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.EmptySyncResponse
}

func manageGroupEntitlement(ctx context.Context, tx *db.ClusterTx, groupName string, object entitlement.Object, relation string, revoke bool) error {
	group, err := cluster.GetGroup(ctx, tx.Tx(), groupName)
	if err != nil {
		return err
	}

	objectType := object.Type()
	objectTypeStr := string(objectType)
	objectRef := object.Ref()
	var dbEntitlement *cluster.Entitlement
	var entityID int
	if objectType == entitlement.ObjectTypeServer {
		dbEntitlement, err = cluster.GetEntitlement(ctx, tx.Tx(), relation, objectTypeStr, objectRef, nil)
	} else {
		entityID, err = tx.GetAuthObjectEntityID(ctx, object)
		if err != nil {
			return err
		}

		dbEntitlement, err = cluster.GetEntitlement(ctx, tx.Tx(), relation, objectTypeStr, objectRef, &entityID)
	}

	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		// Unrelated DB error.
		return err
	} else if err != nil && revoke {
		// Return not found error if revoking, as it should already exist.
		return err
	} else if err != nil {
		// Create the entitlement.
		dbEntitlement = &cluster.Entitlement{
			Relation:   relation,
			ObjectType: objectTypeStr,
			ObjectRef:  object.Ref(),
		}

		if entityID != 0 {
			dbEntitlement.EntityID = &entityID
		}

		id, err := cluster.CreateEntitlement(ctx, tx.Tx(), *dbEntitlement)
		if err != nil {
			return err
		}

		dbEntitlement.ID = id
	}

	if revoke {
		res, err := tx.Tx().ExecContext(ctx, `DELETE FROM groups_entitlements WHERE group_id = ? AND entitlement_id = ?`, group.ID, dbEntitlement.ID)
		if err != nil {
			return err
		}

		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if rowsAffected == 0 {
			return api.StatusErrorf(http.StatusNotFound, "Group %q does not have entitlement %q on object %q", groupName, relation, object)
		}
	} else {
		groupEntitlement := cluster.GroupEntitlement{
			GroupID:       group.ID,
			EntitlementID: dbEntitlement.ID,
		}

		err = cluster.CreateGroupEntitlements(ctx, tx.Tx(), []cluster.GroupEntitlement{groupEntitlement})
		if err != nil {
			return err
		}
	}

	return nil
}
