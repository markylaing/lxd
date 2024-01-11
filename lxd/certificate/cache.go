package certificate

import (
	"crypto/x509"
	"sync"
)

// Cache represents a thread-safe in-memory cache of the certificates in the database.
type Cache struct {
	// certificates is a map of certificate Type to map of certificate fingerprint to x509.Certificate.
	certificates map[Type]map[string]x509.Certificate

	// projects is a map of certificate fingerprint to slice of projects the certificate is restricted to.
	// If a certificate fingerprint is present in certificates, but not present in projects or groups, it means the certificate is
	// not restricted.
	projects map[string][]string

	// groups is a map of certificate fingerprint to slice of group names that the certificate is a member of.
	// If a certificate fingerprint is present in certificates, but not present in projects or groups, it means the certificate is
	// not restricted.
	groups map[string][]string

	mu sync.RWMutex
}

// SetCertificatesProjectsAndGroups sets both certificates and projects on the Cache.
func (c *Cache) SetCertificatesProjectsAndGroups(certificates map[Type]map[string]x509.Certificate, projects map[string][]string, groups map[string][]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.certificates = certificates
	c.projects = projects
	c.groups = groups
}

// SetCertificates sets the certificates on the Cache.
func (c *Cache) SetCertificates(certificates map[Type]map[string]x509.Certificate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.certificates = certificates
}

// GetCertificatesAndProjects returns a read-only copy of the certificate and project maps.
func (c *Cache) GetCertificatesProjectsAndGroups() (map[Type]map[string]x509.Certificate, map[string][]string, map[string][]string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	certificates := make(map[Type]map[string]x509.Certificate, len(c.certificates))
	for t, m := range c.certificates {
		certificates[t] = make(map[string]x509.Certificate, len(m))
		for f, cert := range m {
			certificates[t][f] = cert
		}
	}

	projects := make(map[string][]string, len(c.projects))
	for f, projectNames := range c.projects {
		projectNamesCopy := make([]string, 0, len(projectNames))
		projectNamesCopy = append(projectNamesCopy, projectNames...)
		projects[f] = projectNamesCopy
	}

	groups := make(map[string][]string, len(c.groups))
	for f, groupNames := range c.groups {
		groupsNamesCopy := make([]string, 0, len(groupNames))
		groupsNamesCopy = append(groupsNamesCopy, groupNames...)
		groups[f] = groupsNamesCopy
	}

	return certificates, projects, groups
}

// GetCertificates returns a read-only copy of the certificate map.
func (c *Cache) GetCertificates() map[Type]map[string]x509.Certificate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	certificates := make(map[Type]map[string]x509.Certificate, len(c.certificates))
	for t, m := range c.certificates {
		certificates[t] = make(map[string]x509.Certificate, len(m))
		for f, cert := range m {
			certificates[t][f] = cert
		}
	}

	return certificates
}
