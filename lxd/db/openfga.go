//go:build linux && cgo && !agent

package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/entity"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/storage"

	"github.com/canonical/lxd/shared/api"
)

// NewOpenFGAStore returns a new storage.OpenFGADatastore that is backed directly by the dqlite database.
func NewOpenFGAStore(clusterDB *Cluster) storage.OpenFGADatastore {
	return &openfgaStore{
		clusterDB: clusterDB,
	}
}

type tupleRow struct {
	userEntityType entity.Type
	userRef        []string
	userRelation   string
	relation       string
	entityType     entity.Type
	objectRef      []string
	entityID       int
}

func (t *tupleRow) scan(scan func(dest ...any) error) error {
	var userRef, objectRef string
	err := scan(&t.userEntityType, &userRef, &t.userRelation, &t.relation, &t.entityType, &objectRef, &t.entityID)
	if err != nil {
		return err
	}

	if len(userRef) == 1 && userRef[0] == '*' {
		t.userRef = []string{"*"}
	} else {
		err = json.Unmarshal([]byte(userRef), &t.userRef)
		if err != nil {
			return err
		}
	}

	return json.Unmarshal([]byte(objectRef), &t.objectRef)
}

func (t *tupleRow) user() (string, error) {
	if len(t.userRef) == 1 && t.userRef[0] == "*" {
		return strings.Join([]string{entity.Names[t.userEntityType], "*"}, ":"), nil
	}

	userURL, err := t.userEntityType.URL("", "", t.userRef...)
	if err != nil {
		return "", err
	}

	if t.userRelation != "" {
		userURL = strings.Join([]string{userURL, t.userRelation}, "#")
	}

	return strings.Join([]string{entity.Names[t.userEntityType], userURL}, ":"), nil
}

func (t *tupleRow) object() (string, error) {
	projectName := ""
	location := ""
	if t.entityType.RequiresProject() {
		projectName = t.objectRef[0]
		t.objectRef = t.objectRef[1:]
	}

	if t.entityType == entity.TypeStorageVolume || t.entityType == entity.TypeStorageBucket {
		location = t.objectRef[len(t.objectRef)-1]
		t.objectRef = t.objectRef[:len(t.objectRef)-1]
	}

	objectURL, err := t.entityType.URL(projectName, location, t.objectRef...)
	if err != nil {
		return "", err
	}

	return strings.Join([]string{entity.Names[t.entityType], objectURL}, ":"), nil
}

// openfgaStore is an implementation of storage.OpenFGADatastore that reads from the `openfga_tuple_ref` view in the database.
type openfgaStore struct {
	clusterDB *Cluster
	model     *openfgav1.AuthorizationModel
}

var openFGATupleSelect = `SELECT user_entity_type, user_ref, user_relation, relation, entity_type, object_ref, entity_id FROM openfga_tuple_ref`

func (o *openfgaStore) objectRefFromURI(entityURL string) (string, error) {
	entityType, projectName, location, pathArguments, err := entity.URLToType(entityURL)
	if err != nil {
		return "", err
	}

	if entityType.RequiresProject() {
		pathArguments = append([]string{projectName}, pathArguments...)
	}

	if entityType == entity.TypeStorageBucket || entityType == entity.TypeStorageVolume {
		pathArguments = append(pathArguments, location)
	}

	objectRef, err := json.Marshal(pathArguments)
	if err != nil {
		return "", err
	}

	return string(objectRef), nil
}

// Read reads tuples from the `openfga_tuple_ref` view. It applies where predicates according to the given key.
func (o *openfgaStore) Read(ctx context.Context, s string, key *openfgav1.TupleKey) (storage.TupleIterator, error) {
	var builder strings.Builder
	builder.WriteString(openFGATupleSelect)

	var args []any
	obj := key.GetObject()
	if obj != "" {
		entityTypeStr, entityURL, hasURL := strings.Cut(obj, ":")
		entityType, ok := entity.Types[entityTypeStr]
		if !ok {
			return nil, fmt.Errorf("Invalid object type %q", entityTypeStr)
		}

		builder.WriteString(` WHERE entity_type = ?`)
		args = append(args, entityType)
		if hasURL {
			objectRef, err := o.objectRefFromURI(entityURL)
			if err != nil {
				return nil, err
			}

			builder.WriteString(` AND object_ref = ?`)
			args = append(args, objectRef)
		}
	}

	relation := key.GetRelation()
	if relation != "" {
		if len(args) == 0 {
			builder.WriteString(` WHERE`)
		} else {
			builder.WriteString(` AND`)
		}

		builder.WriteString(` relation = ?`)
		args = append(args, relation)
	}

	user := key.GetUser()
	if user != "" {
		userTypeStr, userURL, hasUserURL := strings.Cut(user, ":")
		userEntityType, ok := entity.Types[userTypeStr]
		if !ok {
			return nil, fmt.Errorf("Invalid user type %q", userTypeStr)
		}

		if len(args) == 0 {
			builder.WriteString(` WHERE`)
		} else {
			builder.WriteString(` AND`)
		}

		builder.WriteString(` user_entity_type = ?`)
		args = append(args, userEntityType)
		if hasUserURL && userURL == "*" {
			builder.WriteString(` AND user_ref = '*' AND user_relation = ''`)
		} else if hasUserURL {
			// May have a reference relation.
			parts := strings.Split(userURL, "#")
			userRelation := ""
			if len(parts) == 2 {
				userURL = parts[0]
				userRelation = parts[1]
			}

			userRef, err := o.objectRefFromURI(userURL)
			if err != nil {
				return nil, err
			}

			builder.WriteString(` AND user_ref = ? AND user_relation = ?`)
			args = append(args, userRef, userRelation)
		}
	}

	rows, err := o.clusterDB.DB().QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}

	return tupleIterator{rows: rows}, nil
}

// tupleIterator is an implementation of storage.TupleIterator.
type tupleIterator struct {
	rows *sql.Rows
}

// Next scans the next row and returns it as an openfgav1.Tuple. It returns a storage.ErrIteratorDone when rows.Next
// returns false (as required by the definition of storage.TupleIterator).
func (t tupleIterator) Next(ctx context.Context) (*openfgav1.Tuple, error) {
	if !t.rows.Next() {
		return nil, storage.ErrIteratorDone
	}

	tupleRow := &tupleRow{}
	err := tupleRow.scan(t.rows.Scan)
	if err != nil {
		return nil, err
	}

	user, err := tupleRow.user()
	if err != nil {
		return nil, err
	}

	object, err := tupleRow.object()
	if err != nil {
		return nil, err
	}

	return &openfgav1.Tuple{
		Key: &openfgav1.TupleKey{
			User:     user,
			Relation: tupleRow.relation,
			Object:   object,
		},
	}, nil
}

// Stop closes the rows.
func (t tupleIterator) Stop() {
	_ = t.rows.Close()
}

// ReadPage is not implemented. It is not required for the functionality we need.
func (*openfgaStore) ReadPage(ctx context.Context, store string, tk *openfgav1.TupleKey, opts storage.PaginationOptions) ([]*openfgav1.Tuple, []byte, error) {
	return nil, nil, api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// ReadUserTuple reads a single tuple from the store.
func (o *openfgaStore) ReadUserTuple(ctx context.Context, store string, tk *openfgav1.TupleKey) (*openfgav1.Tuple, error) {
	var builder strings.Builder
	builder.WriteString(openFGATupleSelect)

	var args []any
	obj := tk.GetObject()
	if obj == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide object")
	}

	entityTypeStr, entityURL, ok := strings.Cut(obj, ":")
	if !ok {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Invalid object format %q", obj)
	}

	entityType, ok := entity.Types[entityTypeStr]
	if !ok {
		return nil, fmt.Errorf("Invalid object type %q", entityTypeStr)
	}

	objectRef, err := o.objectRefFromURI(entityURL)
	if err != nil {
		return nil, err
	}

	builder.WriteString(` WHERE entity_type = ? AND object_ref = ?`)
	args = append(args, entityType, objectRef)

	relation := tk.GetRelation()
	if relation == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide relation")
	}

	builder.WriteString(` AND relation = ?`)
	args = append(args, relation)

	user := tk.GetUser()
	if user == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide user")
	}

	userTypeStr, userURL, ok := strings.Cut(user, ":")
	if !ok {
		return nil, fmt.Errorf("Must provide user reference")
	}

	userEntityType, ok := entity.Types[userTypeStr]
	if !ok {
		return nil, fmt.Errorf("Invalid user type %q", userTypeStr)
	}

	builder.WriteString(` AND user_entity_type = ?`)
	args = append(args, userEntityType)

	if userURL == "*" {
		builder.WriteString(` AND user_ref = '*' AND user_relation = ''`)
	} else {
		// May have a reference relation.
		parts := strings.Split(userURL, "#")
		userRelation := ""
		if len(parts) == 2 {
			userURL = parts[0]
			userRelation = parts[1]
		}

		userRef, err := o.objectRefFromURI(userURL)
		if err != nil {
			return nil, err
		}

		builder.WriteString(` AND user_ref = ? AND user_relation = ?`)
		args = append(args, userRef, userRelation)
	}

	row := o.clusterDB.DB().QueryRowContext(ctx, builder.String(), args...)
	if row.Err() != nil {
		if errors.Is(row.Err(), sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, row.Err()
	}

	tuple := &tupleRow{}
	err = tuple.scan(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}

		return nil, err
	}

	user, err = tuple.user()
	if err != nil {
		return nil, err
	}

	object, err := tuple.object()
	if err != nil {
		return nil, err
	}

	return &openfgav1.Tuple{
		Key: &openfgav1.TupleKey{
			User:     user,
			Relation: relation,
			Object:   object,
		},
	}, nil
}

// ReadUsersetTuples is called on check requests. It accounts for things like type-bound public access tuples.
func (o *openfgaStore) ReadUsersetTuples(ctx context.Context, store string, filter storage.ReadUsersetTuplesFilter) (storage.TupleIterator, error) {
	var builder strings.Builder
	builder.WriteString(openFGATupleSelect)

	var args []any
	obj := filter.Object
	if obj != "" {
		entityTypeStr, entityURL, hasURL := strings.Cut(obj, ":")
		entityType, ok := entity.Types[entityTypeStr]
		if !ok {
			return nil, fmt.Errorf("Invalid object type %q", entityTypeStr)
		}

		builder.WriteString(` WHERE entity_type = ?`)
		args = append(args, entityType)
		if hasURL {
			objectRef, err := o.objectRefFromURI(entityURL)
			if err != nil {
				return nil, err
			}

			builder.WriteString(` AND object_ref = ?`)
			args = append(args, objectRef)
		}
	}

	relation := filter.Relation
	if relation != "" {
		if len(args) == 0 {
			builder.WriteString(` WHERE`)
		} else {
			builder.WriteString(` AND`)
		}

		builder.WriteString(` relation = ?`)
		args = append(args, relation)
	}

	if len(filter.AllowedUserTypeRestrictions) > 0 {
		var orConditions []string
		for _, userset := range filter.AllowedUserTypeRestrictions {
			_, ok := userset.RelationOrWildcard.(*openfgav1.RelationReference_Relation)
			if ok {
				orConditions = append(orConditions, `(user_entity_type = ? AND user_relation = ?)`)
				args = append(args, entity.Types[userset.Type], userset.GetRelation())
			}

			_, ok = userset.RelationOrWildcard.(*openfgav1.RelationReference_Wildcard)
			if ok {
				orConditions = append(orConditions, `(user_entity_type = ? AND user_ref = '*')`)
				args = append(args, entity.Types[userset.Type])
			}
		}

		builder.WriteString(` AND`)
		if len(orConditions) == 1 {
			builder.WriteString(` `)
		} else {
			builder.WriteString(` (`)
		}

		builder.WriteString(orConditions[0])
		if len(orConditions) > 1 {
			for _, orCondition := range orConditions[1:] {
				builder.WriteString(` OR `)
				builder.WriteString(orCondition)
			}

			builder.WriteString(`)`)
		}
	}

	rows, err := o.clusterDB.DB().QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}

	return tupleIterator{rows: rows}, nil
}

// ReadStartingWithUser is used when listing user objects.
func (o *openfgaStore) ReadStartingWithUser(ctx context.Context, store string, filter storage.ReadStartingWithUserFilter) (storage.TupleIterator, error) {
	var builder strings.Builder
	builder.WriteString(openFGATupleSelect)

	if filter.ObjectType == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide object type")
	} else if filter.Relation == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide relation")
	}

	builder.WriteString(` WHERE entity_type = ? AND relation = ?`)
	var args []any
	args = append(args, entity.Types[filter.ObjectType], filter.Relation)

	if len(filter.UserFilter) > 0 {
		var orConditions []string
		for _, u := range filter.UserFilter {
			userTypeStr, userURL, ok := strings.Cut(u.GetObject(), ":")
			if !ok {
				return nil, fmt.Errorf("Must provide user reference")
			}

			userEntityType, ok := entity.Types[userTypeStr]
			if !ok {
				return nil, fmt.Errorf("Invalid user type %q", userTypeStr)
			}

			if userURL == "*" {
				orConditions = append(orConditions, `(user_entity_type = ? AND user_ref = '*' AND user_relation = '')`)
				args = append(args, userEntityType)
				continue
			}

			userRef, err := o.objectRefFromURI(userURL)
			if err != nil {
				return nil, err
			}

			orConditions = append(orConditions, `(user_entity_type = ? AND user_ref = ? AND user_relation = ?)`)
			args = append(args, userEntityType, userRef, u.GetRelation())
		}

		builder.WriteString(` AND`)
		if len(orConditions) == 1 {
			builder.WriteString(` `)
		} else {
			builder.WriteString(` (`)
		}

		builder.WriteString(orConditions[0])
		if len(orConditions) > 1 {
			for _, orCondition := range orConditions[1:] {
				builder.WriteString(` OR `)
				builder.WriteString(orCondition)
			}

			builder.WriteString(`)`)
		}
	}

	rows, err := o.clusterDB.DB().QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return nil, err
	}

	return tupleIterator{rows: rows}, nil
}

// Write is not implemented, we should never be performing writes.
func (*openfgaStore) Write(ctx context.Context, store string, d storage.Deletes, w storage.Writes) error {
	return api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// WriteAuthorizationModel sets the model.
func (o *openfgaStore) WriteAuthorizationModel(ctx context.Context, store string, model *openfgav1.AuthorizationModel) error {
	o.model = model
	return nil
}

// ReadAuthorizationModel returns the model that has been set or an error if it hasn't been set.
func (o *openfgaStore) ReadAuthorizationModel(ctx context.Context, store string, id string) (*openfgav1.AuthorizationModel, error) {
	if o.model != nil {
		return o.model, nil
	}

	return nil, fmt.Errorf("Authorization model not set")
}

// ReadAuthorizationModels returns a slice containing our own model or an error if it hasn't been set yet.
func (o *openfgaStore) ReadAuthorizationModels(ctx context.Context, store string, options storage.PaginationOptions) ([]*openfgav1.AuthorizationModel, []byte, error) {
	if o.model != nil {
		return []*openfgav1.AuthorizationModel{o.model}, nil, nil
	}

	return nil, nil, fmt.Errorf("Authorization model not set")
}

// FindLatestAuthorizationModelID is a no-op.
func (*openfgaStore) FindLatestAuthorizationModelID(ctx context.Context, store string) (string, error) {
	return "", nil
}

// MaxTuplesPerWrite returns -1 because we should never be writing to the store.
func (*openfgaStore) MaxTuplesPerWrite() int {
	return -1
}

// MaxTypesPerAuthorizationModel returns the default value. It doesn't matter as long as it's higher than the number of
// types in our built-in model.
func (*openfgaStore) MaxTypesPerAuthorizationModel() int {
	return 100
}

// CreateStore returns a not implemented error, because there is only one store.
func (*openfgaStore) CreateStore(ctx context.Context, store *openfgav1.Store) (*openfgav1.Store, error) {
	return nil, api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// DeleteStore returns a not implemented error, because there is only one store.
func (*openfgaStore) DeleteStore(ctx context.Context, id string) error {
	return api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// GetStore returns a not implemented error, because there is only one store.
func (*openfgaStore) GetStore(ctx context.Context, id string) (*openfgav1.Store, error) {
	return nil, api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// ListStores returns a not implemented error, because there is only one store.
func (*openfgaStore) ListStores(ctx context.Context, paginationOptions storage.PaginationOptions) ([]*openfgav1.Store, []byte, error) {
	return nil, nil, api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// WriteAssertions returns a not implemented error, because we do not need to use the assertions API.
func (*openfgaStore) WriteAssertions(ctx context.Context, store, modelID string, assertions []*openfgav1.Assertion) error {
	return api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// ReadAssertions returns a not implemented error, because we do not need to use the assertions API.
func (*openfgaStore) ReadAssertions(ctx context.Context, store, modelID string) ([]*openfgav1.Assertion, error) {
	return nil, api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// ReadChanges returns a not implemented error, because we do not need to use the read changes API.
func (*openfgaStore) ReadChanges(ctx context.Context, store, objectType string, paginationOptions storage.PaginationOptions, horizonOffset time.Duration) ([]*openfgav1.TupleChange, []byte, error) {
	return nil, nil, api.StatusErrorf(http.StatusNotImplemented, "not implemented")
}

// IsReady returns true.
func (*openfgaStore) IsReady(ctx context.Context) (storage.ReadinessStatus, error) {
	return storage.ReadinessStatus{
		IsReady: true,
	}, nil
}

// Close is a no-op.
func (*openfgaStore) Close() {}
