package main

import (
	"context"
	"fmt"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entitlement"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
	"strings"
)

var entitlementsCmd = APIEndpoint{
	Name: "entitlements",
	Path: "entitlements",
	Get: APIEndpointAction{
		AllowUntrusted: true,
		Handler:        getEntitlements,
	},
}

var entitlementsByObjectCmd = APIEndpoint{
	Name: "entitlements_object",
	Path: "entitlements/{object}",
	Get: APIEndpointAction{
		AllowUntrusted: true,
		Handler:        getEntitlementsByObject,
	},
}

func getEntitlements(d *Daemon, r *http.Request) response.Response {
	entitlementMap := make(map[entitlement.Object]map[entitlement.Relation][]string)
	var nEntitlements int
	err := d.db.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		rows, err := tx.Tx().QueryContext(ctx, `
SELECT groups.name, printf('%s:%s', entitlements.object_type, entitlements.object_ref), entitlements.relation FROM groups_entitlements
	JOIN groups ON groups_entitlements.group_id = groups.id JOIN entitlements ON groups_entitlements.entitlement_id = entitlements.id
`)
		if err != nil {
			return err
		}

		for rows.Next() {
			var groupName string
			var object entitlement.Object
			var relation entitlement.Relation
			err = rows.Scan(&groupName, &object, &relation)
			if err != nil {
				return err
			}

			relations, ok := entitlementMap[object]
			if !ok {
				entitlementMap[object] = map[entitlement.Relation][]string{}
				for _, relation := range object.Type().Relations() {
					entitlementMap[object][relation] = []string{}
				}

				entitlementMap[object][relation] = append(entitlementMap[object][relation], groupName)
				continue
			}

			groups, ok := relations[relation]
			if !ok {
				relations[relation] = []string{groupName}
				continue
			}

			groups = append(groups, groupName)
		}

		rows, err = tx.Tx().QueryContext(ctx, `
SELECT printf('%s:%s', object_type, object_ref) FROM openfga_tuple_ref WHERE object_type NOT IN ('user')
`)
		if err != nil {
			return err
		}

		for rows.Next() {
			nEntitlements++
			var object entitlement.Object
			err = rows.Scan(&object)
			if err != nil {
				return err
			}

			_, ok := entitlementMap[object]
			if !ok {
				entitlementMap[object] = map[entitlement.Relation][]string{}
				for _, relation := range object.Type().Relations() {
					entitlementMap[object][relation] = []string{}
				}
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	entitlements := make([]api.Entitlement, 0, len(entitlementMap))
	for object, relations := range entitlementMap {
		for relation, groups := range relations {
			entitlements = append(entitlements, api.Entitlement{
				Object:   object,
				Relation: relation,
				Groups:   groups,
			})
		}
	}

	return response.SyncResponse(true, entitlements)
}

func getEntitlementsByObject(d *Daemon, r *http.Request) response.Response {
	objectStr, err := url.PathUnescape(mux.Vars(r)["object"])
	if err != nil {
		return response.SmartError(err)
	}

	objectType, _, isObject := strings.Cut(objectStr, ":")

	var object entitlement.Object
	if isObject {
		object, err = entitlement.ObjectFromString(objectStr)
		if err != nil {
			return response.SmartError(err)
		}

		if len(object.Type().Relations()) == 0 {
			return response.SmartError(fmt.Errorf("Object type %q has no relations", object.Type()))
		}
	} else {
		if len(entitlement.ObjectType(objectType).Relations()) == 0 {
			return response.SmartError(fmt.Errorf("Object type %q has no relations", objectType))
		}
	}

	entitlements := make(map[entitlement.Object]map[entitlement.Relation][]string)
	err = d.db.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		var q string
		var args []any
		if isObject {
			args = []any{object.Type(), object.Ref()}
			q = `
SELECT groups.name, printf('%s:%s', entitlements.object_type, entitlements.object_ref), entitlements.relation FROM groups_entitlements
	JOIN groups ON groups_entitlements.group_id = groups.id 
	JOIN entitlements ON groups_entitlements.entitlement_id = entitlements.id 
	WHERE entitlements.object_type = ? AND entitlements.object_ref = ?
`
		} else {
			args = []any{objectType}
			q = `
SELECT groups.name, printf('%s:%s', entitlements.object_type, entitlements.object_ref), entitlements.relation FROM groups_entitlements
	JOIN groups ON groups_entitlements.group_id = groups.id 
	JOIN entitlements ON groups_entitlements.entitlement_id = entitlements.id 
	WHERE entitlements.object_type = ?
`
		}
		rows, err := tx.Tx().QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}

		for rows.Next() {
			var groupName string
			var object entitlement.Object
			var relation entitlement.Relation
			err = rows.Scan(&groupName, &object, &relation)
			if err != nil {
				return err
			}

			relations, ok := entitlements[object]
			if !ok {
				entitlements[object] = map[entitlement.Relation][]string{}
				for _, relation := range object.Type().Relations() {
					entitlements[object][relation] = []string{}
				}

				entitlements[object][relation] = append(entitlements[object][relation], groupName)
				continue
			}

			groups, ok := relations[relation]
			if !ok {
				relations[relation] = []string{groupName}
				continue
			}

			groups = append(groups, groupName)
		}

		if isObject {
			q = `SELECT printf('%s:%s', object_type, object_ref) FROM openfga_tuple_ref WHERE object_type NOT IN ('user') AND object_type = ? AND object_ref = ?`
		} else {
			q = `SELECT printf('%s:%s', object_type, object_ref) FROM openfga_tuple_ref WHERE object_type NOT IN ('user') AND object_type = ?`
		}

		rows, err = tx.Tx().QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}

		for rows.Next() {
			var object entitlement.Object
			err = rows.Scan(&object)
			if err != nil {
				return err
			}

			_, ok := entitlements[object]
			if !ok {
				entitlements[object] = map[entitlement.Relation][]string{}
				for _, relation := range object.Type().Relations() {
					entitlements[object][relation] = []string{}
				}
			}
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	return response.SyncResponse(true, entitlements)
}

func getEntitlement(d *Daemon, r *http.Request) response.Response {
	return nil
}
