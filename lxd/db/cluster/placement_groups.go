package cluster

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/canonical/lxd/lxd/db/query"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
)

// Code generation directives.
//
//go:generate -command mapper lxd-generate db mapper -t placement_groups.mapper.go
//go:generate mapper reset -i -b "//go:build linux && cgo && !agent"
//
//go:generate mapper stmt -e placement_group objects table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-ID table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-Project table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-ClusterGroup table=placement_groups
//go:generate mapper stmt -e placement_group objects-by-Name-and-Project table=placement_groups
//go:generate mapper stmt -e placement_group id table=placement_groups
//go:generate mapper stmt -e placement_group create struct=PlacementGroup table=placement_groups
//go:generate mapper stmt -e placement_group delete-by-Name-and-Project table=placement_groups
//go:generate mapper stmt -e placement_group update struct=PlacementGroup table=placement_groups
//go:generate mapper stmt -e placement_group rename struct=PlacementGroup table=placement_groups
//
//go:generate mapper method -i -e placement_group GetMany
//go:generate mapper method -i -e placement_group GetOne
//go:generate mapper method -i -e placement_group ID struct=PlacementGroup
//go:generate mapper method -i -e placement_group Exists struct=PlacementGroup
//go:generate mapper method -i -e placement_group Create struct=PlacementGroup
//go:generate mapper method -i -e placement_group DeleteOne-by-Name-and-Project
//go:generate mapper method -i -e placement_group Update struct=PlacementGroup
//go:generate mapper method -i -e placement_group Rename struct=PlacementGroup
//go:generate goimports -w placement_groups.mapper.go
//go:generate goimports -w placement_groups.interface.mapper.go

// PlacementGroup is the database representation of an api.PlacementGroup.
type PlacementGroup struct {
	ID           int
	Name         string `db:"primary=yes"`
	Description  string
	Policy       PlacementPolicy
	Scope        PlacementScope
	Rigor        PlacementRigor
	Project      string `db:"primary=yes&join=projects.name"`
	ClusterGroup string `db:"join=cluster_groups.name"`
}

// PlacementGroupFilter contains the fields used to filter placement groups.
type PlacementGroupFilter struct {
	ID           *int
	Name         *string
	Project      *string
	ClusterGroup *string
}

// PlacementPolicy is used to convert an api.PlacementPolicy into an integer in the database.
type PlacementPolicy api.PlacementPolicy

const (
	placementPolicyCodeDistribute int64 = 0
	placementPolicyCodeCompact    int64 = 1
)

// Value implements driver.Valuer for PlacementPolicy.
func (p PlacementPolicy) Value() (driver.Value, error) {
	switch api.PlacementPolicy(p) {
	case api.PlacementPolicyDistribute:
		return placementPolicyCodeDistribute, nil
	case api.PlacementPolicyCompact:
		return placementPolicyCodeCompact, nil
	}

	return -1, api.StatusErrorf(http.StatusBadRequest, "Invalid placement policy %q", p)
}

// Scan implements sql.Scanner for PlacementPolicy.
func (p *PlacementPolicy) Scan(value any) error {
	if value == nil {
		return errors.New("Placement policy cannot be null")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid placement policy %q: %w", value, err)
	}

	policyInt64, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Placement policy should be an integer, got `%v` (%T)", intValue, intValue)
	}

	switch policyInt64 {
	case placementPolicyCodeDistribute:
		*p = PlacementPolicy(api.PlacementPolicyDistribute)
	case placementPolicyCodeCompact:
		*p = PlacementPolicy(api.PlacementPolicyCompact)
	default:
		return fmt.Errorf("Unknown placement policy `%d` found in database", policyInt64)
	}

	return nil
}

// PlacementScope is used to convert an api.PlacementScope into an integer in the database.
type PlacementScope api.PlacementScope

const (
	placementScopeCodeClusterMember    int64 = 0
	placementScopeCodeAvailabilityZone int64 = 1
)

// Value implements driver.Valuer for PlacementScope.
func (p PlacementScope) Value() (driver.Value, error) {
	switch api.PlacementScope(p) {
	case api.PlacementScopeClusterMember:
		return placementScopeCodeClusterMember, nil
	case api.PlacementScopeAvailabilityZone:
		return placementScopeCodeAvailabilityZone, nil
	}

	return -1, api.StatusErrorf(http.StatusBadRequest, "Invalid placement scope %q", p)
}

// Scan implements sql.Scanner for PlacementScope.
func (p *PlacementScope) Scan(value any) error {
	if value == nil {
		return errors.New("Placement scope cannot be null")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid placement scope %q: %w", value, err)
	}

	scopeInt64, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Placement scope should be an integer, got `%v` (%T)", intValue, intValue)
	}

	switch scopeInt64 {
	case placementScopeCodeClusterMember:
		*p = PlacementScope(api.PlacementScopeClusterMember)
	case placementScopeCodeAvailabilityZone:
		*p = PlacementScope(api.PlacementScopeAvailabilityZone)
	default:
		return fmt.Errorf("Unknown placement scope `%d` found in database", scopeInt64)
	}

	return nil
}

// PlacementRigor is used to convert an api.PlacementRigor into an integer in the database.
type PlacementRigor api.PlacementScope

const (
	placementRigorCodeStrict     int64 = 0
	placementRigorCodePermissive int64 = 1
)

// Value implements driver.Valuer for PlacementRigor.
func (p PlacementRigor) Value() (driver.Value, error) {
	switch api.PlacementRigor(p) {
	case api.PlacementRigorStrict:
		return placementRigorCodeStrict, nil
	case api.PlacementRigorPermissive:
		return placementRigorCodePermissive, nil
	}

	return -1, api.StatusErrorf(http.StatusBadRequest, "Invalid placement rigor %q", p)
}

// Scan implements sql.Scanner for PlacementRigor.
func (p *PlacementRigor) Scan(value any) error {
	if value == nil {
		return errors.New("Placement rigor cannot be null")
	}

	intValue, err := driver.Int32.ConvertValue(value)
	if err != nil {
		return fmt.Errorf("Invalid placement rigor %q: %w", value, err)
	}

	scopeInt64, ok := intValue.(int64)
	if !ok {
		return fmt.Errorf("Placement rigor should be an integer, got `%v` (%T)", intValue, intValue)
	}

	switch scopeInt64 {
	case placementRigorCodeStrict:
		*p = PlacementRigor(api.PlacementRigorStrict)
	case placementRigorCodePermissive:
		*p = PlacementRigor(api.PlacementRigorPermissive)
	default:
		return fmt.Errorf("Unknown placement rigor `%d` found in database", scopeInt64)
	}

	return nil
}

// ToAPIBase populates base fields of PlacementGroup into an api.PlacementGroup without querying for any additional data.
// This is so that additional fields can be populated elsewhere when performing bulk queries.
func (p PlacementGroup) ToAPIBase() api.PlacementGroup {
	return api.PlacementGroup{
		Name:         p.Name,
		Description:  p.Description,
		Policy:       api.PlacementPolicy(p.Policy),
		Scope:        api.PlacementScope(p.Scope),
		Rigor:        api.PlacementRigor(p.Rigor),
		Project:      p.Project,
		ClusterGroup: p.ClusterGroup,
	}
}

// ToAPI converts a PlacementGroup to an api.PlacementGroup, querying for extra data as necessary.
func (p *PlacementGroup) ToAPI(ctx context.Context, tx *sql.Tx) (*api.PlacementGroup, error) {
	usedBy, err := GetPlacementGroupUsedBy(ctx, tx, p.Project, p.Name)
	if err != nil {
		return nil, err
	}

	apiPlacementGroup := p.ToAPIBase()
	apiPlacementGroup.UsedBy = usedBy
	return &apiPlacementGroup, nil
}

// GetPlacementGroupUsedBy returns a list of URLs of all instances and profiles that reference the given placement group in their configuration.
func GetPlacementGroupUsedBy(ctx context.Context, tx *sql.Tx, projectName string, placementGroupName string) ([]string, error) {
	q := `SELECT ` + strconv.Itoa(int(entityTypeCodeInstance)) + `, instances.name FROM instances
JOIN instances_config ON instances.id = instances_config.instance_id
JOIN projects ON instances.project_id = projects.id
WHERE instances_config.key = 'placement.group' AND instances_config.value = ? AND projects.name = ?
UNION SELECT ` + strconv.Itoa(int(entityTypeCodeProfile)) + `, profiles.name FROM profiles
JOIN profiles_config ON profiles.id = profiles_config.profile_id
JOIN projects ON profiles.project_id = projects.id
WHERE profiles_config.key = 'placement.group' AND profiles_config.value = ? AND projects.name = ?
`

	var urls []string
	err := query.Scan(ctx, tx, q, func(scan func(dest ...any) error) error {
		var eType EntityType
		var eName string
		err := scan(&eType, &eName)
		if err != nil {
			return err
		}

		switch entity.Type(eType) {
		case entity.TypeInstance:
			urls = append(urls, api.NewURL().Project(projectName).Path("1.0", "instances", eName).String())
		case entity.TypeProfile:
			urls = append(urls, api.NewURL().Project(projectName).Path("1.0", "profiles", eName).String())
		}

		return nil
	}, placementGroupName, projectName, placementGroupName, projectName)
	if err != nil {
		return nil, fmt.Errorf("Failed to find references to placement group %q: %w", placementGroupName, err)
	}

	return urls, nil
}

// GetAllPlacementGroupUsedByURLs returns a map of project name to map of placement group name to a list of URLs of instances and
// profiles that reference the placement group in their configuration. If a project is given, used by URLs will only be returned for placement groups in that project.
func GetAllPlacementGroupUsedByURLs(ctx context.Context, tx *sql.Tx, project *string) (map[string]map[string][]string, error) {
	var b strings.Builder
	var args []any
	b.WriteString(`SELECT ` + strconv.Itoa(int(entityTypeCodeInstance)) + `, instances.name, projects.name, instances_config.value FROM instances
JOIN instances_config ON instances.id = instances_config.instance_id
JOIN projects ON projects.id = instances.project_id
WHERE instances_config.key = 'placement.group'`)
	if project != nil {
		b.WriteString(" AND projects.name = ?\n")
		args = append(args, *project)
	}

	b.WriteString(`UNION SELECT ` + strconv.Itoa(int(entityTypeCodeProfile)) + `, profiles.name, projects.name, profiles_config.value FROM profiles
	JOIN profiles_config ON profiles.id = profiles_config.profile_id
	JOIN projects ON projects.id = profiles.project_id
	WHERE profiles_config.key = 'placement.group'`)
	if project != nil {
		b.WriteString(" AND projects.name = ?")
		args = append(args, *project)
	}

	urlMap := make(map[string]map[string][]string)
	err := query.Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var eType EntityType
		var eName string
		var projectName string
		var placementGroupName string
		err := scan(&eType, &eName, &projectName, &placementGroupName)
		if err != nil {
			return err
		}

		var u string
		switch entity.Type(eType) {
		case entity.TypeInstance:
			u = api.NewURL().Project(projectName).Path("1.0", "instances", eName).String()
		case entity.TypeProfile:
			u = api.NewURL().Project(projectName).Path("1.0", "profiles", eName).String()
		default:
			return errors.New("Unexpected entity type in placement group usage query")
		}

		projectMap, ok := urlMap[projectName]
		if !ok {
			urlMap[projectName] = map[string][]string{
				placementGroupName: {u},
			}

			return nil
		}

		projectMap[placementGroupName] = append(projectMap[placementGroupName], u)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve used by URLs for placement groups: %w", err)
	}

	return urlMap, nil
}

// GetPlacementGroupNames returns a map of project name to slice of placement group names. If a project name is provided,
// only groups in that project are returned. Otherwise, the returned map will contain all projects.
func GetPlacementGroupNames(ctx context.Context, tx *sql.Tx, project *string) (map[string][]string, error) {
	var b strings.Builder
	b.WriteString(`
SELECT
	projects.name,
	placement_groups.name
FROM placement_groups
JOIN projects ON projects.id = placement_groups.project_id
`)

	var args []any
	if project != nil {
		b.WriteString(`WHERE projects.name = ?`)
		args = []any{*project}
	}

	nameMap := make(map[string][]string)
	err := query.Scan(ctx, tx, b.String(), func(scan func(dest ...any) error) error {
		var projectName string
		var rulesetName string
		err := scan(&projectName, &rulesetName)
		if err != nil {
			return err
		}

		nameMap[projectName] = append(nameMap[projectName], rulesetName)
		return nil
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to query placement group names: %w", err)
	}

	return nameMap, nil
}
