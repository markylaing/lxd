package entity

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/canonical/lxd/shared"

	"github.com/canonical/lxd/shared/version"
)

type Type int

// Numeric type codes identifying different kind of entities.
const (
	TypeContainer             Type = 0
	TypeImage                 Type = 1
	TypeProfile               Type = 2
	TypeProject               Type = 3
	TypeCertificate           Type = 4
	TypeInstance              Type = 5
	TypeInstanceBackup        Type = 6
	TypeInstanceSnapshot      Type = 7
	TypeNetwork               Type = 8
	TypeNetworkACL            Type = 9
	TypeNode                  Type = 10
	TypeOperation             Type = 11
	TypeStoragePool           Type = 12
	TypeStorageVolume         Type = 13
	TypeStorageVolumeBackup   Type = 14
	TypeStorageVolumeSnapshot Type = 15
	TypeWarning               Type = 16
	TypeClusterGroup          Type = 17
	TypeStorageBucket         Type = 18
	TypeServer                Type = 22
	TypeAuthUser              Type = 23
	TypeAuthGroup             Type = 24
	TypeImageAlias            Type = 25
	TypeNetworkZone           Type = 26
)

// Names associates an entity code to its name.
var Names = map[Type]string{
	TypeContainer:             "container",
	TypeImage:                 "image",
	TypeProfile:               "profile",
	TypeProject:               "project",
	TypeCertificate:           "certificate",
	TypeInstance:              "instance",
	TypeInstanceBackup:        "instance_backup",
	TypeInstanceSnapshot:      "instance_snapshot",
	TypeNetwork:               "network",
	TypeNetworkACL:            "network_acl",
	TypeNode:                  "node",
	TypeOperation:             "operation",
	TypeStoragePool:           "storage_pool",
	TypeStorageVolume:         "storage_volume",
	TypeStorageVolumeBackup:   "storage_volume_backup",
	TypeStorageVolumeSnapshot: "storage_volume_snapshot",
	TypeStorageBucket:         "storage_bucket",
	TypeWarning:               "warning",
	TypeClusterGroup:          "cluster_group",
	TypeAuthUser:              "user",
	TypeServer:                "server",
	TypeAuthGroup:             "group",
	TypeImageAlias:            "image_alias",
	TypeNetworkZone:           "network_zone",
}

// Types associates an entity name to its type code.
var Types = map[string]Type{}

// UrIs associates an entity code to its URI pattern.
var UrIs = map[Type]string{
	TypeContainer:             "/" + version.APIVersion + "/containers/%s",
	TypeImage:                 "/" + version.APIVersion + "/images/%s",
	TypeProfile:               "/" + version.APIVersion + "/profiles/%s",
	TypeProject:               "/" + version.APIVersion + "/projects/%s",
	TypeCertificate:           "/" + version.APIVersion + "/certificates/%s",
	TypeInstance:              "/" + version.APIVersion + "/instances/%s",
	TypeInstanceBackup:        "/" + version.APIVersion + "/instances/%s/backups/%s",
	TypeInstanceSnapshot:      "/" + version.APIVersion + "/instances/%s/snapshots/%s",
	TypeNetwork:               "/" + version.APIVersion + "/networks/%s",
	TypeNetworkACL:            "/" + version.APIVersion + "/network-acls/%s",
	TypeNode:                  "/" + version.APIVersion + "/cluster/members/%s",
	TypeOperation:             "/" + version.APIVersion + "/operations/%s",
	TypeStoragePool:           "/" + version.APIVersion + "/storage-pools/%s",
	TypeStorageVolume:         "/" + version.APIVersion + "/storage-pools/%s/volumes/%s/%s",
	TypeStorageVolumeBackup:   "/" + version.APIVersion + "/storage-pools/%s/volumes/%s/%s/backups/%s",
	TypeStorageVolumeSnapshot: "/" + version.APIVersion + "/storage-pools/%s/volumes/%s/%s/snapshots/%s",
	TypeStorageBucket:         "/" + version.APIVersion + "/storage-pools/%s/buckets/%s",
	TypeWarning:               "/" + version.APIVersion + "/warnings/%s",
	TypeClusterGroup:          "/" + version.APIVersion + "/cluster/groups/%s",
	TypeServer:                "/" + version.APIVersion,
	TypeAuthUser:              "/" + version.APIVersion + "/auth/user/%s",
	TypeAuthGroup:             "/" + version.APIVersion + "/auth/group/%s",
	TypeImageAlias:            "/" + version.APIVersion + "/auth/images/aliases",
	TypeNetworkZone:           "/" + version.APIVersion + "/network-zones/%s",
}

func (t Type) String() string {
	name, ok := Names[t]
	if !ok {
		return "unknown"
	}

	return name
}

func (t Type) RequiresProject() bool {
	return shared.ValueInSlice(t, []Type{
		TypeContainer,
		TypeImage,
		TypeProfile,
		TypeInstance,
		TypeInstanceBackup,
		TypeInstanceSnapshot,
		TypeNetwork,
		TypeNetworkACL,
		TypeStorageVolume,
		TypeStorageVolumeBackup,
		TypeStorageVolumeSnapshot,
		TypeStorageBucket,
		TypeImageAlias,
		TypeNetworkZone,
	})
}

func (t Type) URL(projectName string, location string, pathArgs ...string) (string, error) {
	if t.RequiresProject() && projectName == "" {
		projectName = "default"
	}

	pathArgsEscaped := make([]any, 0, len(pathArgs))
	pathArgsCopy := make([]any, 0, len(pathArgs))
	for _, arg := range pathArgs {
		pathArgsCopy = append(pathArgsCopy, arg)
		pathArgsEscaped = append(pathArgsEscaped, url.PathEscape(arg))
	}

	uri, ok := UrIs[t]
	if !ok {
		panic("invalid entity type")
	}

	if len(pathArgs) != strings.Count(uri, "%s") {
		panic("not enough path arguments")
	}

	u := &url.URL{}
	u.Path = fmt.Sprintf(UrIs[t], pathArgsCopy...)
	u.RawPath = fmt.Sprintf(UrIs[t], pathArgsEscaped...)

	if t.RequiresProject() {
		u.Query().Set("project", projectName)
	}

	if location != "" {
		u.Query().Set("target", location)
	}

	u.RawQuery = u.Query().Encode()
	return u.String(), nil
}

func init() {
	for code, name := range Names {
		Types[name] = code
	}
}

// URLToType parses a raw URL string and returns the entity type, the project, the location and the path arguments. The
// returned project is set to "default" if it is not present (unless the entity type is TypeProject, in which case it is
// set to the value of the path parameter). An error is returned if the URL is not recognised.
func URLToType(rawURL string) (entityType Type, projectName string, location string, pathArguments []string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return -1, "", "", nil, fmt.Errorf("Failed to parse url %q into an entity type: %w", rawURL, err)
	}

	// We need to space separate the path because fmt.Sscanf uses this as a delimiter.
	spaceSeparatedURLPath := strings.Replace(u.Path, "/", " / ", -1)
	for entityType, entityURI := range UrIs {

		// Skip if we don't have the same number of slashes.
		if strings.Count(entityURI, "/") != strings.Count(u.Path, "/") {
			continue
		}

		spaceSeparatedEntityPath := strings.Replace(entityURI, "/", " / ", -1)

		// Make an []any for the number of expected path arguments and set each value in the slice to a *string.
		nPathArgs := strings.Count(spaceSeparatedEntityPath, "%s")
		pathArgsAny := make([]any, 0, nPathArgs)
		for i := 0; i < nPathArgs; i++ {
			var pathComponentStr string
			pathArgsAny = append(pathArgsAny, &pathComponentStr)
		}

		// Scan the given URL into the entity URL. If we found all the expected path arguments and there
		// are no errors we have a match.
		nFound, err := fmt.Sscanf(spaceSeparatedURLPath, spaceSeparatedEntityPath, pathArgsAny...)
		if nFound == nPathArgs && err == nil {
			pathArgs := make([]string, 0, nPathArgs)
			for _, pathArgAny := range pathArgsAny {
				pathArgPtr := pathArgAny.(*string)
				pathArg, err := url.PathUnescape(*pathArgPtr)
				if err != nil {
					return -1, "", "", nil, err
				}

				pathArgs = append(pathArgs, pathArg)
			}

			projectName := ""
			if entityType.RequiresProject() {
				projectName = u.Query().Get("project")
				if projectName == "" {
					projectName = "default"
				}
			}

			location := u.Query().Get("target")

			return entityType, projectName, location, pathArgs, nil
		}
	}

	return -1, "", "", nil, fmt.Errorf("Unknown entity URL %q", u.String())
}
