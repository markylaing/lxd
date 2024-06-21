package main

import (
	"fmt"

	"github.com/canonical/lxd/client"
	"github.com/canonical/lxd/lxd/revert"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
)

type initDataNode struct {
	api.ServerPut `yaml:",inline"`
	Networks      []api.NetworksPost     `json:"networks" yaml:"networks"`
	StoragePools  []api.StoragePoolsPost `json:"storage_pools" yaml:"storage_pools"`
	Profiles      []api.ProfilesPost     `json:"profiles" yaml:"profiles"`
}

type initDataCluster struct {
	api.ClusterPut `yaml:",inline"`

	// The path to the cluster certificate
	// Example: /tmp/cluster.crt
	ClusterCertificatePath string `json:"cluster_certificate_path" yaml:"cluster_certificate_path"`

	// A cluster join token
	// Example: BASE64-TOKEN
	ClusterToken string `json:"cluster_token" yaml:"cluster_token"`
}

// Helper to initialize node-specific entities on a LXD instance using the
// definitions from the given initDataNode object.
//
// It's used both by the 'lxd init' command and by the PUT /1.0/cluster API.
//
// In case of error, the returned function can be used to revert the changes.
func initDataNodeApply(d lxd.InstanceServer, config initDataNode) (func(), error) {
	revert := revert.New()
	defer revert.Fail()

	// Apply server configuration
	if config.Config != nil && len(config.Config) > 0 {
		// Get current config
		currentServer, etag, err := d.GetServer()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve current server configuration: %w", err)
		}

		// Setup reverter
		revert.Add(func() { d.UpdateServer(currentServer.Writable(), "") })

		// Prepare the update
		newServer := api.ServerPut{}
		err = shared.DeepCopy(currentServer.Writable(), &newServer)
		if err != nil {
			return nil, fmt.Errorf("Failed to copy server configuration: %w", err)
		}

		for k, v := range config.Config {
			newServer.Config[k] = fmt.Sprintf("%v", v)
		}

		// Apply it
		err = d.UpdateServer(newServer, etag)
		if err != nil {
			return nil, fmt.Errorf("Failed to update server configuration: %w", err)
		}
	}

	// Apply network configuration
	if config.Networks != nil && len(config.Networks) > 0 {
		// Get the list of networks
		networkNames, err := d.GetNetworkNames()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of networks: %w", err)
		}

		// Network creator
		createNetwork := func(network api.NetworksPost) error {
			// Create the network if doesn't exist
			err := d.CreateNetwork(network)
			if err != nil {
				return fmt.Errorf("Failed to create network '%s': %w", network.Name, err)
			}

			// Setup reverter
			revert.Add(func() { d.DeleteNetwork(network.Name) })
			return nil
		}

		// Network updater
		updateNetwork := func(network api.NetworksPost) error {
			// Get the current network
			currentNetwork, etag, err := d.GetNetwork(network.Name)
			if err != nil {
				return fmt.Errorf("Failed to retrieve current network '%s': %w", network.Name, err)
			}

			// Setup reverter
			revert.Add(func() { d.UpdateNetwork(currentNetwork.Name, currentNetwork.Writable(), "") })

			// Prepare the update
			newNetwork := api.NetworkPut{}
			err = shared.DeepCopy(currentNetwork.Writable(), &newNetwork)
			if err != nil {
				return fmt.Errorf("Failed to copy configuration of network '%s': %w", network.Name, err)
			}

			// Description override
			if network.Description != "" {
				newNetwork.Description = network.Description
			}

			// Config overrides
			for k, v := range network.Config {
				newNetwork.Config[k] = fmt.Sprintf("%v", v)
			}

			// Apply it
			err = d.UpdateNetwork(currentNetwork.Name, newNetwork, etag)
			if err != nil {
				return fmt.Errorf("Failed to update network '%s': %w", network.Name, err)
			}

			return nil
		}

		for _, network := range config.Networks {
			// New network
			if !shared.StringInSlice(network.Name, networkNames) {
				err := createNetwork(network)
				if err != nil {
					return nil, err
				}

				continue
			}

			// Existing network
			err := updateNetwork(network)
			if err != nil {
				return nil, err
			}
		}
	}

	// Apply storage configuration
	if config.StoragePools != nil && len(config.StoragePools) > 0 {
		// Get the list of storagePools
		storagePoolNames, err := d.GetStoragePoolNames()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of storage pools: %w", err)
		}

		// StoragePool creator
		createStoragePool := func(storagePool api.StoragePoolsPost) error {
			// Create the storagePool if doesn't exist
			err := d.CreateStoragePool(storagePool)
			if err != nil {
				return fmt.Errorf("Failed to create storage pool '%s': %w", storagePool.Name, err)
			}

			// Setup reverter
			revert.Add(func() { d.DeleteStoragePool(storagePool.Name) })
			return nil
		}

		// StoragePool updater
		updateStoragePool := func(storagePool api.StoragePoolsPost) error {
			// Get the current storagePool
			currentStoragePool, etag, err := d.GetStoragePool(storagePool.Name)
			if err != nil {
				return fmt.Errorf("Failed to retrieve current storage pool '%s': %w", storagePool.Name, err)
			}

			// Quick check.
			if currentStoragePool.Driver != storagePool.Driver {
				return fmt.Errorf("Storage pool '%s' is of type '%s' instead of '%s'", currentStoragePool.Name, currentStoragePool.Driver, storagePool.Driver)
			}

			// Setup reverter
			revert.Add(func() { d.UpdateStoragePool(currentStoragePool.Name, currentStoragePool.Writable(), "") })

			// Prepare the update
			newStoragePool := api.StoragePoolPut{}
			err = shared.DeepCopy(currentStoragePool.Writable(), &newStoragePool)
			if err != nil {
				return fmt.Errorf("Failed to copy configuration of storage pool '%s': %w", storagePool.Name, err)
			}

			// Description override
			if storagePool.Description != "" {
				newStoragePool.Description = storagePool.Description
			}

			// Config overrides
			for k, v := range storagePool.Config {
				newStoragePool.Config[k] = fmt.Sprintf("%v", v)
			}

			// Apply it
			err = d.UpdateStoragePool(currentStoragePool.Name, newStoragePool, etag)
			if err != nil {
				return fmt.Errorf("Failed to update storage pool '%s': %w", storagePool.Name, err)
			}

			return nil
		}

		for _, storagePool := range config.StoragePools {
			// New storagePool
			if !shared.StringInSlice(storagePool.Name, storagePoolNames) {
				err := createStoragePool(storagePool)
				if err != nil {
					return nil, err
				}

				continue
			}

			// Existing storagePool
			err := updateStoragePool(storagePool)
			if err != nil {
				return nil, err
			}
		}
	}

	// Apply profile configuration
	if config.Profiles != nil && len(config.Profiles) > 0 {
		// Get the list of profiles
		profileNames, err := d.GetProfileNames()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of profiles: %w", err)
		}

		// Profile creator
		createProfile := func(profile api.ProfilesPost) error {
			// Create the profile if doesn't exist
			err := d.CreateProfile(profile)
			if err != nil {
				return fmt.Errorf("Failed to create profile '%s': %w", profile.Name, err)
			}

			// Setup reverter
			revert.Add(func() { d.DeleteProfile(profile.Name) })
			return nil
		}

		// Profile updater
		updateProfile := func(profile api.ProfilesPost) error {
			// Get the current profile
			currentProfile, etag, err := d.GetProfile(profile.Name)
			if err != nil {
				return fmt.Errorf("Failed to retrieve current profile '%s': %w", profile.Name, err)
			}

			// Setup reverter
			revert.Add(func() { d.UpdateProfile(currentProfile.Name, currentProfile.Writable(), "") })

			// Prepare the update
			newProfile := api.ProfilePut{}
			err = shared.DeepCopy(currentProfile.Writable(), &newProfile)
			if err != nil {
				return fmt.Errorf("Failed to copy configuration of profile '%s': %w", profile.Name, err)
			}

			// Description override
			if profile.Description != "" {
				newProfile.Description = profile.Description
			}

			// Config overrides
			for k, v := range profile.Config {
				newProfile.Config[k] = fmt.Sprintf("%v", v)
			}

			// Device overrides
			for k, v := range profile.Devices {
				// New device
				_, ok := newProfile.Devices[k]
				if !ok {
					newProfile.Devices[k] = v
					continue
				}

				// Existing device
				for configKey, configValue := range v {
					newProfile.Devices[k][configKey] = fmt.Sprintf("%v", configValue)
				}
			}

			// Apply it
			err = d.UpdateProfile(currentProfile.Name, newProfile, etag)
			if err != nil {
				return fmt.Errorf("Failed to update profile '%s': %w", profile.Name, err)
			}

			return nil
		}

		for _, profile := range config.Profiles {
			// New profile
			if !shared.StringInSlice(profile.Name, profileNames) {
				err := createProfile(profile)
				if err != nil {
					return nil, err
				}

				continue
			}

			// Existing profile
			err := updateProfile(profile)
			if err != nil {
				return nil, err
			}
		}
	}

	revertExternal := revert.Clone() // Clone before calling revert.Success() so we can return the Fail func.
	revert.Success()
	return revertExternal.Fail, nil
}

// Helper to initialize LXD clustering.
//
// Used by the 'lxd init' command.
func initDataClusterApply(d lxd.InstanceServer, config *initDataCluster) error {
	if config == nil || !config.Enabled {
		return nil
	}

	// Get the current cluster configuration
	currentCluster, etag, err := d.GetCluster()
	if err != nil {
		return fmt.Errorf("Failed to retrieve current cluster config: %w", err)
	}

	// Check if already enabled
	if !currentCluster.Enabled {
		// Configure the cluster
		op, err := d.UpdateCluster(config.ClusterPut, etag)
		if err != nil {
			return fmt.Errorf("Failed to configure cluster: %w", err)
		}

		err = op.Wait()
		if err != nil {
			return fmt.Errorf("Failed to configure cluster: %w", err)
		}
	}

	return nil
}
