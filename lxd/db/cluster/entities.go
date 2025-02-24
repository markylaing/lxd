package cluster

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/canonical/lxd/lxd/db/types"
	"net/http"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"
)

// EntityRef represents the expected format of entity URL queries.
type EntityRef struct {
	EntityType  types.EntityType
	EntityID    int
	ProjectName string
	Location    string
	PathArgs    []string
}

// scan accepts a scanning function (e.g. `(*sql.Row).Scan`) and uses it to parse the row and set its fields.
func (e *EntityRef) scan(scan func(dest ...any) error) error {
	var pathArgs string
	err := scan(&e.EntityType, &e.EntityID, &e.ProjectName, &e.Location, &pathArgs)
	if err != nil {
		return fmt.Errorf("Failed to scan entity URL: %w", err)
	}

	err = json.Unmarshal([]byte(pathArgs), &e.PathArgs)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal entity URL path arguments: %w", err)
	}

	return nil
}

// getURL is a convenience for generating a URL from the EntityRef.
func (e *EntityRef) getURL() (*api.URL, error) {
	u, err := entity.Type(e.EntityType).URL(e.ProjectName, e.Location, e.PathArgs...)
	if err != nil {
		return nil, fmt.Errorf("Failed to create entity URL: %w", err)
	}

	return u, nil
}

// GetEntityURL returns the *api.URL of a single entity by its type and ID.
func GetEntityURL(ctx context.Context, tx *sql.Tx, entityType entity.Type, entityID int) (*api.URL, error) {
	if entityType == entity.TypeServer {
		return entity.ServerURL(), nil
	}

	info, ok := types.EntityTypes()[entityType]
	if !ok {
		return nil, fmt.Errorf("Could not get entity URL: Unknown entity type %q", entityType)
	}

	stmt := info.URLByIDQuery()
	if stmt == "" {
		return nil, fmt.Errorf("Could not get entity URL: No statement found for entity type %q", entityType)
	}

	row := tx.QueryRowContext(ctx, stmt, entityID)
	entityRef := &EntityRef{}
	err := entityRef.scan(row.Scan)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("Failed to scan entity URL: %w", err)
	} else if err != nil {
		return nil, api.StatusErrorf(http.StatusNotFound, "No entity found with id `%d` and type %q", entityID, entityType)
	}

	return entityRef.getURL()
}

// GetEntityURLs accepts a project name and a variadic of entity types and returns a map of entity.Type to map of entity ID, to *api.URL.
// This method combines the above queries into a single query using the UNION operator. If no entity types are given, this function will
// return URLs for all entity types. If no project name is given, this function will return URLs for all projects. This may result in
// stupendously large queries, so use with caution!
func GetEntityURLs(ctx context.Context, tx *sql.Tx, projectName string, filteringEntityTypes ...entity.Type) (map[entity.Type]map[int]*api.URL, error) {
	var stmts []string
	var args []any
	result := make(map[entity.Type]map[int]*api.URL)

	// If the server entity type is in the list of entity types, or if we are getting all entity types and
	// not filtering by project, we need to add a server URL to the result. The entity ID of the server entity type is
	// always zero.
	if shared.ValueInSlice(entity.TypeServer, filteringEntityTypes) || (len(filteringEntityTypes) == 0 && projectName == "") {
		result[entity.TypeServer] = map[int]*api.URL{0: entity.ServerURL()}

		// Return early if there are no other entity types in the list (no queries to execute).
		if len(filteringEntityTypes) == 1 {
			return result, nil
		}
	}

	entityTypes := types.EntityTypes()

	// Collate all the statements we need.
	// If the project is not empty, each statement will need an argument for the project name.
	// Additionally, pre-populate the result map as we know the entity types in advance (this is so that we don't have
	// to check and assign on each loop iteration when scanning rows).
	if len(filteringEntityTypes) == 0 && projectName == "" {
		for entityType, info := range entityTypes {
			q := info.AllURLsQuery()
			if q == "" {
				continue
			}

			stmts = append(stmts, q)
			result[entityType] = make(map[int]*api.URL)
		}
	} else if len(filteringEntityTypes) == 0 && projectName != "" {
		for entityType, info := range entityTypes {
			q := info.URLsByProjectQuery()
			if q == "" {
				continue
			}

			stmts = append(stmts, q)
			args = append(args, projectName)
			result[entityType] = make(map[int]*api.URL)
		}
	} else if projectName == "" {
		for _, entityType := range filteringEntityTypes {
			// We've already added the server url to the result.
			if entityType == entity.TypeServer {
				continue
			}

			info, ok := entityTypes[entityType]
			if !ok {
				return nil, fmt.Errorf("Could not get entity URLs: Unknown entity type %q", entityType)
			}

			q := info.AllURLsQuery()
			if q == "" {
				return nil, fmt.Errorf("Could not get entity URLs: No statement found for entity type %q", entityType)
			}

			stmts = append(stmts, q)
			result[entityType] = make(map[int]*api.URL)
		}
	} else {
		for _, entityType := range filteringEntityTypes {
			// We've already added the server url to the result.
			if entityType == entity.TypeServer {
				continue
			}

			info, ok := entityTypes[entityType]
			if !ok {
				return nil, fmt.Errorf("Could not get entity URLs: Unknown entity type %q", entityType)
			}

			q := info.URLsByProjectQuery()
			if q == "" {
				return nil, fmt.Errorf("Could not get entity URLs: No statement found for entity type %q", entityType)
			}

			stmts = append(stmts, q)
			args = append(args, projectName)
			result[entityType] = make(map[int]*api.URL)
		}
	}

	// Join into a single statement with UNION and query.
	stmt := strings.Join(stmts, " UNION ")
	rows, err := tx.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to perform entity URL query: %w", err)
	}

	for rows.Next() {
		entityRef := &EntityRef{}
		err := entityRef.scan(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("Failed to scan entity URL: %w", err)
		}

		u, err := entityRef.getURL()
		if err != nil {
			return nil, err
		}

		result[entity.Type(entityRef.EntityType)][entityRef.EntityID] = u
	}

	return result, nil
}

// PopulateEntityReferencesFromURLs populates the values in the given map with entity references corresponding to the api.URL keys.
// It will return an error if any of the given URLs do not correspond to a LXD entity.
func PopulateEntityReferencesFromURLs(ctx context.Context, tx *sql.Tx, entityURLMap map[*api.URL]*EntityRef) error {
	// If the input list is empty, nothing to do.
	if len(entityURLMap) == 0 {
		return nil
	}

	entityURLs := make([]*api.URL, 0, len(entityURLMap))
	for entityURL := range entityURLMap {
		entityURLs = append(entityURLs, entityURL)
	}

	entityTypes := types.EntityTypes()

	stmts := make([]string, 0, len(entityURLs))
	var args []any //nolint:prealloc
	for i, entityURL := range entityURLs {
		// Parse the URL to get the majority of the fields of the EntityRef for that URL.
		entityType, projectName, location, pathArgs, err := entity.ParseURL(entityURL.URL)
		if err != nil {
			return fmt.Errorf("Failed to get entity IDs from URLs: %w", err)
		}

		// Populate the result map.
		entityURLMap[entityURL] = &EntityRef{
			EntityType:  types.EntityType(entityType),
			ProjectName: projectName,
			Location:    location,
			PathArgs:    pathArgs,
		}

		// If the given URL is the server url it is valid but there is no need to perform a query for it, the entity
		// ID of the server is always zero (by virtue of being the zero value for int).
		if entityType == entity.TypeServer {
			continue
		}

		info, ok := entityTypes[entityType]
		if !ok {
			return fmt.Errorf("Could not get entity IDs from URLs: Unknown entity type %q", entityType)
		}

		// Get the statement corresponding to the entity type.
		stmt := info.IDFromURLQuery()
		if stmt == "" {
			return fmt.Errorf("Could not get entity IDs from URLs: No statement found for entity type %q", entityType)
		}

		// Each statement accepts an identifier for the query, the project name, the location, and all path arguments as arguments.
		// In this case we can use the index of the url from the argument slice as an identifier.
		stmts = append(stmts, stmt)
		args = append(args, i, projectName, location)
		for _, pathArg := range pathArgs {
			args = append(args, pathArg)
		}
	}

	// If the only argument was a server URL we don't have any statements to execute.
	if len(stmts) == 0 {
		return nil
	}

	// Join the statements with a union and execute.
	stmt := strings.Join(stmts, " UNION ")
	rows, err := tx.QueryContext(ctx, stmt, args...)
	if err != nil {
		return fmt.Errorf("Failed to get entityIDs from URLS: %w", err)
	}

	for rows.Next() {
		var rowID, entityID int
		err = rows.Scan(&rowID, &entityID)
		if err != nil {
			return fmt.Errorf("Failed to get entityIDs from URLS: %w", err)
		}

		if rowID >= len(entityURLs) {
			return fmt.Errorf("Failed to get entityIDs from URLS: Internal error, returned row ID greater than number of URLs")
		}

		// Using the row ID, get the *api.URL from the argument slice, then use it as a key in our result map to get the *EntityRef.
		entityRef, ok := entityURLMap[entityURLs[rowID]]
		if !ok {
			return fmt.Errorf("Failed to get entityIDs from URLS: Internal error, entity URL missing from result object")
		}

		// Set the value of the EntityID in the *EntityRef.
		entityRef.EntityID = entityID
	}

	err = rows.Err()
	if err != nil {
		return fmt.Errorf("Failed to get entity IDs from URLs: %w", err)
	}

	// Check that all given URLs have been resolved to an ID.
	for u, ref := range entityURLMap {
		if ref.EntityID == 0 && ref.EntityType != types.EntityType(entity.TypeServer) {
			return fmt.Errorf("Failed to find entity ID for URL %q", u.String())
		}
	}

	return nil
}

// GetEntityReferenceFromURL gets a single EntityRef by parsing the given api.URL and finding the ID of the entity.
// It is used by the OpenFGA datastore implementation to find permissions for the entity with the given URL.
func GetEntityReferenceFromURL(ctx context.Context, tx *sql.Tx, entityURL *api.URL) (*EntityRef, error) {
	// Parse the URL to get the majority of the fields of the EntityRef for that URL.
	entityType, projectName, location, pathArgs, err := entity.ParseURL(entityURL.URL)
	if err != nil {
		return nil, fmt.Errorf("Failed to get entity ID from URL: %w", err)
	}

	// Populate the fields we know from the URL.
	entityRef := &EntityRef{
		EntityType:  types.EntityType(entityType),
		ProjectName: projectName,
		Location:    location,
		PathArgs:    pathArgs,
	}

	// If the given URL is the server url it is valid but there is no need to perform a query for it, the entity
	// ID of the server is always zero (by virtue of being the zero value for int).
	if entityType == entity.TypeServer {
		return entityRef, nil
	}

	entityTypes := types.EntityTypes()

	info, ok := entityTypes[entityType]
	if !ok {
		return nil, fmt.Errorf("Could not get entity ID from URL: Unknown entity type %q", entityType)
	}

	// Get the statement corresponding to the entity type.
	stmt := info.IDFromURLQuery()
	if stmt == "" {
		return nil, fmt.Errorf("Could not get entity ID from URL: No statement found for entity type %q", entityType)
	}

	// The first bind argument in all entityIDFromURL queries is an index that we use to correspond output of large UNION
	// queries (see PopulateEntityReferencesFromURLs). In this case we are only querying for one ID, so the `0` argument
	// is a placeholder.
	args := []any{0, projectName, location}
	for _, pathArg := range pathArgs {
		args = append(args, pathArg)
	}

	row := tx.QueryRowContext(ctx, stmt, args...)

	var rowID, entityID int
	err = row.Scan(&rowID, &entityID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, api.StatusErrorf(http.StatusNotFound, "No such entity %q", entityURL.String())
		}

		return nil, fmt.Errorf("Failed to get entityID from URL: %w", err)
	}

	entityRef.EntityID = entityID

	return entityRef, nil
}
