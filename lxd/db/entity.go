//go:build linux && cgo && !agent

package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/canonical/lxd/lxd/entity"

	"github.com/canonical/lxd/lxd/db/cluster"
)

// ErrUnknownEntityID describes the unknown entity ID error.
var ErrUnknownEntityID = fmt.Errorf("Unknown entity ID")

// GetURIFromEntity returns the URI for the given entity type and entity ID.
func (c *Cluster) GetURIFromEntity(entityType entity.Type, entityID int) (string, error) {
	if entityID == -1 || entityType == -1 {
		return "", nil
	}

	_, ok := entity.Names[entityType]
	if !ok {
		return "", fmt.Errorf("Unknown entity type")
	}

	var err error
	var uri string

	switch entityType {
	case entity.TypeImage:
		var images []cluster.Image

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			images, err = cluster.GetImages(ctx, tx.tx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get images: %w", err)
		}

		for _, image := range images {
			if image.ID != entityID {
				continue
			}

			uri = fmt.Sprintf(entity.UrIs[entityType], image.Fingerprint, image.Project)
			break
		}

		if uri == "" {
			return "", ErrUnknownEntityID
		}

	case entity.TypeProfile:
		var profiles []cluster.Profile

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			profiles, err = cluster.GetProfiles(ctx, tx.Tx())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get profiles: %w", err)
		}

		for _, profile := range profiles {
			if profile.ID != entityID {
				continue
			}

			uri = fmt.Sprintf(entity.UrIs[entityType], profile.Name, profile.Project)
			break
		}

		if uri == "" {
			return "", ErrUnknownEntityID
		}

	case entity.TypeProject:
		projects := make(map[int64]string)

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			projects, err = cluster.GetProjectIDsToNames(context.Background(), tx.tx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get project names and IDs: %w", err)
		}

		name, ok := projects[int64(entityID)]
		if !ok {
			return "", ErrUnknownEntityID
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], name)
	case entity.TypeCertificate:
		var certificates []cluster.Certificate

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			certificates, err = cluster.GetCertificates(context.Background(), tx.tx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get certificates: %w", err)
		}

		for _, cert := range certificates {
			if cert.ID != entityID {
				continue
			}

			uri = fmt.Sprintf(entity.UrIs[entityType], cert.Name)
			break
		}

		if uri == "" {
			return "", ErrUnknownEntityID
		}

	case entity.TypeContainer:
		fallthrough
	case entity.TypeInstance:
		var instances []cluster.Instance

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			instances, err = cluster.GetInstances(ctx, tx.tx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get instances: %w", err)
		}

		for _, instance := range instances {
			if instance.ID != entityID {
				continue
			}

			uri = fmt.Sprintf(entity.UrIs[entityType], instance.Name, instance.Project)
			break
		}

		if uri == "" {
			return "", ErrUnknownEntityID
		}

	case entity.TypeInstanceBackup:
		instanceBackup, err := c.GetInstanceBackupWithID(entityID)
		if err != nil {
			return "", fmt.Errorf("Failed to get instance backup: %w", err)
		}

		var instances []cluster.Instance

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			instances, err = cluster.GetInstances(ctx, tx.tx)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get instances: %w", err)
		}

		for _, instance := range instances {
			if instance.ID != instanceBackup.InstanceID {
				continue
			}

			uri = fmt.Sprintf(entity.UrIs[entityType], instance.Name, instanceBackup.Name, instance.Project)
			break
		}

		if uri == "" {
			return "", ErrUnknownEntityID
		}

	case entity.TypeInstanceSnapshot:
		var snapshots []cluster.InstanceSnapshot

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			snapshots, err = cluster.GetInstanceSnapshots(ctx, tx.Tx())
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get instance snapshots: %w", err)
		}

		for _, snapshot := range snapshots {
			if snapshot.ID != entityID {
				continue
			}

			uri = fmt.Sprintf(entity.UrIs[entityType], snapshot.Name, snapshot.Project)
			break
		}

		if uri == "" {
			return "", ErrUnknownEntityID
		}

	case entity.TypeNetwork:
		networkName, projectName, err := c.GetNetworkNameAndProjectWithID(entityID)
		if err != nil {
			return "", fmt.Errorf("Failed to get network name and project name: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], networkName, projectName)
	case entity.TypeNetworkACL:
		networkACLName, projectName, err := c.GetNetworkACLNameAndProjectWithID(entityID)
		if err != nil {
			return "", fmt.Errorf("Failed to get network ACL name and project name: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], networkACLName, projectName)
	case entity.TypeNode:
		var nodeInfo NodeInfo

		err := c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			nodeInfo, err = tx.GetNodeWithID(ctx, entityID)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get node information: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], nodeInfo.Name)
	case entity.TypeOperation:
		var op cluster.Operation

		err = c.Transaction(context.TODO(), func(ctx context.Context, tx *ClusterTx) error {
			id := int64(entityID)
			filter := cluster.OperationFilter{ID: &id}
			ops, err := cluster.GetOperations(ctx, tx.tx, filter)
			if err != nil {
				return err
			}

			if len(ops) > 1 {
				return fmt.Errorf("More than one operation matches")
			}

			op = ops[0]
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get operation: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], op.UUID)
	case entity.TypeStoragePool:
		_, pool, _, err := c.GetStoragePoolWithID(entityID)
		if err != nil {
			return "", fmt.Errorf("Failed to get storage pool: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], pool.Name)
	case entity.TypeStorageVolume:
		var args StorageVolumeArgs

		err := c.Transaction(c.closingCtx, func(ctx context.Context, tx *ClusterTx) error {
			args, err = tx.GetStoragePoolVolumeWithID(ctx, entityID)
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get storage volume: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], args.PoolName, args.TypeName, args.Name, args.ProjectName)
	case entity.TypeStorageVolumeBackup:
		backup, err := c.GetStoragePoolVolumeBackupWithID(entityID)
		if err != nil {
			return "", fmt.Errorf("Failed to get volume backup: %w", err)
		}

		var volume StorageVolumeArgs

		err = c.Transaction(c.closingCtx, func(ctx context.Context, tx *ClusterTx) error {
			volume, err = tx.GetStoragePoolVolumeWithID(ctx, int(backup.VolumeID))
			if err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return "", fmt.Errorf("Failed to get storage volume: %w", err)
		}

		uri = fmt.Sprintf(entity.UrIs[entityType], volume.PoolName, volume.TypeName, volume.Name, backup.Name, volume.ProjectName)
	case entity.TypeStorageVolumeSnapshot:
		snapshot, err := c.GetStorageVolumeSnapshotWithID(entityID)
		if err != nil {
			return "", fmt.Errorf("Failed to get volume snapshot: %w", err)
		}

		fields := strings.Split(snapshot.Name, "/")

		uri = fmt.Sprintf(entity.UrIs[entityType], snapshot.PoolName, snapshot, snapshot.TypeName, fields[0], fields[1], snapshot.ProjectName)
	}

	return uri, nil
}
