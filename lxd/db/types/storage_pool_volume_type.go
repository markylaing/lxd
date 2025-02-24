package types

import (
	"database/sql/driver"
	"errors"
	"fmt"
)

type StoragePoolVolumeType string

const (
	StoragePoolVolumeTypeContainer int64 = 0
	StoragePoolVolumeTypeImage     int64 = 1
	StoragePoolVolumeTypeCustom    int64 = 2
	StoragePoolVolumeTypeVM        int64 = 3
)

const (
	StoragePoolVolumeTypeNameContainer = "container"
	StoragePoolVolumeTypeNameVM        = "virtual-machine"
	StoragePoolVolumeTypeNameImage     = "image"
	StoragePoolVolumeTypeNameCustom    = "custom"
)

// StoragePoolVolumeTypeNames represents a map of storage volume types and their names.
var StoragePoolVolumeTypeNames = map[int64]StoragePoolVolumeType{
	StoragePoolVolumeTypeContainer: StoragePoolVolumeTypeNameContainer,
	StoragePoolVolumeTypeImage:     StoragePoolVolumeTypeNameImage,
	StoragePoolVolumeTypeCustom:    StoragePoolVolumeTypeNameCustom,
	StoragePoolVolumeTypeVM:        StoragePoolVolumeTypeNameVM,
}

// Scan implements sql.Scanner for StoragePoolVolumeType. This converts the integer value back into the correct volume type name or
// returns an error.
func (a *StoragePoolVolumeType) Scan(value any) error {
	if a == nil {
		return errors.New("Cannot set nil storage volume type")
	}

	if value == nil {
		return errors.New("Encountered null value for storage volume type")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid storage pool volume type: %w", err)
	}

	volTypeInt64, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Storage pool volume type should be an integer, got `%v` (%T)", intValue, intValue)
	}

	t, ok := StoragePoolVolumeTypeNames[volTypeInt64]
	if !ok {
		return fmt.Errorf("Unknown storage pool volume type `%d`", volTypeInt64)
	}

	*a = t
	return nil
}

// Value implements driver.Valuer for StoragePoolVolumeType. This converts the API constant into an integer or throws an error.
func (a StoragePoolVolumeType) Value() (driver.Value, error) {
	switch a {
	case StoragePoolVolumeTypeNameContainer:
		return StoragePoolVolumeTypeContainer, nil
	case StoragePoolVolumeTypeNameImage:
		return StoragePoolVolumeTypeImage, nil
	case StoragePoolVolumeTypeNameCustom:
		return StoragePoolVolumeTypeCustom, nil
	case StoragePoolVolumeTypeNameVM:
		return StoragePoolVolumeTypeVM, nil
	}

	return nil, fmt.Errorf("Invalid storage pool volume type %q", a)
}
