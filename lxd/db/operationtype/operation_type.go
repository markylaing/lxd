package operationtype

import (
	"github.com/canonical/lxd/shared/entitlement"
)

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
func (t Type) Permission() (entitlement.ObjectType, entitlement.Relation) {
	switch t {
	case BackupCreate:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageBackups
	case BackupRename:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageBackups
	case BackupRestore:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageBackups
	case BackupRemove:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageBackups
	case ConsoleShow:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanAccessConsole
	case InstanceFreeze:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanUpdateState
	case InstanceUnfreeze:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanUpdateState
	case InstanceStart:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanUpdateState
	case InstanceStop:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanUpdateState
	case InstanceRestart:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanUpdateState
	case CommandExec:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanExec
	case SnapshotCreate:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageSnapshots
	case SnapshotRename:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageSnapshots
	case SnapshotTransfer:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageSnapshots
	case SnapshotUpdate:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageSnapshots
	case SnapshotDelete:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanManageSnapshots

	case InstanceCreate:
		return entitlement.ObjectTypeProject, entitlement.RelationCanManageInstances
	case InstanceUpdate:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit
	case InstanceRename:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit
	case InstanceMigrate:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit
	case InstanceLiveMigrate:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit
	case InstanceDelete:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit
	case InstanceRebuild:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit
	case SnapshotRestore:
		return entitlement.ObjectTypeInstance, entitlement.RelationCanEdit

	case ImageDownload:
		return entitlement.ObjectTypeImage, entitlement.RelationCanEdit
	case ImageDelete:
		return entitlement.ObjectTypeImage, entitlement.RelationCanEdit
	case ImageToken:
		return entitlement.ObjectTypeImage, entitlement.RelationCanEdit
	case ImageRefresh:
		return entitlement.ObjectTypeImage, entitlement.RelationCanEdit
	case ImagesUpdate:
		return entitlement.ObjectTypeImage, entitlement.RelationCanEdit
	case ImagesSynchronize:
		return entitlement.ObjectTypeImage, entitlement.RelationCanEdit

	case CustomVolumeSnapshotsExpire:
		return entitlement.ObjectTypeStorageVolume, entitlement.RelationCanEdit
	case CustomVolumeBackupCreate:
		return entitlement.ObjectTypeStorageVolume, entitlement.RelationCanManageBackups
	case CustomVolumeBackupRemove:
		return entitlement.ObjectTypeStorageVolume, entitlement.RelationCanManageBackups
	case CustomVolumeBackupRename:
		return entitlement.ObjectTypeStorageVolume, entitlement.RelationCanManageBackups
	case CustomVolumeBackupRestore:
		return entitlement.ObjectTypeStorageVolume, entitlement.RelationCanEdit
	}

	return "", ""
}
