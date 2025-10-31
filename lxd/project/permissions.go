package project

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/lxd/auth"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/entity"
	"github.com/canonical/lxd/shared/logger"
)

// CheckRestrictedDevicesDiskPaths checks whether the disk's source path is within the allowed paths specified in
// the project's restricted.devices.disk.paths config setting.
// If no allowed paths are specified in project, then it allows all paths, and returns true and empty string.
// If allowed paths are specified, and one matches, returns true and the matching allowed parent source path.
// Otherwise if sourcePath not allowed returns false and empty string.
func CheckRestrictedDevicesDiskPaths(projectConfig map[string]string, sourcePath string) (bool, string) {
	if projectConfig["restricted.devices.disk.paths"] == "" {
		return true, ""
	}

	// Clean, then add trailing slash, to ensure we are prefix matching on whole path.
	sourcePath = filepath.Clean(shared.HostPath(sourcePath)) + "/"
	for parentSourcePath := range strings.SplitSeq(projectConfig["restricted.devices.disk.paths"], ",") {
		// Clean, then add trailing slash, to ensure we are prefix matching on whole path.
		parentSourcePathTrailing := filepath.Clean(shared.HostPath(parentSourcePath)) + "/"
		if strings.HasPrefix(sourcePath, parentSourcePathTrailing) {
			return true, parentSourcePath
		}
	}

	return false, ""
}

// FilterUsedBy filters a UsedBy list based on the entities that the requestor is able to view.
func FilterUsedBy(ctx context.Context, authorizer auth.Authorizer, entries []auth.Entity) []string {
	// Get a map of URLs by entity type. If there are multiple entries of a particular entity type we can reduce the
	// number of calls to the authorizer.
	entitiesByType := make(map[entity.Type][]auth.Entity)
	for _, entry := range entries {
		entityType := entry.EntityType()
		entitiesByType[entityType] = append(entitiesByType[entityType], entry)
	}

	// Filter the entries.
	usedBy := make([]string, 0, len(entries))

	for entityType, entriesOfType := range entitiesByType {
		// If only one entry of this type, check directly.
		if len(entriesOfType) == 1 {
			err := authorizer.CheckPermission(ctx, auth.EntitlementCanView, entriesOfType[0].EntityType(), entriesOfType[0].DatabaseID())
			if err != nil {
				continue
			}

			usedBy = append(usedBy, entriesOfType[0].URL().String())
			continue
		}

		// Otherwise get a permission checker for the entity type.
		canViewEntity, err := authorizer.GetPermissionChecker(ctx, auth.EntitlementCanView, entityType)
		if err != nil {
			logger.Error("Failed to get permission checker for project used-by filtering", logger.Ctx{"entity_type": entityType, "err": err})
			continue
		}

		// Check each url and append.
		for _, u := range entriesOfType {
			if canViewEntity(u.DatabaseID()) {
				usedBy = append(usedBy, u.URL().String())
			}
		}
	}

	return usedBy
}
