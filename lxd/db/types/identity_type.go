package types

import (
	"database/sql/driver"
	"fmt"
	"github.com/canonical/lxd/lxd/certificate"
	"github.com/canonical/lxd/shared/api"
)

// IdentityType indicates the type of the identity.
//
// IdentityType is defined on string so that API constants can be converted by casting. The sql.Scanner and
// driver.Valuer interfaces are implemented on this type such that the string constants are converted into their int64
// counterparts as they are written to the database, or converted back into an IdentityType as they are read from the
// database. It is not possible to read/write an invalid identity types from/to the database when using this type.
type IdentityType string

const (
	IdentityTypeCertificateClientRestricted    int64 = 1
	IdentityTypeCertificateClientUnrestricted  int64 = 2
	IdentityTypeCertificateServer              int64 = 3
	IdentityTypeCertificateMetricsRestricted   int64 = 4
	IdentityTypeOIDCClient                     int64 = 5
	IdentityTypeCertificateMetricsUnrestricted int64 = 6
	IdentityTypeCertificateClient              int64 = 7
	IdentityTypeCertificateClientPending       int64 = 8
)

// Scan implements sql.Scanner for IdentityType. This converts the integer value back into the correct API constant or
// returns an error.
func (i *IdentityType) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("Identity type cannot be null")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid identity type: %w", err)
	}

	identityTypeInt, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Identity type should be an integer, got `%v` (%T)", intValue, intValue)
	}

	switch identityTypeInt {
	case IdentityTypeCertificateClientRestricted:
		*i = api.IdentityTypeCertificateClientRestricted
	case IdentityTypeCertificateClientUnrestricted:
		*i = api.IdentityTypeCertificateClientUnrestricted
	case IdentityTypeCertificateServer:
		*i = api.IdentityTypeCertificateServer
	case IdentityTypeCertificateMetricsRestricted:
		*i = api.IdentityTypeCertificateMetricsRestricted
	case IdentityTypeCertificateMetricsUnrestricted:
		*i = api.IdentityTypeCertificateMetricsUnrestricted
	case IdentityTypeOIDCClient:
		*i = api.IdentityTypeOIDCClient
	case IdentityTypeCertificateClient:
		*i = api.IdentityTypeCertificateClient
	case IdentityTypeCertificateClientPending:
		*i = api.IdentityTypeCertificateClientPending
	default:
		return fmt.Errorf("Unknown identity type `%d`", identityTypeInt)
	}

	return nil
}

// Value implements driver.Valuer for IdentityType. This converts the API constant into an integer or throws an error.
func (i IdentityType) Value() (driver.Value, error) {
	switch i {
	case api.IdentityTypeCertificateClientRestricted:
		return IdentityTypeCertificateClientRestricted, nil
	case api.IdentityTypeCertificateClientUnrestricted:
		return IdentityTypeCertificateClientUnrestricted, nil
	case api.IdentityTypeCertificateServer:
		return IdentityTypeCertificateServer, nil
	case api.IdentityTypeCertificateMetricsRestricted:
		return IdentityTypeCertificateMetricsRestricted, nil
	case api.IdentityTypeCertificateMetricsUnrestricted:
		return IdentityTypeCertificateMetricsUnrestricted, nil
	case api.IdentityTypeOIDCClient:
		return IdentityTypeOIDCClient, nil
	case api.IdentityTypeCertificateClient:
		return IdentityTypeCertificateClient, nil
	case api.IdentityTypeCertificateClientPending:
		return IdentityTypeCertificateClientPending, nil
	}

	return nil, fmt.Errorf("Invalid identity type %q", i)
}

// ToCertificateType returns the equivalent certificate.Type for the IdentityType.
func (i IdentityType) ToCertificateType() (certificate.Type, error) {
	switch i {
	case api.IdentityTypeCertificateClientRestricted:
		return certificate.TypeClient, nil
	case api.IdentityTypeCertificateClientUnrestricted:
		return certificate.TypeClient, nil
	case api.IdentityTypeCertificateServer:
		return certificate.TypeServer, nil
	case api.IdentityTypeCertificateMetricsRestricted:
		return certificate.TypeMetrics, nil
	case api.IdentityTypeCertificateMetricsUnrestricted:
		return certificate.TypeMetrics, nil
	}

	return -1, fmt.Errorf("Identity type %q is not a certificate", i)
}
