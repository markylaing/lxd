package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/entity"
)

// entityTypeInstance implements entityTypeDBInfo for an Instance.
type entityTypeInstance struct{}

func (e entityTypeInstance) code() int64 {
	return entityTypeCodeInstance
}

func (e entityTypeInstance) allURLsQuery() string {
	return fmt.Sprintf(`
SELECT %d, instances.id, projects.name, '', json_array(instances.name) 
FROM instances 
JOIN projects ON instances.project_id = projects.id`, e.code())
}

func (e entityTypeInstance) urlsByProjectQuery() string {
	return fmt.Sprintf(`%s WHERE projects.name = ?`, e.allURLsQuery())
}

func (e entityTypeInstance) urlByIDQuery() string {
	return fmt.Sprintf(`%s WHERE instances.id = ?`, e.allURLsQuery())
}

func (e entityTypeInstance) idFromURLQuery() string {
	return `
SELECT ?, instances.id 
FROM instances 
JOIN projects ON instances.project_id = projects.id 
WHERE projects.name = ? 
	AND '' = ? 
	AND instances.name = ?`
}

func (e entityTypeInstance) onDeleteTriggerSQL() (name string, sql string) {
	name = "on_instance_delete"
	return name, fmt.Sprintf(`
CREATE TRIGGER %s
	AFTER DELETE ON instances
	BEGIN
	DELETE FROM auth_groups_permissions 
		WHERE entity_type = %d 
		AND entity_id = OLD.id;
	DELETE FROM warnings
		WHERE entity_type_code = %d
		AND entity_id = OLD.id;
	END
`, name, e.code(), e.code())
}

func (e entityTypeInstance) parseSelector(selector Selector) (names []string, projects []string, configKey string, configValues []string, err error) {
	if entity.Type(selector.EntityType) != entity.TypeInstance {
		return nil, nil, "", nil, fmt.Errorf("Invalid selector entity type %q (expected %q)", selector.EntityType, entity.TypeInstance)
	}

	if len(selector.Matchers) == 0 {
		return nil, nil, "", nil, fmt.Errorf("Selector for entity type %q has no matchers", entity.TypeInstance)
	}

	var hasConfigMatcher bool
	existingProperties := make([]string, 0, len(selector.Matchers))
	for _, m := range selector.Matchers {
		if shared.ValueInSlice(m.Property, existingProperties) {
			return nil, nil, "", nil, fmt.Errorf("Repeated selector matcher property %q", m.Property)
		}

		existingProperties = append(existingProperties, m.Property)

		var isConfigMatcher bool
		configKey, isConfigMatcher = strings.CutPrefix(m.Property, "config.")
		if isConfigMatcher {
			if hasConfigMatcher {
				return nil, nil, "", nil, fmt.Errorf("Multiple configuration matchers not supported")
			}

			hasConfigMatcher = true
		}

		if !shared.ValueInSlice(m.Property, []string{"name", "project"}) && !isConfigMatcher {
			return nil, nil, "", nil, fmt.Errorf("Invalid selector property %q for entity type %q", m.Property, entity.TypeInstance)
		}

		if len(m.Values) == 0 {
			return nil, nil, "", nil, fmt.Errorf("Selector matcher with property %q for entity type %q requires at least one value", m.Property, entity.TypeClusterGroup)
		}

		switch m.Property {
		case "name":
			names = m.Values
		case "project":
			projects = m.Values
		default:
			configValues = m.Values
		}
	}

	return names, projects, configKey, configValues, nil
}

func (e entityTypeInstance) runSelector(ctx context.Context, tx *sql.Tx, selector Selector) ([]int, error) {
	names, projects, configKey, configValues, err := e.parseSelector(selector)
	if err != nil {
		return nil, err
	}

	args := make([]any, 0, len(names)+2*(len(projects)+1))
	appendArgs := func(in []string) {
		for _, v := range in {
			args = append(args, v)
		}
	}

	var clauses []string
	var b strings.Builder
	writeLine := func(line string) {
		b.WriteString(line + "\n")
	}

	writeLine(`SELECT instances.id, instances_config.value, 1000000 AS apply_order FROM instances`)
	if len(names) > 0 {
		clauses = append(clauses, "instances.name IN "+query.Params(len(names)))
		appendArgs(names)
	}

	if len(projects) > 0 {
		writeLine(`JOIN projects ON instances.project_id = projects.id`)
		clauses = append(clauses, "projects.name IN "+query.Params(len(projects)))
		appendArgs(projects)
	}

	if len(configValues) > 0 {
		writeLine(`JOIN instances_config ON instances.id = instances_config.instance_id`)
		clauses = append(clauses, "instances_config.key = ?")
		args = append(args, configKey)
	}

	if len(clauses) > 0 {
		writeLine(`WHERE`)
		writeLine(strings.Join(clauses, "\nAND "))
	}

	if len(configValues) > 0 {
		clauses = []string{}
		writeLine("UNION")
		writeLine("SELECT instances.id, profiles_config.value, instances_profiles.apply_order AS apply_order FROM instances")
		if len(projects) > 0 {
			writeLine(`JOIN projects ON instances.project_id = projects.id`)
			clauses = append(clauses, "projects.name IN "+query.Params(len(projects)))
			appendArgs(projects)
		}

		writeLine(`JOIN instances_profiles ON instances.id = instances_profiles.instance_id`)
		writeLine(`JOIN profiles ON instances_profiles.profile_id = profiles.id`)
		writeLine(`JOIN profiles_config ON profiles.id = profiles_config.profile_id`)
		clauses = append(clauses, "profiles_config.key = ?")
		args = append(args, configKey)

		writeLine(`WHERE`)
		writeLine(strings.Join(clauses, "\nAND "))
		writeLine(`ORDER BY apply_order`)
	}

	type precedenceMapValue struct {
		precedence int
		value      string
	}

	instances := make(map[int]precedenceMapValue)
	err = query.Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var instID int
		var configValue string
		var applyOrder int
		err := scan(&instID, &configValue, &applyOrder)
		if err != nil {
			return err
		}

		current, ok := instances[instID]
		if ok && current.precedence > applyOrder {
			return nil
		}

		instances[instID] = precedenceMapValue{precedence: applyOrder, value: configValue}
		return nil
	}, args...)
	if err != nil {
		return nil, err
	}

	var candidates []int
	for instanceID, v := range instances {
		if shared.ValueInSlice(v.value, configValues) {
			candidates = append(candidates, instanceID)
		}
	}

	return candidates, nil
}
