package types

import (
	"fmt"

	"github.com/canonical/lxd/shared/api"
)

// entityTypeIdentity implements EntityTypeDB for an Identity.
type entityTypeIdentity struct{}

func (e entityTypeIdentity) Code() int64 {
	return entityTypeCodeIdentity
}

func (e entityTypeIdentity) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT 
	%d, 
	identities.id, 
	'', 
	'', 
	json_array(
		CASE identities.auth_method
			WHEN %d THEN '%s'
			WHEN %d THEN '%s'
		END,
		identities.identifier
	) 
FROM identities
WHERE type IN (%d, %d, %d)
`,
		e.Code(),
		AuthMethodTLS, api.AuthenticationMethodTLS,
		AuthMethodOIDC, api.AuthenticationMethodOIDC,
		IdentityTypeOIDCClient, IdentityTypeCertificateClient, IdentityTypeCertificateClientPending,
	)
}

func (e entityTypeIdentity) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeIdentity) URLByIDQuery() string {
	return fmt.Sprintf(`%s AND identities.id = ?`, e.AllURLsQuery())
}

func (e entityTypeIdentity) IDFromURLQuery() string {
	return fmt.Sprintf(`
SELECT ?, identities.id 
FROM identities 
WHERE '' = ? 
	AND '' = ? 
	AND CASE identities.auth_method 
		WHEN %d THEN '%s' 
		WHEN %d THEN '%s' 
	END = ? 
	AND identities.identifier = ?
	AND identities.type IN (%d)
`, AuthMethodTLS, api.AuthenticationMethodTLS,
		AuthMethodOIDC, api.AuthenticationMethodOIDC,
		IdentityTypeOIDCClient,
	)
}

func (e entityTypeIdentity) OnDeleteTriggerSQL() (name string, sql string) {
	name = "on_identity_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON identities
	BEGIN
	DELETE FROM auth_groups_permissions 
		WHERE entity_type = %d 
		AND entity_id = OLD.id;
	DELETE FROM warnings
		WHERE entity_type_code = %d
		AND entity_id = OLD.id;
	END
`, name, e.Code(), e.Code())
}
