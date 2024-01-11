package api

type Group struct {
	GroupPost `yaml:",inline"`
	GroupPut  `yaml:",inline"`
}

type GroupPost struct {
	// Name is the name of the group.
	Name string `json:"name" yaml:"name"`
}

type GroupPut struct {
	// Description is a short description of the group.
	Description string `json:"description" yaml:"description"`

	/* Entitlements are a map of `object_type:object_id` to the entitlement
	relations on that object.
	For example is JSON this might look like:
	{
		"project:default": ["viewer"]
		"project:foo": ["operator"]
		"instance:bar/baz": ["can_exec", "can_access_files"]
	}
	*/
	Entitlements map[string][]string `json:"entitlements" yaml:"entitlements"`

	// TLSUsers are the TLS users that are members of this group. This is a list
	// of certificate fingerprints.
	TLSUsers []string `json:"tls_users" yaml:"tls_users"`

	// OIDCUsers are the OIDC users that are members of this group. This is a
	// list of OIDC subjects.
	OIDCUsers []string `json:"oidc_users" yaml:"oidc_users"`

	// IdPGroups are a list of groups from the IdP whose mapping includes this
	// group.
	IdPGroups []string `json:"idp_groups" yaml:"idp_groups"`
}

type IdPGroup struct {
	IdPGroupPost `yaml:",inline"`
	IdPGroupPut  `yaml:",inline"`
}

type IdPGroupPost struct {
	// Name is the name of the IdP group.
	Name string `json:"name" yaml:"name"`
}

type IdPGroupPut struct {
	// Groups are the groups the IdP group resolves to.
	Groups []string `json:"groups" yaml:"groups"`
}
