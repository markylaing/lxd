package certificate

import (
	"encoding/json"
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// Type indicates the type of the certificate.
type Type int

// TypeClient indicates a client certificate type.
const TypeClient = Type(1)

// TypeServer indicates a server certificate type.
const TypeServer = Type(2)

// TypeMetrics indicates a metrics certificate type.
const TypeMetrics = Type(3)

// FromAPIType converts an API type to the equivalent Type.
func FromAPIType(apiType string) (Type, error) {
	switch apiType {
	case api.CertificateTypeClient:
		return TypeClient, nil
	case api.CertificateTypeServer:
		return TypeServer, nil
	case api.CertificateTypeMetrics:
		return TypeMetrics, nil
	}

	return -1, fmt.Errorf("Invalid certificate type")
}

func (t Type) String() string {
	switch t {
	case TypeClient:
		return api.CertificateTypeClient
	case TypeServer:
		return api.CertificateTypeServer
	case TypeMetrics:
		return api.CertificateTypeMetrics
	default:
		return api.CertificateTypeUnknown
	}
}

func (t *Type) SetString(s string) error {
	newT, err := FromAPIType(s)
	if err != nil {
		return nil
	}

	*t = newT
	return nil
}

func (t *Type) UnmarshalJSON(b []byte) error {
	var typeString string
	err := json.Unmarshal(b, &typeString)
	if err != nil {
		return err
	}

	return t.SetString(typeString)
}

func (t Type) MarshalJSON() ([]byte, error) {
	str := t.String()
	_, err := FromAPIType(str)
	if err != nil {
		return nil, err
	}

	return json.Marshal(str)
}
