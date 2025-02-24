package types

// entityTypeServer implements EntityTypeDB for a Server.
type entityTypeServer struct{}

func (e entityTypeServer) Code() int64 {
	return entityTypeCodeServer
}

// allURLsQuery returns an empty string because there are no Server entities in the database.
func (e entityTypeServer) AllURLsQuery() string {
	return ""
}

func (e entityTypeServer) URLsByProjectQuery() string {
	return ""
}

func (e entityTypeServer) URLByIDQuery() string {
	return ""
}

// idFromURLQuery returns an empty string because there are no Server entities in the database.
func (e entityTypeServer) IDFromURLQuery() string {
	return ""
}

func (e entityTypeServer) OnDeleteTriggerSQL() (name string, sql string) {
	return "", ""
}
