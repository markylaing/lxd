package operationtype

import "github.com/canonical/lxd/lxd/entity"

// Type is a numeric code indentifying the type of an Operation.
type Type int64

// Possible values for Type
//
// WARNING: The type codes are stored in the database, so this list of
// definitions should be normally append-only. Any other change
// requires a database update.
const (
	Unknown Type = iota
	ClusterBootstrap
	ClusterJoin
	BackupCreate
	BackupRename
	BackupRestore
	BackupRemove
	ConsoleShow
	InstanceCreate
	InstanceUpdate
	InstanceRename
	InstanceMigrate
	InstanceLiveMigrate
	InstanceFreeze
	InstanceUnfreeze
	InstanceDelete
	InstanceStart
	InstanceStop
	InstanceRestart
	InstanceRebuild
	CommandExec
	SnapshotCreate
	SnapshotRename
	SnapshotRestore
	SnapshotTransfer
	SnapshotUpdate
	SnapshotDelete
	ImageDownload
	ImageDelete
	ImageToken
	ImageRefresh
	VolumeCopy
	VolumeCreate
	VolumeMigrate
	VolumeMove
	VolumeSnapshotCreate
	VolumeSnapshotDelete
	VolumeSnapshotUpdate
	ProjectRename
	ImagesExpire
	ImagesPruneLeftover
	ImagesUpdate
	ImagesSynchronize
	LogsExpire
	InstanceTypesUpdate
	BackupsExpire
	SnapshotsExpire
	CustomVolumeSnapshotsExpire
	CustomVolumeBackupCreate
	CustomVolumeBackupRemove
	CustomVolumeBackupRename
	CustomVolumeBackupRestore
	WarningsPruneResolved
	ClusterJoinToken
	VolumeSnapshotRename
	ClusterMemberEvacuate
	ClusterMemberRestore
	CertificateAddToken
	RemoveOrphanedOperations
	RenewServerCertificate
	RemoveExpiredTokens
	ClusterHeal
)

// Description return a human-readable description of the operation type.
func (t Type) Description() string {
	switch t {
	case ClusterBootstrap:
		return "Creating bootstrap node"
	case ClusterJoin:
		return "Joining cluster"
	case BackupCreate:
		return "Backing up instance"
	case BackupRename:
		return "Renaming instance backup"
	case BackupRestore:
		return "Restoring backup"
	case BackupRemove:
		return "Removing instance backup"
	case ConsoleShow:
		return "Showing console"
	case InstanceCreate:
		return "Creating instance"
	case InstanceUpdate:
		return "Updating instance"
	case InstanceRename:
		return "Renaming instance"
	case InstanceMigrate:
		return "Migrating instance"
	case InstanceLiveMigrate:
		return "Live-migrating instance"
	case InstanceFreeze:
		return "Freezing instance"
	case InstanceUnfreeze:
		return "Unfreezing instance"
	case InstanceDelete:
		return "Deleting instance"
	case InstanceStart:
		return "Starting instance"
	case InstanceStop:
		return "Stopping instance"
	case InstanceRestart:
		return "Restarting instance"
	case InstanceRebuild:
		return "Rebuilding instance"
	case CommandExec:
		return "Executing command"
	case SnapshotCreate:
		return "Snapshotting instance"
	case SnapshotRename:
		return "Renaming snapshot"
	case SnapshotRestore:
		return "Restoring snapshot"
	case SnapshotTransfer:
		return "Transferring snapshot"
	case SnapshotUpdate:
		return "Updating snapshot"
	case SnapshotDelete:
		return "Deleting snapshot"
	case ImageDownload:
		return "Downloading image"
	case ImageDelete:
		return "Deleting image"
	case ImageToken:
		return "Image download token"
	case ImageRefresh:
		return "Refreshing image"
	case VolumeCopy:
		return "Copying storage volume"
	case VolumeCreate:
		return "Creating storage volume"
	case VolumeMigrate:
		return "Migrating storage volume"
	case VolumeMove:
		return "Moving storage volume"
	case VolumeSnapshotCreate:
		return "Creating storage volume snapshot"
	case VolumeSnapshotDelete:
		return "Deleting storage volume snapshot"
	case VolumeSnapshotUpdate:
		return "Updating storage volume snapshot"
	case VolumeSnapshotRename:
		return "Renaming storage volume snapshot"
	case ProjectRename:
		return "Renaming project"
	case ImagesExpire:
		return "Cleaning up expired images"
	case ImagesPruneLeftover:
		return "Pruning leftover image files"
	case ImagesUpdate:
		return "Updating images"
	case ImagesSynchronize:
		return "Synchronizing images"
	case LogsExpire:
		return "Expiring log files"
	case InstanceTypesUpdate:
		return "Updating instance types"
	case BackupsExpire:
		return "Cleaning up expired backups"
	case SnapshotsExpire:
		return "Cleaning up expired instance snapshots"
	case CustomVolumeSnapshotsExpire:
		return "Cleaning up expired volume snapshots"
	case CustomVolumeBackupCreate:
		return "Creating custom volume backup"
	case CustomVolumeBackupRemove:
		return "Deleting custom volume backup"
	case CustomVolumeBackupRename:
		return "Renaming custom volume backup"
	case CustomVolumeBackupRestore:
		return "Restoring custom volume backup"
	case WarningsPruneResolved:
		return "Pruning resolved warnings"
	case ClusterMemberEvacuate:
		return "Evacuating cluster member"
	case ClusterMemberRestore:
		return "Restoring cluster member"
	case RemoveOrphanedOperations:
		return "Remove orphaned operations"
	case RenewServerCertificate:
		return "Renewing server certificate"
	case RemoveExpiredTokens:
		return "Remove expired tokens"
	case ClusterHeal:
		return "Healing cluster"
	default:
		return "Executing operation"
	}
}

// Permission returns the auth.ObjectType and auth.Relation required to cancel the operation.
func (t Type) Permission() (entity.Type, entity.Entitlement) {
	switch t {
	case BackupCreate:
		return entity.TypeInstance, entity.EntitlementCanManageBackups
	case BackupRename:
		return entity.TypeInstance, entity.EntitlementCanManageBackups
	case BackupRestore:
		return entity.TypeInstance, entity.EntitlementCanManageBackups
	case BackupRemove:
		return entity.TypeInstance, entity.EntitlementCanManageBackups
	case ConsoleShow:
		return entity.TypeInstance, entity.EntitlementCanAccessConsole
	case InstanceFreeze:
		return entity.TypeInstance, entity.EntitlementCanUpdateState
	case InstanceUnfreeze:
		return entity.TypeInstance, entity.EntitlementCanUpdateState
	case InstanceStart:
		return entity.TypeInstance, entity.EntitlementCanUpdateState
	case InstanceStop:
		return entity.TypeInstance, entity.EntitlementCanUpdateState
	case InstanceRestart:
		return entity.TypeInstance, entity.EntitlementCanUpdateState
	case CommandExec:
		return entity.TypeInstance, entity.EntitlementCanExec
	case SnapshotCreate:
		return entity.TypeInstance, entity.EntitlementCanManageSnapshots
	case SnapshotRename:
		return entity.TypeInstance, entity.EntitlementCanManageSnapshots
	case SnapshotTransfer:
		return entity.TypeInstance, entity.EntitlementCanManageSnapshots
	case SnapshotUpdate:
		return entity.TypeInstance, entity.EntitlementCanManageSnapshots
	case SnapshotDelete:
		return entity.TypeInstance, entity.EntitlementCanManageSnapshots

	case InstanceCreate:
		return entity.TypeProject, entity.EntitlementCanManageInstances
	case InstanceUpdate:
		return entity.TypeInstance, entity.EntitlementCanEdit
	case InstanceRename:
		return entity.TypeInstance, entity.EntitlementCanEdit
	case InstanceMigrate:
		return entity.TypeInstance, entity.EntitlementCanEdit
	case InstanceLiveMigrate:
		return entity.TypeInstance, entity.EntitlementCanEdit
	case InstanceDelete:
		return entity.TypeInstance, entity.EntitlementCanEdit
	case InstanceRebuild:
		return entity.TypeInstance, entity.EntitlementCanEdit
	case SnapshotRestore:
		return entity.TypeInstance, entity.EntitlementCanEdit

	case ImageDownload:
		return entity.TypeImage, entity.EntitlementCanEdit
	case ImageDelete:
		return entity.TypeImage, entity.EntitlementCanEdit
	case ImageToken:
		return entity.TypeImage, entity.EntitlementCanEdit
	case ImageRefresh:
		return entity.TypeImage, entity.EntitlementCanEdit
	case ImagesUpdate:
		return entity.TypeImage, entity.EntitlementCanEdit
	case ImagesSynchronize:
		return entity.TypeImage, entity.EntitlementCanEdit

	case CustomVolumeSnapshotsExpire:
		return entity.TypeStorageVolume, entity.EntitlementCanEdit
	case CustomVolumeBackupCreate:
		return entity.TypeStorageVolume, entity.EntitlementCanManageBackups
	case CustomVolumeBackupRemove:
		return entity.TypeStorageVolume, entity.EntitlementCanManageBackups
	case CustomVolumeBackupRename:
		return entity.TypeStorageVolume, entity.EntitlementCanManageBackups
	case CustomVolumeBackupRestore:
		return entity.TypeStorageVolume, entity.EntitlementCanEdit
	}

	return -1, ""
}
