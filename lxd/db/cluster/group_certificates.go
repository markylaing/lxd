package cluster

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t group_certificates.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e group_certificate objects table=certificates_groups
//go:generate mapper stmt -e group_certificate objects-by-GroupID table=certificates_groups
//go:generate mapper stmt -e group_certificate create struct=GroupCertificate table=certificates_groups
//go:generate mapper stmt -e group_certificate delete-by-GroupID table=certificates_groups
//
//go:generate mapper method -i -e group_certificate GetMany struct=Group
//go:generate mapper method -i -e group_certificate DeleteMany struct=Group
//go:generate mapper method -i -e group_certificate Create struct=Group
//go:generate mapper method -i -e group_certificate Update struct=Group

// GroupCertificate is an association table struct that associates
// Groups to Certificates.
type GroupCertificate struct {
	CertificateID int
	GroupID       int `db:"primary=yes"`
}

// CertificateGroupFilter specifies potential query parameter fields.
type GroupCertificateFilter struct {
	CertificateID *int
	GroupID       *int
}
