package types

import (
	"fmt"

	"database/sql/driver"
	"github.com/canonical/lxd/shared/api"
)

// AuthMethod is a database representation of an authentication method.
//
// AuthMethod is defined on string so that API constants can be converted by casting. The sql.Scanner and
// driver.Valuer interfaces are implemented on this type such that the string constants are converted into their int64
// counterparts as they are written to the database, or converted back into an AuthMethod as they are read from the
// database. It is not possible to read/write an invalid authentication methods from/to the database when using this type.
type AuthMethod string

const (
	AuthMethodTLS  int64 = 1
	AuthMethodOIDC int64 = 2
)

// Scan implements sql.Scanner for AuthMethod. This converts the integer value back into the correct API constant or
// returns an error.
func (a *AuthMethod) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("Authentication method cannot be null")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid authentication method type: %w", err)
	}

	authMethodInt, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Authentication method should be an integer, got `%v` (%T)", intValue, intValue)
	}

	switch authMethodInt {
	case AuthMethodTLS:
		*a = api.AuthenticationMethodTLS
	case AuthMethodOIDC:
		*a = api.AuthenticationMethodOIDC
	default:
		return fmt.Errorf("Unknown authentication method `%d`", authMethodInt)
	}

	return nil
}

// Value implements driver.Valuer for AuthMethod. This converts the API constant into an integer or throws an error.
func (a AuthMethod) Value() (driver.Value, error) {
	switch a {
	case api.AuthenticationMethodTLS:
		return AuthMethodTLS, nil
	case api.AuthenticationMethodOIDC:
		return AuthMethodOIDC, nil
	}

	return nil, fmt.Errorf("Invalid authentication method %q", a)
}
