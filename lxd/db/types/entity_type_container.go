package types

import (
	"fmt"

	"github.com/canonical/lxd/lxd/instance/instancetype"
)

// entityTypeContainer implements EntityTypeDB for a Container.
type entityTypeContainer struct{}

func (e entityTypeContainer) Code() int64 {
	return entityTypeCodeContainer
}

func (e entityTypeContainer) AllURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, instances.id, projects.name, '', json_array(instances.name) 
FROM instances 
JOIN projects ON instances.project_id = projects.id 
WHERE instances.type = %d
`, e.Code(), instancetype.Container)
}

func (e entityTypeContainer) URLsByProjectQuery() string {
	return fmt.Sprintf(`%s AND projects.name = ?`, e.AllURLsQuery())
}

func (e entityTypeContainer) URLByIDQuery() string {
	return fmt.Sprintf(`%s AND instances.id = ?`, e.AllURLsQuery())
}

func (e entityTypeContainer) IDFromURLQuery() string {
	return fmt.Sprintf(`
SELECT ?, instances.id 
FROM instances 
JOIN projects ON instances.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND instances.name = ? 
	AND instances.type = %d
`, instancetype.Container)
}

func (e entityTypeContainer) OnDeleteTriggerSQL() (name string, sql string) {
	return "", ""
}
