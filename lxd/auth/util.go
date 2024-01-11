package auth

import (
	"fmt"
	"github.com/canonical/lxd/shared/entitlement"
	"github.com/canonical/lxd/shared/version"
	"github.com/gorilla/mux"
	"net/http"
	"net/url"
	"strings"
)

// ObjectFromRequest returns an object created from the request by evaluating the given mux vars.
// Mux vars must be provided in the order that they are found in the endpoint path. If the object
// requires a project name, this is taken from the project query parameter unless the URL begins
// with /1.0/projects.
func ObjectFromRequest(r *http.Request, objectType entitlement.ObjectType, muxVars ...string) (entitlement.Object, error) {
	// Shortcut for server objects which don't require any arguments.
	if objectType == entitlement.ObjectTypeServer {
		return entitlement.ObjectServer(), nil
	}

	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", err
	}

	projectName := values.Get("project")
	if projectName == "" {
		projectName = "default"
	}

	location := values.Get("target")

	muxValues := make([]string, 0, len(muxVars))
	vars := mux.Vars(r)
	for _, muxVar := range muxVars {
		var err error
		var muxValue string

		if muxVar == "location" {
			// Special handling for the location which is not present as a real mux var.
			if location == "" {
				continue
			}

			muxValue = location
		} else {
			muxValue, err = url.PathUnescape(vars[muxVar])
			if err != nil {
				return "", fmt.Errorf("Failed to unescape mux var %q for object type %q: %w", muxVar, objectType, err)
			}

			if muxValue == "" {
				return "", fmt.Errorf("Mux var %q not found for object type %q", muxVar, objectType)
			}
		}

		muxValues = append(muxValues, muxValue)
	}

	// If using projects API we want to pass in the mux var, not the query parameter.
	if objectType == entitlement.ObjectTypeProject && strings.HasPrefix(r.URL.Path, fmt.Sprintf("/%s/projects", version.APIVersion)) {
		if len(muxValues) == 0 {
			return "", fmt.Errorf("Missing project name path variable")
		}

		return entitlement.ObjectProject(muxValues[0]), nil
	}

	return entitlement.NewObject(objectType, projectName, muxValues...)
}
