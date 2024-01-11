//go:build linux && cgo && !agent

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/shared/entitlement"

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

// openfgaStore is an implementation of storage.OpenFGADatastore that reads from the `openfga_tuple_ref` view in the database.
type openfgaStore struct {
	clusterDB *Cluster
	model     *openfgav1.AuthorizationModel
}

// Read reads tuples from the `openfga_tuple_ref` view. It applies where predicates according to the given key.
func (o *openfgaStore) Read(ctx context.Context, s string, key *openfgav1.TupleKey) (storage.TupleIterator, error) {
	var builder strings.Builder
	builder.WriteString(`SELECT user, relation, object_type, object_ref FROM openfga_tuple_ref`)

	var args []any
	obj := key.GetObject()
	if obj != "" {
		objectType, objectID, ok := strings.Cut(obj, ":")

		builder.WriteString(` WHERE object_type = ?`)
		args = append(args, objectType)
		if ok {
			builder.WriteString(` AND object_ref = ?`)
			args = append(args, objectID)
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
		if len(args) == 0 {
			builder.WriteString(` WHERE`)
		} else {
			builder.WriteString(` AND`)
		}

		builder.WriteString(` user = ?`)
		args = append(args, user)
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

	var user, relation, objectType, objectID string
	err := t.rows.Scan(&user, &relation, &objectType, &objectID)
	if err != nil {
		return nil, err
	}

	return &openfgav1.Tuple{
		Key: &openfgav1.TupleKey{
			User:     user,
			Relation: relation,
			Object:   strings.Join([]string{objectType, objectID}, ":"),
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
	builder.WriteString(`SELECT user, relation, object_type, object_ref FROM openfga_tuple_ref`)

	var args []any
	obj := tk.GetObject()
	if obj == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide object")
	}

	objectType, objectID, ok := strings.Cut(obj, ":")
	if !ok {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Invalid object format %q", obj)
	}

	builder.WriteString(` WHERE object_type = ? AND object_ref = ?`)
	args = append(args, objectType, objectID)

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

	builder.WriteString(` AND user = ?`)
	args = append(args, user)

	row := o.clusterDB.DB().QueryRowContext(ctx, builder.String(), args...)
	if row.Err() != nil {
		return nil, row.Err()
	}

	err := row.Scan(&user, &relation, &objectType, &objectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}

		return nil, err
	}

	return &openfgav1.Tuple{
		Key: &openfgav1.TupleKey{
			User:     user,
			Relation: relation,
			Object:   strings.Join([]string{objectType, objectID}, ":"),
		},
	}, nil
}

// ReadUsersetTuples is called on check requests. It accounts for things like type-bound public access tuples.
func (o *openfgaStore) ReadUsersetTuples(ctx context.Context, store string, filter storage.ReadUsersetTuplesFilter) (storage.TupleIterator, error) {
	var builder strings.Builder
	builder.WriteString(`SELECT user, relation, object_type, object_ref FROM openfga_tuple_ref`)

	var args []any
	obj := filter.Object
	if obj != "" {
		objectType, objectID, ok := strings.Cut(obj, ":")

		builder.WriteString(` WHERE object_type = ?`)
		args = append(args, objectType)
		if ok {
			builder.WriteString(` AND object_ref = ?`)
			args = append(args, objectID)
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
				orConditions = append(orConditions, fmt.Sprintf(`user LIKE '%s:%%#%s'`, userset.Type, userset.GetRelation()))
			}

			_, ok = userset.RelationOrWildcard.(*openfgav1.RelationReference_Wildcard)
			if ok {
				orConditions = append(orConditions, `user = ?`)
				args = append(args, fmt.Sprintf("%s:*", userset.Type))
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
	builder.WriteString(`SELECT user, relation, object_type, object_ref FROM openfga_tuple_ref`)

	if filter.ObjectType == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide object type")
	} else if filter.Relation == "" {
		return nil, api.StatusErrorf(http.StatusBadRequest, "Must provide relation")
	}

	builder.WriteString(` WHERE object_type = ? AND relation = ?`)
	var args []any
	args = append(args, filter.ObjectType, filter.Relation)

	if len(filter.UserFilter) > 0 {
		var orConditions []string
		for _, u := range filter.UserFilter {
			targetUser := u.GetObject()
			if u.GetRelation() != "" {
				targetUser = strings.Join([]string{u.GetObject(), u.GetRelation()}, "#")
			}

			orConditions = append(orConditions, fmt.Sprintf("user = '%s'", targetUser))
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

func (c *ClusterTx) GetAuthObjectEntityID(ctx context.Context, object entitlement.Object) (int, error) {
	var id int64
	var err error
	switch object.Type() {
	case entitlement.ObjectTypeGroup:
		id, err = cluster.GetGroupID(ctx, c.Tx(), object.Elements()[0])
	case entitlement.ObjectTypeCertificate:
		id, err = cluster.GetCertificateID(ctx, c.Tx(), object.Elements()[0])
	case entitlement.ObjectTypeStoragePool:
		id, err = c.GetStoragePoolID(ctx, object.Elements()[0])
	case entitlement.ObjectTypeProject:
		id, err = cluster.GetProjectID(ctx, c.Tx(), object.Project())
	case entitlement.ObjectTypeImage:
		project := object.Project()
		idInt, _, err := c.GetImageByFingerprintPrefix(ctx, object.Elements()[0], cluster.ImageFilter{Project: &project})
		if err != nil {
			return 0, err
		}

		id = int64(idInt)
	case entitlement.ObjectTypeImageAlias:
		idInt, _, err := c.GetImageAlias(ctx, object.Project(), object.Elements()[0], true)
		if err != nil {
			return 0, err
		}

		id = int64(idInt)
	case entitlement.ObjectTypeInstance:
		id, err = cluster.GetInstanceID(ctx, c.Tx(), object.Project(), object.Elements()[0])
	case entitlement.ObjectTypeNetwork:
		id, err = c.GetNetworkID(ctx, object.Project(), object.Elements()[0])
	case entitlement.ObjectTypeNetworkACL:
		id, err = c.GetNetworkACLID(ctx, object.Project(), object.Elements()[0])
	case entitlement.ObjectTypeNetworkZone:
		id, err = c.GetNetworkZoneID(ctx, object.Elements()[0])
	case entitlement.ObjectTypeProfile:
		id, err = cluster.GetProfileID(ctx, c.Tx(), object.Project(), object.Elements()[0])
	case entitlement.ObjectTypeStorageBucket:
		elements := object.Elements()
		var node string
		if len(elements) == 3 {
			node = elements[2]
		}

		id, err = c.GetStoragePoolBucketID(ctx, object.Elements()[1], object.Elements()[0], object.Project(), node)
	case entitlement.ObjectTypeStorageVolume:
		elements := object.Elements()
		var node string
		if len(elements) == 4 {
			node = elements[3]
		}

		var volumeType int
		volumeType, err = VolumeTypeNameToDBType(object.Elements()[1])
		if err != nil {
			return 0, err
		}

		id, err = c.GetStoragePoolVolumeID(ctx, object.Elements()[0], volumeType, object.Elements()[2], object.Project(), node)
	default:
		return 0, api.StatusErrorf(http.StatusBadRequest, "Objects of type %q do not have an entity ID", object.Type())
	}

	if err != nil {
		return 0, err
	}

	return int(id), nil
}
