package types

import (
	"database/sql/driver"
	"fmt"
	"github.com/canonical/lxd/shared/entity"
)

// EntityType is a database representation of an entity type.
//
// EntityType is defined on string so that entity.Type constants can be converted by casting. The sql.Scanner and
// driver.Valuer interfaces are implemented on this type such that the string constants are converted into their int64
// counterparts as they are written to the database, or converted back into an EntityType as they are read from the
// database. It is not possible to read/write invalid entity types from/to the database when using this type.
type EntityType string

// EntityTypeDB defines how an entity type behaves at the database level.
//
// To create a new entity type, first create a new `(shared/entity).Type` then create a type that implements the methods
// defined on EntityTypeDB.
//
// The code method must return a unique int64 for the entity type (see the entityTypeCode constants below). Other SQL
// related method may return an empty string if the method is not applicable to the entity type. For example,
// urlsByProjectQuery is only applicable to entity types that are project specific, so it is ok to return an empty string
// from this method for the e.g. certificate entity type.
type EntityTypeDB interface {
	// Code must return a unique int64 for the entity type.
	Code() int64

	// AllURLsQuery must return a SQL query that when executed, returns values in this order and format:
	// 1. The type code of the entity type.
	// 2. The ID of the entity in it's corresponding database table.
	// 3. The project that contains the entity (or an empty string if the entity is not project specific).
	// 4. The location of the entity (or an empty string if the entity is not localised to a specific member).
	// 5. A JSON formatted array of path arguments that compose the URL of the entity.
	//
	// urlByIDQuery and urlsByProjectQuery must also follow this format. This is so that the three query types can be
	// composed with a SQL UNION. Returning the code of the entity type as the first argument allows us to UNION these
	// queries over multiple entity types. This reduces the total number of queries that need to be performed.
	AllURLsQuery() string

	// URLByIDQuery must return a SQL query that when executed, returns values identically to allURLsQuery. The query
	// must accept a single integer bind argument for the ID of the resource.
	URLByIDQuery() string

	// URLsByProjectQuery must return a SQL query that when executed, returns values identically to allURLsQuery. The
	// query must accept a single string bind argument for the project name.
	URLsByProjectQuery() string

	// IDFromURLQuery must return a SQL query that returns an identifier for the query, and the ID of the entity in the
	// database. It expects the following bind arguments:
	// 1. An identifier for this returned row. This is because these queries are designed to work in UNION with queries
	//    of other entity types.
	// 2. The project name (even if the entity is not project specific, this should be passed as an empty string).
	// 3. The location (even if the entity is not location specific, this should be passed as an empty string).
	// 4. All path arguments from the URL.
	IDFromURLQuery() string

	// OnDeleteTriggerSQL must return the SQL for a trigger that runs when an entity of this type is deleted. These
	// triggers are in place so that warnings and group permissions do not contain stale entries. The first return value
	// must be the name of the trigger, the second return value must be the SQL for creating the trigger.
	OnDeleteTriggerSQL() (name string, sql string)
}

func EntityTypes() map[entity.Type]EntityTypeDB {
	return entityTypes
}

var entityTypes = map[entity.Type]EntityTypeDB{
	entity.TypeContainer:             entityTypeContainer{},
	entity.TypeImage:                 entityTypeImage{},
	entity.TypeProfile:               entityTypeProfile{},
	entity.TypeProject:               entityTypeProject{},
	entity.TypeCertificate:           entityTypeCertificate{},
	entity.TypeInstance:              entityTypeInstance{},
	entity.TypeInstanceBackup:        entityTypeInstanceBackup{},
	entity.TypeInstanceSnapshot:      entityTypeInstanceSnapshot{},
	entity.TypeNetwork:               entityTypeNetwork{},
	entity.TypeNetworkACL:            entityTypeNetworkACL{},
	entity.TypeClusterMember:         entityTypeClusterMember{},
	entity.TypeOperation:             entityTypeOperation{},
	entity.TypeStoragePool:           entityTypeStoragePool{},
	entity.TypeStorageVolume:         entityTypeStorageVolume{},
	entity.TypeStorageVolumeBackup:   entityTypeStorageVolumeBackup{},
	entity.TypeStorageVolumeSnapshot: entityTypeStorageVolumeSnapshot{},
	entity.TypeWarning:               entityTypeWarning{},
	entity.TypeClusterGroup:          entityTypeClusterGroup{},
	entity.TypeStorageBucket:         entityTypeStorageBucket{},
	entity.TypeServer:                entityTypeServer{},
	entity.TypeImageAlias:            entityTypeImageAlias{},
	entity.TypeNetworkZone:           entityTypeNetworkZone{},
	entity.TypeIdentity:              entityTypeIdentity{},
	entity.TypeAuthGroup:             entityTypeAuthGroup{},
	entity.TypeIdentityProviderGroup: entityTypeIdentityProviderGroup{},
}

const (
	entityTypeCodeNone                  int64 = -1
	entityTypeCodeContainer             int64 = 0
	entityTypeCodeImage                 int64 = 1
	entityTypeCodeProfile               int64 = 2
	entityTypeCodeProject               int64 = 3
	entityTypeCodeCertificate           int64 = 4
	entityTypeCodeInstance              int64 = 5
	entityTypeCodeInstanceBackup        int64 = 6
	entityTypeCodeInstanceSnapshot      int64 = 7
	entityTypeCodeNetwork               int64 = 8
	entityTypeCodeNetworkACL            int64 = 9
	entityTypeCodeClusterMember         int64 = 10
	entityTypeCodeOperation             int64 = 11
	entityTypeCodeStoragePool           int64 = 12
	entityTypeCodeStorageVolume         int64 = 13
	entityTypeCodeStorageVolumeBackup   int64 = 14
	entityTypeCodeStorageVolumeSnapshot int64 = 15
	entityTypeCodeWarning               int64 = 16
	entityTypeCodeClusterGroup          int64 = 17
	entityTypeCodeStorageBucket         int64 = 18
	entityTypeCodeNetworkZone           int64 = 19
	entityTypeCodeImageAlias            int64 = 20
	entityTypeCodeServer                int64 = 21
	entityTypeCodeAuthGroup             int64 = 22
	entityTypeCodeIdentityProviderGroup int64 = 23
	entityTypeCodeIdentity              int64 = 24
)

var entityTypeByCode = map[int64]EntityType{
	entityTypeCodeNone: EntityType(""),
}

func init() {
	for entityType, info := range entityTypes {
		entityTypeByCode[info.Code()] = EntityType(entityType)
	}
}

// Scan implements sql.Scanner for EntityType. This converts the integer value back into the correct entity.Type
// constant or returns an error.
func (e *EntityType) Scan(value any) error {
	// Always expect null values to be coalesced into entityTypeNone (-1).
	if value == nil {
		return fmt.Errorf("Entity type cannot be null")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid entity type `%v`: %w", value, err)
	}

	entityTypeInt, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Entity should be an integer, got `%v` (%T)", intValue, intValue)
	}

	entityType, ok := entityTypeByCode[entityTypeInt]
	if !ok {
		return fmt.Errorf("Unknown entity type %d", entityTypeInt)
	}

	*e = entityType
	return nil
}

// Value implements driver.Valuer for EntityType. This converts the EntityType into an integer or throws an error.
func (e EntityType) Value() (driver.Value, error) {
	if e == "" {
		return entityTypeCodeNone, nil
	}

	info, ok := entityTypes[entity.Type(e)]
	if !ok {
		return nil, fmt.Errorf("Unknown entity type %q", e)
	}

	return info.Code(), nil
}
