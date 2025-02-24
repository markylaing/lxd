package types

import (
	"fmt"
)

// entityTypeCertificate implements EntityTypeDB for a Certificate.
type entityTypeCertificate struct{}

func (e entityTypeCertificate) Code() int64 {
	return entityTypeCodeCertificate
}

func (e entityTypeCertificate) AllURLsQuery() string {
	return fmt.Sprintf(
		`SELECT %d, identities.id, '', '', json_array(identities.identifier) FROM identities WHERE auth_method = %d AND type IN (%d, %d, %d, %d, %d)`,
		e.Code(),
		AuthMethodTLS,
		IdentityTypeCertificateClientRestricted,
		IdentityTypeCertificateClientUnrestricted,
		IdentityTypeCertificateServer,
		IdentityTypeCertificateMetricsRestricted,
		IdentityTypeCertificateMetricsUnrestricted,
	)
}

func (e entityTypeCertificate) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeCertificate) URLByIDQuery() string {
	return fmt.Sprintf(`%s AND identities.id = ?`, e.AllURLsQuery())
}

func (e entityTypeCertificate) IDFromURLQuery() string {
	return fmt.Sprintf(`
SELECT ?, identities.id 
FROM identities 
WHERE '' = ? 
	AND '' = ? 
	AND identities.identifier = ? 
	AND identities.auth_method = %d
	AND identities.type IN (%d, %d, %d, %d, %d)
`, AuthMethodTLS,
		IdentityTypeCertificateClientRestricted,
		IdentityTypeCertificateClientUnrestricted,
		IdentityTypeCertificateServer,
		IdentityTypeCertificateMetricsRestricted,
		IdentityTypeCertificateMetricsUnrestricted,
	)
}

func (e entityTypeCertificate) OnDeleteTriggerSQL() (name string, sql string) {
	return "", ""
}
