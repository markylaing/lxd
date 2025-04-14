package entity

import (
	"fmt"
	"strings"

	"github.com/canonical/lxd/shared/api"
)

// Type represents a resource type in LXD that is addressable via the API.
type Type string

// typeInfo represents common attributes an entity type must have.
//
// To create a new entity type, add a new const Type, then create a type that implements typeInfo and add it to the
// entityTypes map.
type typeInfo interface {
	// requiresProject returns whether the Type requires a project to be uniquely specified, e.g. true if it is project
	// specific, false if not.
	requiresProject() bool

	// path returns the API path for the resource. Where path arguments are expected, this should be replaced with `{property_name}`.
	// For example, in a storage volume URL, the storage pool path parameter is represented as `{pool}`.
	path() []string

	// propertyOverrides returns a list of properties for the entity type.
	// This is used to override or append to the properties returned by Type.Properties, which attempts to generalise things
	// based on other entity information, but isn't always 100% correct (e.g. operation doesn't require a project, so this isn't
	// added as a property by default, so we need to add it by implementing this method for operation).
	propertyOverrides() []api.MetadataConfigurationEntityProperties
}

const (
	// TypeContainer represents container resources.
	TypeContainer Type = "container"

	// TypeImage represents image resources.
	TypeImage Type = "image"

	// TypeProfile represents profile resources.
	TypeProfile Type = "profile"

	// TypeProject represents project resources.
	TypeProject Type = "project"

	// TypeCertificate represents certificate resources.
	TypeCertificate Type = "certificate"

	// TypeInstance represents instance resources.
	TypeInstance Type = "instance"

	// TypeInstanceBackup represents instance backup resources.
	TypeInstanceBackup Type = "instance_backup"

	// TypeInstanceSnapshot represents instance snapshot resources.
	TypeInstanceSnapshot Type = "instance_snapshot"

	// TypeNetwork represents network resources.
	TypeNetwork Type = "network"

	// TypeNetworkACL represents network acl resources.
	TypeNetworkACL Type = "network_acl"

	// TypeClusterMember represents node resources.
	TypeClusterMember Type = "cluster_member"

	// TypeOperation represents operation resources.
	TypeOperation Type = "operation"

	// TypeStoragePool represents storage pool resources.
	TypeStoragePool Type = "storage_pool"

	// TypeStorageVolume represents storage volume resources.
	TypeStorageVolume Type = "storage_volume"

	// TypeStorageVolumeBackup represents storage volume backup resources.
	TypeStorageVolumeBackup Type = "storage_volume_backup"

	// TypeStorageVolumeSnapshot represents storage volume snapshot resources.
	TypeStorageVolumeSnapshot Type = "storage_volume_snapshot"

	// TypeWarning represents warning resources.
	TypeWarning Type = "warning"

	// TypeClusterGroup represents cluster group resources.
	TypeClusterGroup Type = "cluster_group"

	// TypeStorageBucket represents storage bucket resources.
	TypeStorageBucket Type = "storage_bucket"

	// TypeServer represents the top level /1.0 resource.
	TypeServer Type = "server"

	// TypeImageAlias represents image alias resources.
	TypeImageAlias Type = "image_alias"

	// TypeNetworkZone represents network zone resources.
	TypeNetworkZone Type = "network_zone"

	// TypeIdentity represents identity resources.
	TypeIdentity Type = "identity"

	// TypeAuthGroup represents authorization group resources.
	TypeAuthGroup Type = "group"

	// TypeIdentityProviderGroup represents identity provider group resources.
	TypeIdentityProviderGroup Type = "identity_provider_group"
)

// String implements fmt.Stringer for Type.
func (t Type) String() string {
	return string(t)
}

// Validate returns an error if the Type is not in the list of allowed types. If the allowEmpty argument is set to true
// an empty string is allowed. This is to accommodate that warnings may not refer to a specific entity type.
func (t Type) Validate() error {
	_, ok := entityTypes[t]
	if !ok {
		return fmt.Errorf("Unknown entity type %q", t)
	}

	return nil
}

// RequiresProject returns true if an entity of the Type can only exist within the context of a project. Operations and
// warnings may still be project specific but it is not an absolute requirement.
func (t Type) RequiresProject() (bool, error) {
	err := t.Validate()
	if err != nil {
		return false, err
	}

	return entityTypes[t].requiresProject(), nil
}

// PathTemplate returns a template of the URL path to an entity of this type, not including query parameters.
func (t Type) PathTemplate() (string, error) {
	err := t.Validate()
	if err != nil {
		return "", err
	}

	return strings.Join(append([]string{"/1.0"}, entityTypes[t].path()...), "/"), nil
}

// Properties returns a list of properties of the entity type, to be reported as entity metadata.
func (t Type) Properties() ([]api.MetadataConfigurationEntityProperties, error) {
	err := t.Validate()
	if err != nil {
		return nil, err
	}

	info := entityTypes[t]
	propertyMap := make(map[string]api.MetadataConfigurationEntityProperties)
	for _, e := range info.path() {
		if e[0] != '{' {
			continue
		}

		name := strings.Trim(e, `{}`)
		propertyMap[name] = api.MetadataConfigurationEntityProperties{
			Name:          name,
			InURLPath:     true,
			RequiredInURL: true,
			URLName:       name,
		}
	}

	if info.requiresProject() {
		propertyMap["project"] = api.MetadataConfigurationEntityProperties{
			Name:          "project",
			InURLPath:     false,
			InURLQuery:    true,
			RequiredInURL: true,
			URLName:       "project",
		}
	}

	for _, override := range info.propertyOverrides() {
		propertyMap[override.Name] = override
	}

	properties := make([]api.MetadataConfigurationEntityProperties, 0, len(propertyMap))
	for _, property := range propertyMap {
		properties = append(properties, property)
	}

	return properties, nil
}

// entityTypes is the source of truth for available entity types in LXD. This should never be modified at runtime.
var entityTypes = map[Type]typeInfo{
	TypeContainer:             container{},
	TypeImage:                 image{},
	TypeProfile:               profile{},
	TypeProject:               project{},
	TypeCertificate:           certificate{},
	TypeInstance:              instance{},
	TypeInstanceBackup:        instanceBackup{},
	TypeInstanceSnapshot:      instanceSnapshot{},
	TypeNetwork:               network{},
	TypeNetworkACL:            networkACL{},
	TypeClusterMember:         clusterMember{},
	TypeOperation:             operation{},
	TypeStoragePool:           storagePool{},
	TypeStorageVolume:         storageVolume{},
	TypeStorageVolumeBackup:   storageVolumeBackup{},
	TypeStorageVolumeSnapshot: storageVolumeSnapshot{},
	TypeWarning:               warning{},
	TypeClusterGroup:          clusterGroup{},
	TypeStorageBucket:         storageBucket{},
	TypeServer:                server{},
	TypeImageAlias:            imageAlias{},
	TypeNetworkZone:           networkZone{},
	TypeIdentity:              identity{},
	TypeAuthGroup:             authGroup{},
	TypeIdentityProviderGroup: identityProviderGroup{},
}

func init() {
	allEntityTypes = make([]Type, 0, len(entityTypes))
	for t := range entityTypes {
		allEntityTypes = append(allEntityTypes, t)
	}
}

var allEntityTypes []Type

func AllEntityTypes() []Type {
	return allEntityTypes
}

// metricsEntityTypes is the source of truth for which entity types can be used to categorize endpoints
// for the API metrics.
var metricsEntityTypes = []Type{
	TypeImage,
	TypeProfile,
	TypeProject,
	TypeCertificate,
	TypeInstance,
	TypeNetwork,
	TypeClusterMember,
	TypeOperation,
	TypeStoragePool,
	TypeWarning,
	TypeServer,
	TypeIdentity,
}

// APIMetricsEntityTypes returns the list of entity types relevant for the API metrics.
func APIMetricsEntityTypes() []Type {
	return metricsEntityTypes
}

type typeInfoCommon struct{}

func (typeInfoCommon) propertyOverrides() []api.MetadataConfigurationEntityProperties {
	return []api.MetadataConfigurationEntityProperties{}
}

type container struct {
	typeInfoCommon
}

func (container) requiresProject() bool {
	return true
}

func (container) path() []string {
	return []string{"containers", "{name}"}
}

type image struct {
	typeInfoCommon
}

func (image) requiresProject() bool {
	return true
}

func (image) path() []string {
	return []string{"images", "{fingerprint}"}
}

type profile struct {
	typeInfoCommon
}

func (profile) requiresProject() bool {
	return true
}

func (profile) path() []string {
	return []string{"profiles", "{name}"}
}

type project struct {
	typeInfoCommon
}

func (project) requiresProject() bool {
	return false
}

func (project) path() []string {
	return []string{"projects", "{name}"}
}

type certificate struct {
	typeInfoCommon
}

func (certificate) requiresProject() bool {
	return false
}

func (certificate) path() []string {
	return []string{"certificates", "{fingerprint}"}
}

type instance struct {
	typeInfoCommon
}

func (instance) requiresProject() bool {
	return true
}

func (instance) path() []string {
	return []string{"instances", "{name}"}
}

type instanceBackup struct {
	typeInfoCommon
}

func (instanceBackup) requiresProject() bool {
	return true
}

func (instanceBackup) path() []string {
	return []string{"instances", "{instance_name}", "backups", "{name}"}
}

type instanceSnapshot struct {
	typeInfoCommon
}

func (instanceSnapshot) requiresProject() bool {
	return true
}

func (instanceSnapshot) path() []string {
	return []string{"instances", "{instance_name}", "snapshots", "{name}"}
}

type network struct {
	typeInfoCommon
}

func (network) requiresProject() bool {
	return true
}

func (network) path() []string {
	return []string{"networks", "{name}"}
}

type networkACL struct {
	typeInfoCommon
}

func (networkACL) requiresProject() bool {
	return true
}

func (networkACL) path() []string {
	return []string{"network-acls", "{name}"}
}

type clusterMember struct {
	typeInfoCommon
}

func (clusterMember) requiresProject() bool {
	return false
}

func (clusterMember) path() []string {
	return []string{"cluster", "members", "{name}"}
}

type operation struct {
	typeInfoCommon
}

func (operation) requiresProject() bool {
	return false
}

func (operation) path() []string {
	return []string{"operations", "{id}"}
}

func (operation) propertyOverrides() []api.MetadataConfigurationEntityProperties {
	return []api.MetadataConfigurationEntityProperties{
		// Operations don't require a project but can have one.
		{
			Name:       "project",
			InURLQuery: true,
			URLName:    "project",
		},
	}
}

type storagePool struct {
	typeInfoCommon
}

func (storagePool) requiresProject() bool {
	return false
}

func (storagePool) path() []string {
	return []string{"storage-pools", "{name}"}
}

type storageVolume struct {
	typeInfoCommon
}

func (storageVolume) requiresProject() bool {
	return true
}

func (storageVolume) path() []string {
	return []string{"storage-pools", "{pool}", "volumes", "{type}", "{name}"}
}

func (storageVolume) propertyOverrides() []api.MetadataConfigurationEntityProperties {
	return []api.MetadataConfigurationEntityProperties{
		// Local storage volumes may require a target query parameter to uniquely reference a single volume.
		// Note that the property name is different to it's URL representation.
		{
			Name:       "location",
			InURLQuery: true,
			URLName:    "target",
		},
	}
}

type storageVolumeBackup struct {
	typeInfoCommon
}

func (storageVolumeBackup) requiresProject() bool {
	return true
}

func (storageVolumeBackup) path() []string {
	return []string{"storage-pools", "{pool}", "volumes", "{type}", "{volume_name}", "backups", "{name}"}
}

func (storageVolumeBackup) propertyOverrides() []api.MetadataConfigurationEntityProperties {
	return []api.MetadataConfigurationEntityProperties{
		// Local storage volume backups may require a target query parameter to uniquely reference a single volume.
		// Note that the property name is different to it's URL representation.
		{
			Name:       "location",
			InURLQuery: true,
			URLName:    "target",
		},
	}
}

type storageVolumeSnapshot struct {
	typeInfoCommon
}

func (storageVolumeSnapshot) requiresProject() bool {
	return true
}

func (storageVolumeSnapshot) path() []string {
	return []string{"storage-pools", "{pool}", "volumes", "{type}", "{volume_name}", "snapshots", "{name}"}
}

func (storageVolumeSnapshot) propertyOverrides() []api.MetadataConfigurationEntityProperties {
	return []api.MetadataConfigurationEntityProperties{
		// Local storage volume snapshots may require a target query parameter to uniquely reference a single volume.
		// Note that the property name is different to it's URL representation.
		{
			Name:       "location",
			InURLQuery: true,
			URLName:    "target",
		},
	}
}

type warning struct {
	typeInfoCommon
}

func (warning) requiresProject() bool {
	return false
}

func (warning) path() []string {
	return []string{"warnings", "{id}"}
}

func (warning) propertyOverrides() []api.MetadataConfigurationEntityProperties {
	return []api.MetadataConfigurationEntityProperties{
		// Warnings don't require a project but can have one.
		{
			Name:       "project",
			InURLQuery: true,
			URLName:    "project",
		},
	}
}

type clusterGroup struct {
	typeInfoCommon
}

func (clusterGroup) requiresProject() bool {
	return false
}

func (clusterGroup) path() []string {
	return []string{"cluster", "groups", "{name}"}
}

type storageBucket struct {
	typeInfoCommon
}

func (storageBucket) requiresProject() bool {
	return true
}

func (storageBucket) path() []string {
	return []string{"storage-pools", "{pool}", "buckets", "{name}"}
}

type server struct {
	typeInfoCommon
}

func (server) requiresProject() bool {
	return false
}

func (server) path() []string {
	return []string{}
}

type imageAlias struct {
	typeInfoCommon
}

func (imageAlias) requiresProject() bool {
	return true
}

func (imageAlias) path() []string {
	return []string{"images", "aliases", "{name}"}
}

type networkZone struct {
	typeInfoCommon
}

func (networkZone) requiresProject() bool {
	return true
}

func (networkZone) path() []string {
	return []string{"network-zones", "{name}"}
}

type identity struct {
	typeInfoCommon
}

func (identity) requiresProject() bool {
	return false
}

func (identity) path() []string {
	return []string{"auth", "identities", "{auth_method}", "{identifier}"}
}

type authGroup struct {
	typeInfoCommon
}

func (authGroup) requiresProject() bool {
	return false
}

func (authGroup) path() []string {
	return []string{"auth", "groups", "{name}"}
}

type identityProviderGroup struct {
	typeInfoCommon
}

func (identityProviderGroup) requiresProject() bool {
	return false
}

func (identityProviderGroup) path() []string {
	return []string{"auth", "identity-provider-groups", "{name}"}
}
