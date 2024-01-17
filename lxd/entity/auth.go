package entity

import (
	"fmt"
	"github.com/canonical/lxd/shared"
)

func (t Type) AuthObject(projectName string, location string, pathArgs ...string) string {
	u, _ := t.URL(projectName, location, pathArgs...)
	return fmt.Sprintf("%s:%s", t.String(), u)
}

// Entitlement is a type representation of a permission as it applies to a particular ObjectType.
type Entitlement string

const (
	// Relations that apply to all resources.
	EntitlementCanEdit Entitlement = "can_edit"
	EntitlementCanView Entitlement = "can_view"

	// Server entitlements.
	EntitlementAdmin                               Entitlement = "admin"
	EntitlementOperator                            Entitlement = "operator"
	EntitlementViewer                              Entitlement = "viewer"
	EntitlementCanManagePermissions                Entitlement = "can_manage_permissions"
	EntitlementCanManageStoragePools               Entitlement = "can_manage_storage_pools"
	EntitlementCanManageProjects                   Entitlement = "can_manage_projects"
	EntitlementCanViewResources                    Entitlement = "can_view_resources"
	EntitlementCanManageCertificates               Entitlement = "can_manage_certificates"
	EntitlementCanViewMetrics                      Entitlement = "can_view_metrics"
	EntitlementCanOverrideClusterTargetRestriction Entitlement = "can_override_cluster_target_restriction"
	EntitlementCanViewPrivilegedEvents             Entitlement = "can_view_privileged_events"
	EntitlementCanViewWarnings                     Entitlement = "can_view_warnings"

	// Project entitlements.
	EntitlementManager                 Entitlement = "manager"
	EntitlementCanManageImages         Entitlement = "can_manage_images"
	EntitlementCanManageImageAliases   Entitlement = "can_manage_image_aliases"
	EntitlementCanManageInstances      Entitlement = "can_manage_instances"
	EntitlementCanManageNetworks       Entitlement = "can_manage_networks"
	EntitlementCanManageNetworkACLs    Entitlement = "can_manage_network_acls"
	EntitlementCanManageNetworkZones   Entitlement = "can_manage_network_zones"
	EntitlementCanManageProfiles       Entitlement = "can_manage_profiles"
	EntitlementCanManageStorageVolumes Entitlement = "can_manage_storage_volumes"
	EntitlementCanManageStorageBuckets Entitlement = "can_manage_storage_buckets"
	EntitlementCanViewOperations       Entitlement = "can_view_operations"
	EntitlementCanViewEvents           Entitlement = "can_view_events"

	// Instance entitlements.
	EntitlementUser             Entitlement = "user"
	EntitlementCanUpdateState   Entitlement = "can_update_state"
	EntitlementCanConnectSFTP   Entitlement = "can_connect_sftp"
	EntitlementCanAccessFiles   Entitlement = "can_access_files"
	EntitlementCanAccessConsole Entitlement = "can_access_console"
	EntitlementCanExec          Entitlement = "can_exec"

	// Instance and storage volume entitlements.
	EntitlementCanManageSnapshots Entitlement = "can_manage_snapshots"
	EntitlementCanManageBackups   Entitlement = "can_manage_backups"

	// Object to object Entitlements
	EntitlementServer  Entitlement = "server"
	EntitlementProject Entitlement = "project"

	// User and group Entitlements
	EntitlementMember Entitlement = "member"
)

func (t Type) Entitlements() []Entitlement {
	switch t {
	case TypeAuthGroup:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeServer:
		return []Entitlement{
			EntitlementAdmin,
			EntitlementOperator,
			EntitlementViewer,
			EntitlementCanEdit,
			EntitlementCanView,
			EntitlementCanManagePermissions,
			EntitlementCanManageStoragePools,
			EntitlementCanManageProjects,
			EntitlementCanViewResources,
			EntitlementCanManageCertificates,
			EntitlementCanViewMetrics,
			EntitlementCanOverrideClusterTargetRestriction,
			EntitlementCanViewPrivilegedEvents,
			EntitlementCanViewWarnings,
		}
	case TypeCertificate:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeStoragePool:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeProject:
		return []Entitlement{
			EntitlementManager,
			EntitlementOperator,
			EntitlementViewer,
			EntitlementCanManageImages,
			EntitlementCanManageImageAliases,
			EntitlementCanManageInstances,
			EntitlementCanManageNetworks,
			EntitlementCanManageNetworkACLs,
			EntitlementCanManageNetworkZones,
			EntitlementCanManageProfiles,
			EntitlementCanManageStorageVolumes,
			EntitlementCanManageStorageBuckets,
			EntitlementCanViewOperations,
			EntitlementCanViewEvents,
		}
	case TypeImage:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeImageAlias:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeInstance:
		return []Entitlement{
			EntitlementManager,
			EntitlementOperator,
			EntitlementUser,
			EntitlementViewer,
			EntitlementCanEdit,
			EntitlementCanView,
			EntitlementCanUpdateState,
			EntitlementCanManageSnapshots,
			EntitlementCanManageBackups,
			EntitlementCanConnectSFTP,
			EntitlementCanAccessFiles,
			EntitlementCanAccessConsole,
			EntitlementCanExec,
		}
	case TypeNetwork:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeNetworkACL:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeNetworkZone:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeProfile:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	case TypeStorageVolume:
		return []Entitlement{
			EntitlementCanView,
			EntitlementCanEdit,
			EntitlementCanManageSnapshots,
			EntitlementCanManageBackups,
		}
	case TypeStorageBucket:
		return []Entitlement{EntitlementCanView, EntitlementCanEdit}
	}

	return nil
}

func (t Type) ValidateEntitlement(entitlement Entitlement) error {
	entitlements := t.Entitlements()
	if !shared.ValueInSlice(entitlement, entitlements) {
		return fmt.Errorf("No such entitlement %q for objects of type %q", entitlement, t)
	}

	return nil
}
