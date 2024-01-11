//go:build linux && cgo && !agent

package cluster

import (
	"context"
	"database/sql"
)

// GroupCertificateGenerated is an interface of generated methods for GroupCertificate.
type GroupCertificateGenerated interface {
	// GetGroupCertificates returns all available Certificates for the Group.
	// generator: group_certificate GetMany
	GetGroupCertificates(ctx context.Context, tx *sql.Tx, groupID int) ([]Certificate, error)

	// DeleteGroupCertificates deletes the group_certificate matching the given key parameters.
	// generator: group_certificate DeleteMany
	DeleteGroupCertificates(ctx context.Context, tx *sql.Tx, groupID int) error

	// CreateGroupCertificates adds a new group_certificate to the database.
	// generator: group_certificate Create
	CreateGroupCertificates(ctx context.Context, tx *sql.Tx, objects []GroupCertificate) error

	// UpdateGroupCertificates updates the group_certificate matching the given key parameters.
	// generator: group_certificate Update
	UpdateGroupCertificates(ctx context.Context, tx *sql.Tx, groupID int, certificateFingerprints []string) error
}
