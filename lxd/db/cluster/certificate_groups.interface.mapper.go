//go:build linux && cgo && !agent

package cluster

import (
	"context"
	"database/sql"
)

// CertificateGroupGenerated is an interface of generated methods for CertificateGroup.
type CertificateGroupGenerated interface {
	// GetCertificateGroups returns all available Groups for the Certificate.
	// generator: certificate_group GetMany
	GetCertificateGroups(ctx context.Context, tx *sql.Tx, certificateID int) ([]Group, error)

	// DeleteCertificateGroups deletes the certificate_group matching the given key parameters.
	// generator: certificate_group DeleteMany
	DeleteCertificateGroups(ctx context.Context, tx *sql.Tx, certificateID int) error

	// CreateCertificateGroups adds a new certificate_group to the database.
	// generator: certificate_group Create
	CreateCertificateGroups(ctx context.Context, tx *sql.Tx, objects []CertificateGroup) error

	// UpdateCertificateGroups updates the certificate_group matching the given key parameters.
	// generator: certificate_group Update
	UpdateCertificateGroups(ctx context.Context, tx *sql.Tx, certificateID int, groupNames []string) error
}
