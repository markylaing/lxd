package api

type Entitlement struct {
	// Object is the object the entitlement is defined against. This is the
	// object type and object ref delimited by a colon, e.g.
	// {objectType}:{objectRef}.
	Object string `json:"object" yaml:"object"`

	// Relation is one of the OpenFGA relations that are defined for the object
	// type.
	Relation string `json:"relation" yaml:"relation"`

	// Groups is a list of groups that are currently have the entitlement.
	Groups []string `json:"groups" yaml:"groups"`
}
