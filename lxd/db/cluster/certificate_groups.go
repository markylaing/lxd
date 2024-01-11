package cluster

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t certificate_groups.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e certificate_group objects
//go:generate mapper stmt -e certificate_group objects-by-CertificateID
//go:generate mapper stmt -e certificate_group create struct=CertificateGroup
//go:generate mapper stmt -e certificate_group delete-by-CertificateID
//
//go:generate mapper method -i -e certificate_group GetMany struct=Certificate
//go:generate mapper method -i -e certificate_group DeleteMany struct=Certificate
//go:generate mapper method -i -e certificate_group Create struct=Certificate
//go:generate mapper method -i -e certificate_group Update struct=Certificate

// CertificateGroup is an association table struct that associates
// Certificates to Groups.
type CertificateGroup struct {
	CertificateID int `db:"primary=yes"`
	GroupID       int
}

// CertificateGroupFilter specifies potential query parameter fields.
type CertificateGroupFilter struct {
	CertificateID *int
	GroupID       *int
}
