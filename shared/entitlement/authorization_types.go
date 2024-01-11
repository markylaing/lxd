package entitlement

import (
	"fmt"
	"github.com/canonical/lxd/shared"
)

// Relation is a type representation of a permission as it applies to a particular ObjectType.
type Relation string

const (
	// Relations that apply to all resources.
	RelationCanEdit Relation = "can_edit"
	RelationCanView Relation = "can_view"

	// Server entitlements.
	RelationAdmin                               Relation = "admin"
	RelationOperator                            Relation = "operator"
	RelationViewer                              Relation = "viewer"
	RelationCanManagePermissions                Relation = "can_manage_permissions"
	RelationCanManageStoragePools               Relation = "can_manage_storage_pools"
	RelationCanManageProjects                   Relation = "can_manage_projects"
	RelationCanViewResources                    Relation = "can_view_resources"
	RelationCanManageCertificates               Relation = "can_manage_certificates"
	RelationCanViewMetrics                      Relation = "can_view_metrics"
	RelationCanOverrideClusterTargetRestriction Relation = "can_override_cluster_target_restriction"
	RelationCanViewPrivilegedEvents             Relation = "can_view_privileged_events"
	RelationCanViewWarnings                     Relation = "can_view_warnings"

	// Project entitlements.
	RelationManager                 Relation = "manager"
	RelationCanManageImages         Relation = "can_manage_images"
	RelationCanManageImageAliases   Relation = "can_manage_image_aliases"
	RelationCanManageInstances      Relation = "can_manage_instances"
	RelationCanManageNetworks       Relation = "can_manage_networks"
	RelationCanManageNetworkACLs    Relation = "can_manage_network_acls"
	RelationCanManageNetworkZones   Relation = "can_manage_network_zones"
	RelationCanManageProfiles       Relation = "can_manage_profiles"
	RelationCanManageStorageVolumes Relation = "can_manage_storage_volumes"
	RelationCanManageStorageBuckets Relation = "can_manage_storage_buckets"
	RelationCanViewOperations       Relation = "can_view_operations"
	RelationCanViewEvents           Relation = "can_view_events"

	// Instance entitlements.
	RelationUser             Relation = "user"
	RelationCanUpdateState   Relation = "can_update_state"
	RelationCanConnectSFTP   Relation = "can_connect_sftp"
	RelationCanAccessFiles   Relation = "can_access_files"
	RelationCanAccessConsole Relation = "can_access_console"
	RelationCanExec          Relation = "can_exec"

	// Instance and storage volume entitlements.
	RelationCanManageSnapshots Relation = "can_manage_snapshots"
	RelationCanManageBackups   Relation = "can_manage_backups"

	// Object to object relations
	RelationServer  Relation = "server"
	RelationProject Relation = "project"

	// User and group relations
	RelationMember Relation = "member"
)

// ObjectType is a type of resource within LXD.
type ObjectType string

const (
	ObjectTypeUser          ObjectType = "user"
	ObjectTypeGroup         ObjectType = "group"
	ObjectTypeServer        ObjectType = "server"
	ObjectTypeCertificate   ObjectType = "certificate"
	ObjectTypeStoragePool   ObjectType = "storage_pool"
	ObjectTypeProject       ObjectType = "project"
	ObjectTypeImage         ObjectType = "image"
	ObjectTypeImageAlias    ObjectType = "image_alias"
	ObjectTypeInstance      ObjectType = "instance"
	ObjectTypeNetwork       ObjectType = "network"
	ObjectTypeNetworkACL    ObjectType = "network_acl"
	ObjectTypeNetworkZone   ObjectType = "network_zone"
	ObjectTypeProfile       ObjectType = "profile"
	ObjectTypeStorageBucket ObjectType = "storage_bucket"
	ObjectTypeStorageVolume ObjectType = "storage_volume"
)

func (o ObjectType) Relations() []Relation {
	switch o {
	case ObjectTypeGroup:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeServer:
		return []Relation{
			RelationAdmin,
			RelationOperator,
			RelationViewer,
			RelationCanEdit,
			RelationCanView,
			RelationCanManagePermissions,
			RelationCanManageStoragePools,
			RelationCanManageProjects,
			RelationCanViewResources,
			RelationCanManageCertificates,
			RelationCanViewMetrics,
			RelationCanOverrideClusterTargetRestriction,
			RelationCanViewPrivilegedEvents,
			RelationCanViewWarnings,
		}
	case ObjectTypeCertificate:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeStoragePool:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeProject:
		return []Relation{
			RelationManager,
			RelationOperator,
			RelationViewer,
			RelationCanManageImages,
			RelationCanManageImageAliases,
			RelationCanManageInstances,
			RelationCanManageNetworks,
			RelationCanManageNetworkACLs,
			RelationCanManageNetworkZones,
			RelationCanManageProfiles,
			RelationCanManageStorageVolumes,
			RelationCanManageStorageBuckets,
			RelationCanViewOperations,
			RelationCanViewEvents,
		}
	case ObjectTypeImage:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeImageAlias:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeInstance:
		return []Relation{
			RelationManager,
			RelationOperator,
			RelationUser,
			RelationViewer,
			RelationCanEdit,
			RelationCanView,
			RelationCanUpdateState,
			RelationCanManageSnapshots,
			RelationCanManageBackups,
			RelationCanConnectSFTP,
			RelationCanAccessFiles,
			RelationCanAccessConsole,
			RelationCanExec,
		}
	case ObjectTypeNetwork:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeNetworkACL:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeNetworkZone:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeProfile:
		return []Relation{RelationCanView, RelationCanEdit}
	case ObjectTypeStorageVolume:
		return []Relation{
			RelationCanView,
			RelationCanEdit,
			RelationCanManageSnapshots,
			RelationCanManageBackups,
		}
	case ObjectTypeStorageBucket:
		return []Relation{RelationCanView, RelationCanEdit}
	}

	return nil
}

func (o ObjectType) ValidateRelation(relation Relation) error {
	relations := o.Relations()
	if !shared.ValueInSlice(relation, relations) {
		return fmt.Errorf("No such relation %q for objects of type %q", relation, o)
	}

	return nil
}
