package query

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/canonical/lxd/shared/api"
)

// APINamer is used by the generic functions below to return API friendly error messages.
type APINamer interface {
	APIName() string
}

// APIPluralNamer can be optionally implemented by types implementing [APINamer] to fully specify the pluralised form.
// Otherwise, all names will be pluralised with an (s) suffix.
// This is used for e.g. Identity -> Identities (rather than Identitys).
type APIPluralNamer interface {
	APIPluralName() string
}

// TableNamer is a type that reports the table that it lives in.
type TableNamer interface {
	TableName() string
}

// Valuer defines the values used at create time for the implementing type.
// The primary key should not be included, as it is expected that this is an auto-incrementing integer value to be
// returned by the database.
type Valuer interface {
	Values(columns ...string) ([]any, error)
}

// Creatable is a type that knows how to create itself.
type Creatable interface {
	Referenceable
	DefaultCreateColumns() []string
}

// Referenceable is a type that knows how to reference itself in the database.
// We need the table name and the columns and values of the primary key.
type Referenceable interface {
	APINamer
	TableNamer
	Valuer
	PrimaryKeyColumns() []string
}

// Updatable defines a type that knows how to update itself.
type Updatable interface {
	Referenceable
	DefaultUpdateColumns() []string
}

// Selectable specifies how to query for all the basic data for a given type.
// The order of the columns in SelectColumns should match the order from [ScanArger].
type Selectable interface {
	APINamer
	TableNamer
	DefaultSelectColumns() []string
	Joins() []string
}

// ScanArger specifies how a type should advertise arguments for scanning rows.
// It must be implemented on a pointer to a [Selectable], and the order of the arguments must
// match the order of columns from [Selectable.SelectColumns].
type ScanArger interface {
	ScanArgs(columns ...string) ([]any, error)
}

// notFoundErr returns a 404 Not Found error for the given type.
func notFoundErr(t APINamer) error {
	return api.NewStatusError(http.StatusNotFound, t.APIName()+" not found")
}

// conflictErr returns a 409 Conflict error for the given type.
func conflictErr(t APINamer) error {
	return api.NewStatusError(http.StatusConflict, t.APIName()+" already exists")
}

// plural returns the pluralised form of the [APINamer].
func plural(t APINamer) string {
	pluralNamer, ok := any(t).(APIPluralNamer)
	if ok {
		return pluralNamer.APIPluralName()
	}

	return t.APIName() + "s"
}

func createOneStmt(c Creatable, columns []string, replace bool) (string, []any, error) {
	if len(columns) == 0 {
		return "", nil, errors.New("No columns specified")
	}

	values, err := c.Values(columns...)
	if err != nil {
		return "", nil, err
	}

	var b strings.Builder
	b.WriteString("INSERT ")
	if replace {
		b.WriteString("OR REPLACE ")
	}

	b.WriteString("INTO ")
	b.WriteString(c.TableName())
	b.WriteString(" (")
	b.WriteString(columns[0])
	for _, column := range columns[1:] {
		b.WriteString(",")
		b.WriteString(column)
	}

	b.WriteString(") VALUES ")
	b.WriteString(Params(len(values)))

	return b.String(), values, nil
}

func create(ctx context.Context, tx *sql.Tx, c Creatable, columns []string, replace bool) (int64, error) {
	stmt, values, err := createOneStmt(c, columns, replace)
	if err != nil {
		return -1, fmt.Errorf("Failed creating %s: %w", strings.ToLower(c.APIName()), err)
	}

	res, err := tx.ExecContext(ctx, stmt, values...)
	if err != nil {
		if IsConflictErr(err) {
			return -1, conflictErr(c)
		}

		return -1, fmt.Errorf("Failed creating %s: %w", strings.ToLower(c.APIName()), err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("Failed getting ID of new %s: %w", strings.ToLower(c.APIName()), err)
	}

	return id, nil
}

// Create creates a [Creatable]. All columns are set except for the primary key.
// This is because it is assumed that the primary key is auto-assigned at the database layer.
func Create(ctx context.Context, tx *sql.Tx, c Creatable) (int64, error) {
	return create(ctx, tx, c, c.DefaultCreateColumns(), false)
}

// CreateOrReplace creates or replaces a [Creatable]. All columns are set except for the primary key.
// This is because it is assumed that the primary key is auto-assigned at the database layer.
func CreateOrReplace(ctx context.Context, tx *sql.Tx, c Creatable) (int64, error) {
	return create(ctx, tx, c, c.DefaultCreateColumns(), true)
}

func CreateWithColumns(ctx context.Context, tx *sql.Tx, c Creatable, columns []string) (int64, error) {
	return create(ctx, tx, c, columns, false)
}

func CreateOrReplaceWithColumns(ctx context.Context, tx *sql.Tx, c Creatable, columns []string) (int64, error) {
	return create(ctx, tx, c, columns, true)
}

// CreateMany creates multiple [Creatable] entities in a single multi-row INSERT statement.
// If the slice is empty, it returns nil. If there is only one element, it delegates to [Create].
func CreateMany[T Creatable, _ interface {
	Creatable
	*T
}](ctx context.Context, tx *sql.Tx, creatables []T) error {
	var first T
	switch len(creatables) {
	case 0:
		return nil
	case 1:
		_, err := Create(ctx, tx, creatables[0])
		return err
	default:
		first = creatables[0]
	}

	columns := first.DefaultCreateColumns()
	firstStmt, firstValues, err := createOneStmt(first, columns, false)
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(firstStmt)
	args := firstValues
	paramsStr := Params(len(args))
	for _, c := range creatables[1:] {
		values, err := c.Values(columns...)
		if err != nil {
			return fmt.Errorf("Failed creating %s: %w", strings.ToLower(plural(first)), err)
		}

		b.WriteString(", ")
		b.WriteString(paramsStr)
		args = append(args, values...)
	}

	_, err = tx.ExecContext(ctx, b.String(), args...)
	if err != nil {
		if IsConflictErr(err) {
			return conflictErr(first)
		}

		return fmt.Errorf("Failed creating %s: %w", strings.ToLower(plural(first)), err)
	}

	return nil
}

func updateManyStmt(u Updatable, columns []string) (string, []any, error) {
	if len(columns) == 0 {
		return "", nil, errors.New("No columns specified")
	}

	values, err := u.Values(columns...)
	if err != nil {
		return "", nil, err
	}

	var b strings.Builder
	b.WriteString("UPDATE ")
	b.WriteString(u.TableName())
	b.WriteString(" SET ")

	b.WriteString(columns[0])
	b.WriteString(" = ?")
	for _, c := range columns[1:] {
		b.WriteString(", ")
		b.WriteString(c)
		b.WriteString(" = ?")
	}

	return b.String(), values, nil
}

// UpdateByPrimaryKey updates an [Updatable] type by its primary key. It sets all fields except for the primary key.
// The expected usage is to get an [Updatable] entry from the database (e.g. via [SelectOne]), change fields on it
// directly, and then call Update. This pattern is encouraged so that we get an entity, perform an authorization check,
// and then update the values.
func UpdateByPrimaryKey(ctx context.Context, tx *sql.Tx, u Updatable) error {
	return UpdateColumnsByPrimaryKey(ctx, tx, u, u.DefaultUpdateColumns())
}

func UpdateColumnsByPrimaryKey(ctx context.Context, tx *sql.Tx, u Updatable, columns []string) error {
	stmt, values, err := updateManyStmt(u, columns)
	if err != nil {
		return fmt.Errorf("Failed updating %s by primary key: %w", strings.ToLower(u.APIName()), err)
	}

	pkClause, pkValues, err := primaryKeyClause(u)
	if err != nil {
		return fmt.Errorf("Failed updating %s by primary key: %w", strings.ToLower(u.APIName()), err)
	}

	res, err := tx.ExecContext(ctx, stmt+pkClause, append(values, pkValues...)...)
	if err != nil {
		if IsConflictErr(err) {
			return conflictErr(u)
		}

		return fmt.Errorf("Failed updating %s by primary key: %w", strings.ToLower(u.APIName()), err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("Failed verifying %s update: %w", strings.ToLower(u.APIName()), err)
	}

	if n == 0 || n > 1 {
		err = fmt.Errorf("Update by primary key affected %d rows", n)
		if n == 0 {
			if len(pkValues) == 1 {
				switch i := pkValues[0].(type) {
				case int, int64:
					if i == 0 {
						return fmt.Errorf("%w: Integer primary key is unset", err)
					}
				}
			}

			// Don't return a not found error here. If we try to update something by primary key and it is not present,
			// there is an error in our business logic (e.g. we may need to prevent deletion of the resource until an
			// operation/handler is finished).
			return err
		}

		return NewMultipleMatchErr(err)
	}

	return nil
}

// UpdateOne updates a single [Updatable] that matches the given clause. It sets all fields except for the primary key.
// The args variadic should contain only the bind arguments for the given clause.
// Remaining bind arguments are defined by the [Updatable] type.
func UpdateOne(ctx context.Context, tx *sql.Tx, u Updatable, clause string, args ...any) error {
	return UpdateOneWithColumns(ctx, tx, u, u.DefaultUpdateColumns(), clause, args...)
}

func UpdateOneWithColumns(ctx context.Context, tx *sql.Tx, u Updatable, columns []string, clause string, args ...any) error {
	stmt, values, err := updateManyStmt(u, columns)
	if err != nil {
		return fmt.Errorf("Failed updating %s: %w", strings.ToLower(u.APIName()), err)
	}

	var b strings.Builder
	b.WriteString(stmt)
	b.WriteString(" ")
	b.WriteString(clause)

	res, err := tx.ExecContext(ctx, b.String(), append(values, args...)...)
	if err != nil {
		if IsConflictErr(err) {
			return conflictErr(u)
		}

		return fmt.Errorf("Failed updating %s: %w", strings.ToLower(u.APIName()), err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("Failed verifying %s update: %w", strings.ToLower(u.APIName()), err)
	}

	if n < 1 {
		return notFoundErr(u)
	} else if n > 1 {
		return NewMultipleMatchErr(fmt.Errorf("Expected to update one %s but updated %d", strings.ToLower(u.APIName()), n))
	}

	return nil
}

// Select is a wrapper for [SelectFunc] which passes a simple function to append each scanned row to a slice and returns it.
func Select[T Selectable, PT interface {
	ScanArgs(...string) ([]any, error)
	*T
}](ctx context.Context, tx *sql.Tx, clause string, args ...any) ([]T, error) {
	t := *new(T)
	return SelectColumns[T, PT](ctx, tx, clause, t.DefaultSelectColumns(), args...)
}

// SelectColumns is a wrapper for [SelectFunc] which passes a simple function to append each scanned row to a slice and returns it.
func SelectColumns[T Selectable, PT interface {
	ScanArgs(...string) ([]any, error)
	*T
}](ctx context.Context, tx *sql.Tx, clause string, columns []string, args ...any) ([]T, error) {
	var ts []T
	f := func(t T) error {
		ts = append(ts, t)
		return nil
	}

	err := SelectColumnsFunc[T, PT](ctx, tx, clause, columns, f, args...)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

// SelectFunc selects data as defined by the base query of the given type. A function is called for every row scanned.
// Supplementary SQL can be passed in to specify additional joins, clauses, or aggregations.
// The type constraint specifies that the non-pointer type must implement BaseQuerier, but a pointer to the same type
// must implement [ScanArger]. This is important because it means that new(T), a pointer to T, is non-nil.
// It also means that we call ScanArgs on the pointer to the type, so we scan a row directly into each fields' memory.
func SelectFunc[T Selectable, PT interface {
	ScanArgs(...string) ([]any, error)
	*T
}](ctx context.Context, tx *sql.Tx, clause string, rowFunc func(T) error, args ...any) error {
	t := *new(T)
	return SelectColumnsFunc[T, PT](ctx, tx, clause, t.DefaultSelectColumns(), rowFunc, args...)
}

func SelectColumnsFunc[T Selectable, PT interface {
	ScanArgs(...string) ([]any, error)
	*T
}](ctx context.Context, tx *sql.Tx, clause string, columns []string, rowFunc func(T) error, args ...any) error {
	if len(columns) == 0 {
		return errors.New("No columns specified")
	}

	pt := PT(new(T))
	t := *pt
	_, err := pt.ScanArgs(columns...)
	if err != nil {
		return fmt.Errorf("Failed preparing %s query: %w", strings.ToLower(t.APIName()), err)
	}

	var b strings.Builder
	b.WriteString("SELECT ")

	b.WriteString(columns[0])
	for _, column := range columns[1:] {
		b.WriteString(", ")
		b.WriteString(column)
	}

	b.WriteString(" FROM ")
	b.WriteString(t.TableName())
	joins := t.Joins()
	for _, join := range joins {
		b.WriteString(" ")
		b.WriteString(join)
	}

	b.WriteString(" ")
	b.WriteString(clause)

	rows, err := tx.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return fmt.Errorf("Failed loading %s: %w", strings.ToLower(plural(t)), err)
	}

	defer func() { _ = rows.Close() }()
	for rows.Next() {
		pt := PT(new(T))
		scanArgs, err := pt.ScanArgs(columns...)
		if err != nil {
			return fmt.Errorf("Failed reading %s: %w", strings.ToLower(t.APIName()), err)
		}

		err = rows.Scan(scanArgs...)
		if err != nil {
			return fmt.Errorf("Failed reading %s: %w", strings.ToLower(t.APIName()), err)
		}

		err = rowFunc(*pt)
		if err != nil {
			return err
		}
	}

	err = rows.Err()
	if err != nil {
		return fmt.Errorf("Failed loading %s: %w", strings.ToLower(plural(t)), err)
	}

	return nil
}

// SelectOne selects a single row and errors if more than one row is found.
// Supplementary SQL can be passed in to specify additional joins, clauses, or aggregations.
// The type constraint specifies that the non-pointer type must implement BaseQuerier, but a pointer to the same type
// must implement [ScanArger].
func SelectOne[T Selectable, PT interface {
	ScanArgs(...string) ([]any, error)
	*T
}](ctx context.Context, tx *sql.Tx, clause string, args ...any) (PT, error) {
	var pt PT
	f := func(t T) error {
		if pt != nil {
			return NewMultipleMatchErr(fmt.Errorf("More than one %s found", strings.ToLower(t.APIName())))
		}

		pt = &t
		return nil
	}

	err := SelectFunc[T, PT](ctx, tx, clause, f, args...)
	if err != nil {
		return nil, err
	}

	if pt == nil {
		return nil, notFoundErr(*new(T))
	}

	return pt, nil
}

func primaryKeyClause(r Referenceable) (string, []any, error) {
	pkColumns := r.PrimaryKeyColumns()
	if len(pkColumns) == 0 {
		return "", nil, errors.New("Failed getting primary key clause: No primary key columns specified")
	}

	pkValues, err := r.Values(pkColumns...)
	if err != nil {
		return "", nil, fmt.Errorf("Failed getting primary key clause: %w", err)
	}

	var b strings.Builder
	b.WriteString(" WHERE ")
	b.WriteString(pkColumns[0])
	b.WriteString(" = ?")
	for _, c := range pkColumns[1:] {
		b.WriteString(" AND ")
		b.WriteString(c)
		b.WriteString(" = ?")
	}

	return b.String(), pkValues, nil
}

// DeleteByPrimaryKey deletes a [Referenceable] type by primary key.
// The expected usage is to get a [Referenceable] (e.g. via [SelectOne]) and then call Delete on it.
// Note that this function does not call DeleteOne. This is because the Referenceable argument might
// not satisfy the type constraint.
func DeleteByPrimaryKey[T Referenceable](ctx context.Context, tx *sql.Tx, t T) error {
	tableName := t.TableName()
	clause, pkValues, err := primaryKeyClause(t)
	if err != nil {
		return fmt.Errorf("Failed deleting %s: %w", strings.ToLower(t.APIName()), err)
	}

	var b strings.Builder
	b.WriteString("DELETE FROM ")
	b.WriteString(tableName)
	b.WriteString(clause)

	result, err := tx.ExecContext(ctx, b.String(), pkValues...)
	if err != nil {
		return fmt.Errorf("Failed deleting %s: %w", strings.ToLower(t.APIName()), err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Failed getting number of deleted rows: %w", err)
	}

	if deleted == 0 || deleted > 1 {
		err = fmt.Errorf("Deletion by primary key affected %d rows", deleted)
		if deleted == 0 {
			// Don't return a not found error here. If we try to delete something by primary key and it is not present,
			// there is an error in our business logic (e.g. we may need a lock).
			return err
		}

		return NewMultipleMatchErr(err)
	}

	return nil
}

// DeleteMany deletes all rows from the table described by the given [TableNamer] that match the given clause.
// The type constraint enforces that both the type and a pointer to the type implement the [TableNamer] interface.
// This is the only way to enforce that (*new(T)) is a non-pointer type, and therefore non-nil.
func DeleteMany[T Referenceable, _ interface {
	Referenceable
	*T
}](ctx context.Context, tx *sql.Tx, clause string, args ...any) (int64, error) {
	r := *new(T)

	var b strings.Builder
	b.WriteString("DELETE FROM ")
	b.WriteString(r.TableName())
	b.WriteString(" ")
	b.WriteString(clause)

	result, err := tx.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return -1, fmt.Errorf("Failed deleting %s: %w", strings.ToLower(r.APIName()), err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return -1, fmt.Errorf("Failed getting number of deleted rows: %w", err)
	}

	return n, nil
}

// DeleteOne deletes all rows from the table described by the given [TableNamer] that match the given clause.
// If the number of affected rows is not one, an error is returned (and the transaction should be rolled back).
// The type constraint enforces that both the type and a pointer to the type implement the [TableNamer] interface.
// This is the only way to enforce that (*new(T)) is a non-pointer type, and therefore non-nil.
func DeleteOne[T Referenceable, PT interface {
	Referenceable
	*T
}](ctx context.Context, tx *sql.Tx, clause string, args ...any) error {
	n, err := DeleteMany[T, PT](ctx, tx, clause, args...)
	if err != nil {
		return err
	}

	r := *new(T)
	if n < 1 {
		return notFoundErr(r)
	} else if n > 1 {
		return NewMultipleMatchErr(fmt.Errorf("Expected to delete one %s but deleted %d", strings.ToLower(r.APIName()), n))
	}

	return nil
}
