package types

import (
	"database/sql/driver"
	"errors"
	"fmt"
)

type StoragePoolVolumeContentType string

// Content types.
const (
	StoragePoolVolumeContentTypeFS    int64 = 0
	StoragePoolVolumeContentTypeBlock int64 = 1
	StoragePoolVolumeContentTypeISO   int64 = 2
)

const (
	StoragePoolVolumeContentTypeNameFS    = "filesystem"
	StoragePoolVolumeContentTypeNameBlock = "block"
	StoragePoolVolumeContentTypeNameISO   = "iso"
)

// Scan implements sql.Scanner for StoragePoolVolumeContentType. This converts the integer value back into the correct volume type name or
// returns an error.
func (a *StoragePoolVolumeContentType) Scan(value any) error {
	if a == nil {
		return errors.New("Cannot set nil storage volume content type")
	}

	if value == nil {
		return errors.New("Encountered null storage volume content type")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid storage pool volume type: %w", err)
	}

	volTypeInt64, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Storage pool volume type should be an integer, got `%v` (%T)", intValue, intValue)
	}

	switch volTypeInt64 {
	case StoragePoolVolumeContentTypeFS:
		*a = StoragePoolVolumeContentTypeNameFS
	case StoragePoolVolumeContentTypeBlock:
		*a = StoragePoolVolumeContentTypeNameBlock
	case StoragePoolVolumeContentTypeISO:
		*a = StoragePoolVolumeContentTypeNameISO
	default:
		return fmt.Errorf("Unknown storage pool volume content type `%d`", volTypeInt64)
	}

	return nil
}

// Value implements driver.Valuer for StoragePoolVolumeType. This converts the API constant into an integer or throws an error.
func (a StoragePoolVolumeContentType) Value() (driver.Value, error) {
	switch a {
	case StoragePoolVolumeContentTypeNameFS:
		return StoragePoolVolumeContentTypeFS, nil
	case StoragePoolVolumeContentTypeNameBlock:
		return StoragePoolVolumeContentTypeBlock, nil
	case StoragePoolVolumeContentTypeNameISO:
		return StoragePoolVolumeContentTypeISO, nil
	}

	return nil, fmt.Errorf("Invalid storage pool volume content type %q", a)
}
