package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/lxd/lifecycle"
	"github.com/canonical/lxd/lxd/project"
	"github.com/canonical/lxd/lxd/project/limits"
	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/lxd/response"
	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/validate"
)

var placementGroupsCmd = APIEndpoint{
	Path:        "placement-groups",
	MetricsType: entity.TypePlacementGroup,

	Get:  APIEndpointAction{Handler: placementGroupsGet, AccessHandler: allowProjectResourceList},
	Post: APIEndpointAction{Handler: placementGroupsPost, AccessHandler: allowPermission(entity.TypeProject, auth.EntitlementCanCreatePlacementGroups)},
}

var placementGroupCmd = APIEndpoint{
	Path:        "placement-groups/{placementGroupName}",
	MetricsType: entity.TypePlacementGroup,

	Delete: APIEndpointAction{Handler: placementGroupDelete, AccessHandler: allowPermission(entity.TypePlacementGroup, auth.EntitlementCanDelete, "placementGroupName")},
	Get:    APIEndpointAction{Handler: placementGroupGet, AccessHandler: allowPermission(entity.TypePlacementGroup, auth.EntitlementCanView, "placementGroupName")},
	Put:    APIEndpointAction{Handler: placementGroupPut, AccessHandler: allowPermission(entity.TypePlacementGroup, auth.EntitlementCanEdit, "placementGroupName")},
	Patch:  APIEndpointAction{Handler: placementGroupPut, AccessHandler: allowPermission(entity.TypePlacementGroup, auth.EntitlementCanEdit, "placementGroupName")},
	Post:   APIEndpointAction{Handler: placementGroupPost, AccessHandler: allowPermission(entity.TypePlacementGroup, auth.EntitlementCanEdit, "placementGroupName")},
}

// API endpoints.

// swagger:operation GET /1.0/placement-groups placement-groups placement_groups_get
//
//  Get the placement groups
//
//  Returns a list of placement groups (URLs).
//
//  ---
//  produces:
//    - application/json
//  parameters:
//    - in: query
//      name: project
//      description: Project name
//      type: string
//      example: default
//    - in: query
//      name: all-projects
//      description: Retrieve placement groups from all projects
//      type: boolean
//      example: true
//  responses:
//    "200":
//      description: API endpoints
//      schema:
//        type: object
//        description: Sync response
//        properties:
//          type:
//            type: string
//            description: Response type
//            example: sync
//          status:
//            type: string
//            description: Status description
//            example: Success
//          status_code:
//            type: integer
//            description: Status code
//            example: 200
//          metadata:
//            type: array
//            description: List of endpoints
//            items:
//              type: string
//            example: |-
//              [
//                "/1.0/placement-groups/group1",
//                "/1.0/placement-groups/group2"
//              ]
//    "403":
//      $ref: "#/responses/Forbidden"
//    "500":
//      $ref: "#/responses/InternalServerError"

// swagger:operation GET /1.0/placement-groups?recursion=1 placement-groups placement_groups_get_recursion1
//
//	Get the placement groups
//
//	Returns a list of placement groups (structs).
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	  - in: query
//	    name: all-projects
//	    description: Retrieve placement groups from all projects
//	    type: boolean
//	    example: true
//	responses:
//	  "200":
//	    description: API endpoints
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          type: array
//	          description: List of placement groups
//	          items:
//	            $ref: "#/definitions/PlacementGroup"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func placementGroupsGet(d *Daemon, r *http.Request) response.Response {
	s := d.State()

	allProjects := shared.IsTrue(request.QueryParam(r, "all-projects"))
	projectName := request.QueryParam(r, "project")
	if allProjects && projectName != "" {
		return response.BadRequest(errors.New("Cannot specify a project when requesting all projects"))
	}

	if !allProjects && projectName == "" {
		projectName = api.ProjectDefaultName
	}

	recursion := util.IsRecursionRequest(r)
	withEntitlements, err := extractEntitlementsFromQuery(r, entity.TypePlacementGroup, true)
	if err != nil {
		return response.SmartError(err)
	}

	canViewPlacementGroup, err := s.Authorizer.GetPermissionChecker(r.Context(), auth.EntitlementCanView, entity.TypePlacementGroup)
	if err != nil {
		return response.InternalError(err)
	}

	var placementGroups []cluster.PlacementGroup
	var usedByMap map[string]map[string][]string
	var placementGroupNames map[string][]string
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		var projectFilter *string
		if !allProjects {
			projectFilter = &projectName
		}

		if !recursion {
			placementGroupNames, err = cluster.GetPlacementGroupNames(ctx, tx.Tx(), projectFilter)
			return err
		}

		var filters []cluster.PlacementGroupFilter
		if !allProjects {
			filters = append(filters, cluster.PlacementGroupFilter{Project: &projectName})
		}

		placementGroups, err = cluster.GetPlacementGroups(ctx, tx.Tx(), filters...)
		if err != nil {
			return err
		}

		usedByMap, err = cluster.GetAllPlacementGroupUsedByURLs(ctx, tx.Tx(), projectFilter)
		return err
	})
	if err != nil {
		return response.SmartError(err)
	}

	if !recursion {
		var urls []string
		for projectName, groups := range placementGroupNames {
			for _, placementGroup := range groups {
				u := api.NewURL().Project(projectName).Path("1.0", "placement-groups", placementGroup)
				if !canViewPlacementGroup(u) {
					continue
				}

				urls = append(urls, u.String())
			}
		}

		return response.SyncResponse(true, urls)
	}

	apiGroups := make([]*api.PlacementGroup, 0, len(placementGroups))
	entitlementReportingMap := make(map[*api.URL]auth.EntitlementReporter)
	for _, placementGroup := range placementGroups {
		u := entity.PlacementGroupURL(placementGroup.Project, placementGroup.Name)
		if !canViewPlacementGroup(u) {
			continue
		}

		apiGroup := placementGroup.ToAPIBase()
		apiGroup.UsedBy = project.FilterUsedBy(s.Authorizer, r, usedByMap[placementGroup.Project][placementGroup.Name])
		apiGroups = append(apiGroups, &apiGroup)
		entitlementReportingMap[u] = &apiGroup
	}

	if len(withEntitlements) > 0 {
		err = reportEntitlements(r.Context(), s.Authorizer, s.IdentityCache, entity.TypePlacementGroup, withEntitlements, entitlementReportingMap)
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.SyncResponse(true, apiGroups)
}

// swagger:operation POST /1.0/placement-groups placement-groups placement_groups_post
//
//	Add a placement group
//
//	Creates a new placement group.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	  - in: body
//	    name: placementGroup
//	    description: The new placement group
//	    required: true
//	    schema:
//	      $ref: "#/definitions/PlacementGroupsPost"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func placementGroupsPost(d *Daemon, r *http.Request) response.Response {
	s := d.State()
	if !s.ServerClustered {
		return response.BadRequest(errors.New("This server is not clustered"))
	}

	if s.GlobalConfig.InstancesPlacementScriptlet() != "" {
		return response.Conflict(errors.New("Cannot create placement groups when an instance placement scriptlet is defined"))
	}

	req := api.PlacementGroupsPost{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	err = validate.IsDeviceName(req.Name)
	if err != nil {
		return response.BadRequest(err)
	}

	projectName := request.ProjectParam(r)
	newGroup := cluster.PlacementGroup{
		Name:         req.Name,
		Description:  req.Description,
		Policy:       cluster.PlacementPolicy(req.Policy),
		Scope:        cluster.PlacementScope(req.Scope),
		Rigor:        cluster.PlacementRigor(req.Rigor),
		Project:      projectName,
		ClusterGroup: req.ClusterGroup,
	}

	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		err := validatePlacementGroup(ctx, tx.Tx(), newGroup)
		if err != nil {
			return err
		}

		_, err = cluster.CreatePlacementGroup(ctx, tx.Tx(), newGroup)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	lc := lifecycle.PlacementGroupCreated.Event(projectName, req.Name, request.CreateRequestor(r), nil)
	s.Events.SendLifecycle(projectName, lc)

	return response.SyncResponseLocation(true, nil, lc.Source)
}

// swagger:operation DELETE /1.0/placement-groups/{name} placement-groups placement_group_delete
//
//	Delete the placement group
//
//	Removes the placement group.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func placementGroupDelete(d *Daemon, r *http.Request) response.Response {
	s := d.State()

	projectName := request.ProjectParam(r)
	placementGroupName, err := url.PathUnescape(mux.Vars(r)["placementGroupName"])
	if err != nil {
		return response.SmartError(err)
	}

	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		_, err := cluster.GetPlacementGroupID(ctx, tx.Tx(), placementGroupName, projectName)
		if err != nil {
			return err
		}

		usedBy, err := cluster.GetPlacementGroupUsedBy(ctx, tx.Tx(), projectName, placementGroupName)
		if err != nil {
			return err
		}

		if len(usedBy) > 0 {
			return api.StatusErrorf(http.StatusBadRequest, "Placement group %q is currently in use", placementGroupName)
		}

		return cluster.DeletePlacementGroup(ctx, tx.Tx(), placementGroupName, projectName)
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.Events.SendLifecycle(projectName, lifecycle.PlacementGroupDeleted.Event(projectName, placementGroupName, request.CreateRequestor(r), nil))

	return response.EmptySyncResponse
}

// swagger:operation GET /1.0/placement-groups/{name} placement-groups placement_group_get
//
//	Get the placement group
//
//	Gets a specific placement group.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	responses:
//	  "200":
//	    description: zone
//	    schema:
//	      type: object
//	      description: Sync response
//	      properties:
//	        type:
//	          type: string
//	          description: Response type
//	          example: sync
//	        status:
//	          type: string
//	          description: Status description
//	          example: Success
//	        status_code:
//	          type: integer
//	          description: Status code
//	          example: 200
//	        metadata:
//	          $ref: "#/definitions/PlacementGroup"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func placementGroupGet(d *Daemon, r *http.Request) response.Response {
	s := d.State()

	projectName := request.ProjectParam(r)
	placementGroupName, err := url.PathUnescape(mux.Vars(r)["placementGroupName"])
	if err != nil {
		return response.SmartError(err)
	}

	withEntitlements, err := extractEntitlementsFromQuery(r, entity.TypePlacementGroup, false)
	if err != nil {
		return response.SmartError(err)
	}

	var placementGroup *api.PlacementGroup
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		dbGroup, err := cluster.GetPlacementGroup(ctx, tx.Tx(), placementGroupName, projectName)
		if err != nil {
			return err
		}

		placementGroup, err = dbGroup.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	etag := *placementGroup
	placementGroup.UsedBy = project.FilterUsedBy(s.Authorizer, r, placementGroup.UsedBy)
	if len(withEntitlements) > 0 {
		err = reportEntitlements(r.Context(), s.Authorizer, s.IdentityCache, entity.TypePlacementGroup, withEntitlements, map[*api.URL]auth.EntitlementReporter{entity.PlacementGroupURL(projectName, placementGroupName): placementGroup})
		if err != nil {
			return response.SmartError(err)
		}
	}

	return response.SyncResponseETag(true, placementGroup, etag)
}

// swagger:operation PATCH /1.0/placement-groups/{name} placement-groups placement_group_patch
//
//  Partially update the placement group
//
//  Updates a subset of the placement group configuration.
//
//  ---
//  consumes:
//    - application/json
//  produces:
//    - application/json
//  parameters:
//    - in: query
//      name: project
//      description: Project name
//      type: string
//      example: default
//    - in: body
//      name: placement group
//      description: placement group
//      required: true
//      schema:
//        $ref: "#/definitions/PlacementGroupPut"
//  responses:
//    "200":
//      $ref: "#/responses/EmptySyncResponse"
//    "400":
//      $ref: "#/responses/BadRequest"
//    "403":
//      $ref: "#/responses/Forbidden"
//    "412":
//      $ref: "#/responses/PreconditionFailed"
//    "500":
//      $ref: "#/responses/InternalServerError"

// swagger:operation PUT /1.0/placement-groups/{name} placement-groups placement_group_put
//
//	Update the placement group
//
//	Updates the entire placement group.
//
//	---
//	consumes:
//	  - application/json
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	  - in: body
//	    name: placement group
//	    description: placement group
//	    required: true
//	    schema:
//	      $ref: "#/definitions/PlacementGroupPut"
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "412":
//	    $ref: "#/responses/PreconditionFailed"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func placementGroupPut(d *Daemon, r *http.Request) response.Response {
	s := d.State()

	projectName := request.ProjectParam(r)
	placementGroupName, err := url.PathUnescape(mux.Vars(r)["placementGroupName"])
	if err != nil {
		return response.SmartError(err)
	}

	req := api.PlacementGroupPut{}
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	var existing *api.PlacementGroup
	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		dbGroup, err := cluster.GetPlacementGroup(ctx, tx.Tx(), placementGroupName, projectName)
		if err != nil {
			return err
		}

		existing, err = dbGroup.ToAPI(ctx, tx.Tx())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	err = util.EtagCheck(r, existing)
	if err != nil {
		return response.SmartError(err)
	}

	duplicate := *existing
	switch r.Method {
	case http.MethodPut:
		duplicate.Description = req.Description
		duplicate.Policy = req.Policy
		duplicate.Scope = req.Scope
		duplicate.ClusterGroup = req.ClusterGroup
		duplicate.Rigor = req.Rigor
	case http.MethodPatch:
		if req.Description != "" {
			duplicate.Description = req.Description
		}

		if req.Policy != "" {
			duplicate.Policy = req.Policy
		}

		if req.Scope != "" {
			duplicate.Scope = req.Scope
		}

		if req.ClusterGroup != "" {
			duplicate.ClusterGroup = req.ClusterGroup
		}

		if req.Rigor != "" {
			duplicate.Rigor = req.Rigor
		}
	}

	dbPlacementGroup := cluster.PlacementGroup{
		Name:         duplicate.Name,
		Description:  duplicate.Description,
		Policy:       cluster.PlacementPolicy(duplicate.Policy),
		Scope:        cluster.PlacementScope(duplicate.Scope),
		Rigor:        cluster.PlacementRigor(duplicate.Rigor),
		Project:      duplicate.Project,
		ClusterGroup: duplicate.ClusterGroup,
	}

	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		err = validatePlacementGroup(ctx, tx.Tx(), dbPlacementGroup)
		if err != nil {
			return err
		}

		err = cluster.UpdatePlacementGroup(ctx, tx.Tx(), placementGroupName, projectName, dbPlacementGroup)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.Events.SendLifecycle(projectName, lifecycle.PlacementGroupUpdated.Event(projectName, placementGroupName, request.CreateRequestor(r), nil))

	return response.EmptySyncResponse
}

// swagger:operation POST /1.0/placement-groups/{name} placement-groups placement_group_post
//
//	Rename a placement group
//
//	Renames the placement group.
//
//	---
//	produces:
//	  - application/json
//	parameters:
//	  - in: query
//	    name: project
//	    description: Project name
//	    type: string
//	    example: default
//	responses:
//	  "200":
//	    $ref: "#/responses/EmptySyncResponse"
//	  "400":
//	    $ref: "#/responses/BadRequest"
//	  "403":
//	    $ref: "#/responses/Forbidden"
//	  "500":
//	    $ref: "#/responses/InternalServerError"
func placementGroupPost(d *Daemon, r *http.Request) response.Response {
	s := d.State()

	projectName := request.ProjectParam(r)
	placementGroupName, err := url.PathUnescape(mux.Vars(r)["placementGroupName"])
	if err != nil {
		return response.SmartError(err)
	}

	req := api.PlacementGroupPost{}
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return response.BadRequest(err)
	}

	err = validate.IsDeviceName(req.Name)
	if err != nil {
		return response.BadRequest(err)
	}

	err = s.DB.Cluster.Transaction(r.Context(), func(ctx context.Context, tx *db.ClusterTx) error {
		_, err := cluster.GetPlacementGroupID(ctx, tx.Tx(), placementGroupName, projectName)
		if err != nil {
			return err
		}

		usedBy, err := cluster.GetPlacementGroupUsedBy(ctx, tx.Tx(), projectName, placementGroupName)
		if err != nil {
			return err
		}

		if len(usedBy) > 0 {
			return api.StatusErrorf(http.StatusBadRequest, "Placement group %q is currently in use", placementGroupName)
		}

		return cluster.RenamePlacementGroup(ctx, tx.Tx(), placementGroupName, projectName, req.Name)
	})
	if err != nil {
		return response.SmartError(err)
	}

	s.Events.SendLifecycle(projectName, lifecycle.PlacementGroupRenamed.Event(projectName, placementGroupName, request.CreateRequestor(r), nil))

	return response.SyncResponseLocation(true, nil, api.NewURL().Project(projectName).Path("1.0", "placement-groups", req.Name).String())
}

// validatePlacementGroup returns an API friendly error if the given cluster.PlacementGroup is not valid.
// Some of these checks are enforced already by the database schema and types, but this function will return a more
// succinct error message and the correct status code.
func validatePlacementGroup(ctx context.Context, tx *sql.Tx, placementGroup cluster.PlacementGroup) error {
	err := validate.IsDeviceName(placementGroup.Name)
	if err != nil {
		return err
	}

	_, err = placementGroup.Policy.Value()
	if err != nil {
		return err
	}

	_, err = placementGroup.Scope.Value()
	if err != nil {
		return err
	}

	_, err = placementGroup.Rigor.Value()
	if err != nil {
		return err
	}

	dbProject, err := cluster.GetProject(ctx, tx, placementGroup.Project)
	if err != nil {
		return err
	}

	apiProject, err := dbProject.ToAPI(ctx, tx)
	if err != nil {
		return err
	}

	err = limits.AllowClusterGroup(apiProject, placementGroup.ClusterGroup)
	if err != nil {
		return err
	}

	exists, err := cluster.ClusterGroupExists(ctx, tx, placementGroup.ClusterGroup)
	if err != nil {
		return err
	}

	if !exists {
		return api.StatusErrorf(http.StatusBadRequest, "No cluster group with name %q", placementGroup.ClusterGroup)
	}

	return nil
}
