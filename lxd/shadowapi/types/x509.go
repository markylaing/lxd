package types

import (
	"crypto/x509"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
)

// X509CertificatePEM is a json/yaml marshallable/unmarshallable type wrapper for x509.Certificate.
//
//swagger:type string
type X509CertificatePEM struct {
	*x509.Certificate
}

func (c *X509CertificatePEM) SetString(in string) error {
	if in == "" {
		return nil
	}

	cert, err := shared.ParseCert([]byte(in))
	if err != nil {
		return api.StatusErrorf(http.StatusBadRequest, "Failed to parse PEM encoded x509 certificate: %w", err)
	}

	c.Certificate = cert
	return nil
}

// String returns the x509.Certificate as a PEM encoded string.
func (c X509CertificatePEM) String() string {
	if c.Certificate == nil {
		return ""
	}

	return shared.CertPEM(c.Certificate)
}

// MarshalJSON implements json.Marshaler for X509CertificatePEM.
func (c X509CertificatePEM) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// MarshalYAML implements yaml.Marshaller for X509CertificatePEM.
func (c X509CertificatePEM) MarshalYAML() (any, error) {
	return c.String(), nil
}

// UnmarshalJSON implements json.Unmarshaler for X509CertificatePEM.
func (c *X509CertificatePEM) UnmarshalJSON(b []byte) error {
	var certStr string
	err := json.Unmarshal(b, &certStr)
	if err != nil {
		return err
	}

	return c.SetString(certStr)
}

// UnmarshalYAML implements yaml.Unmarshaler for X509CertificatePEM.
func (c *X509CertificatePEM) UnmarshalYAML(unmarshal func(v any) error) error {
	var certStr string
	err := unmarshal(&certStr)
	if err != nil {
		return err
	}

	return c.SetString(certStr)
}

// IsEmpty checks if the X509CertificatePEM has an underlying certificate or is empty.
func (c X509CertificatePEM) IsEmpty() bool {
	return c.Certificate == nil
}

func (c *X509CertificatePEM) Scan(value any) error {
	if value == nil {
		return nil
	}

	strValue, err := driver.String.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid PEM encoded X509 certificate type: %w", err)
	}

	certStr, ok := strValue.(string)
	if !ok {
		return fmt.Errorf("PEM encoded X509 certificate should be a string, got `%v` (%T)", strValue, strValue)
	}

	return c.SetString(certStr)
}

func (c X509CertificatePEM) Value() (driver.Value, error) {
	return c.String(), nil
}

// X509CertificateBase64 is a json/yaml marshallable/unmarshallable type wrapper for x509.Certificate.
//
//swagger:type string
type X509CertificateBase64 struct {
	*x509.Certificate
}

func (c *X509CertificateBase64) SetString(in string) error {
	if in == "" {
		return nil
	}

	// Add supplied certificate.
	data, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return api.StatusErrorf(http.StatusBadRequest, "Invalid base64 encoding: %w", err)
	}

	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return api.StatusErrorf(http.StatusBadRequest, "Invalid certificate material: %w", err)
	}

	c.Certificate = cert
	return nil
}

// String returns the x509.Certificate as a PEM encoded string.
func (c X509CertificateBase64) String() string {
	if c.Certificate == nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(c.Certificate.Raw)
}

// MarshalJSON implements json.Marshaler for X509CertificateBase64.
func (c X509CertificateBase64) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// MarshalYAML implements yaml.Marshaller for X509CertificateBase64.
func (c X509CertificateBase64) MarshalYAML() (any, error) {
	return c.String(), nil
}

// UnmarshalJSON implements json.Unmarshaler for X509CertificateBase64.
func (c *X509CertificateBase64) UnmarshalJSON(b []byte) error {
	var certStr string
	err := json.Unmarshal(b, &certStr)
	if err != nil {
		return err
	}

	return c.SetString(certStr)
}

// UnmarshalYAML implements yaml.Unmarshaler for X509CertificateBase64.
func (c *X509CertificateBase64) UnmarshalYAML(unmarshal func(v any) error) error {
	var certStr string
	err := unmarshal(&certStr)
	if err != nil {
		return err
	}

	return c.SetString(certStr)
}

// IsEmpty checks if the X509CertificateBase64 has an underlying certificate or is empty.
func (c X509CertificateBase64) IsEmpty() bool {
	return c.Certificate == nil
}

func (c *X509CertificateBase64) Scan(value any) error {
	if value == nil {
		return nil
	}

	strValue, err := driver.String.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid base64 encoded x509 certificate type: %w", err)
	}

	certStr, ok := strValue.(string)
	if !ok {
		return fmt.Errorf("Base64 encoded X509 certificate should be a string, got `%v` (%T)", strValue, strValue)
	}

	return c.SetString(certStr)
}

func (c X509CertificateBase64) Value() (driver.Value, error) {
	return c.String(), nil
}
