package identity

import (
	"crypto/x509"
	"net/http"
	"slices"
	"sync"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/datastructures"
)

// Cache represents a thread-safe in-memory cache of the identities in the database.
type Cache struct {
	serverCertificates map[string]*x509.Certificate
	clientCertificates map[string]*x509.Certificate
	secrets            map[string][]byte
	mu                 sync.RWMutex
}

func (c *Cache) getCertificates(m map[string]*x509.Certificate, fingerprints ...string) map[string]x509.Certificate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out, _ := datastructures.MapToMapFilter(m, func(k string, v *x509.Certificate) (bool, error) {
		return len(fingerprints) == 0 || slices.Contains(fingerprints, k), nil
	}, func(k string, v *x509.Certificate) (string, x509.Certificate, error) {
		return k, *v, nil
	})

	return out
}

// GetServerCertificates returns matching server certificates.
func (c *Cache) GetServerCertificates(fingerprints ...string) map[string]x509.Certificate {
	return c.getCertificates(c.serverCertificates, fingerprints...)
}

// GetClientCertificates returns matching client certificates.
func (c *Cache) GetClientCertificates(fingerprints ...string) map[string]x509.Certificate {
	return c.getCertificates(c.clientCertificates, fingerprints...)
}

// GetSecret returns the secret of a bearer identity by their UUID.
func (c *Cache) GetSecret(bearerIdentityUUID string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	secret, ok := c.secrets[bearerIdentityUUID]
	if !ok {
		return nil, api.NewStatusError(http.StatusNotFound, "No secret found for bearer token identity")
	}

	return secret, nil
}

// ReplaceAll deletes all entries and identity provider groups from the cache and replaces them with the given values.
func (c *Cache) ReplaceAll(serverCerts map[string]*x509.Certificate, clientCerts map[string]*x509.Certificate, secrets map[string][]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.serverCertificates = serverCerts
	c.clientCertificates = clientCerts
	c.secrets = secrets
	return nil
}
