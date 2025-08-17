//go:build go1.18
// +build go1.18

package sqluct

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/Masterminds/squirrel"
)

// SerialID is the name of field tag to indicate integer serial (auto increment) ID of the table.
const SerialID = "serialIdentity"

// Get retrieves a single row from database storage.
func Get[V any](ctx context.Context, s *Storage, qb ToSQL) (V, error) {
	var v V

	err := s.Select(ctx, qb, &v)

	return v, err
}

// List retrieves a collection of rows from database storage.
func List[V any](ctx context.Context, s *Storage, qb ToSQL) ([]V, error) {
	var v []V

	err := s.Select(ctx, qb, &v)

	return v, err
}

// StorageOf is a type-safe facade to work with rows of specific type.
type StorageOf[V any] struct {
	*Referencer
	R         *V
	s         *Storage
	tableName string
	id        string
}

// Table configures and returns StorageOf in a table.
func Table[V any](storage *Storage, tableName string) StorageOf[V] {
	ar := StorageOf[V]{}
	ar.s = storage
	ar.tableName = tableName

	var v V

	ar.R = &v

	tm := mapper(ar.s.Mapper).typeMap(reflect.TypeOf(v))
	for _, fi := range tm.Index {
		if fi.Embedded {
			continue
		}

		if _, ok := fi.Options[SerialID]; ok {
			ar.id = fi.Name

			break
		}
	}

	ar.Referencer = storage.MakeReferencer()
	ar.AddTableAlias(ar.R, tableName)

	return ar
}

// List retrieves a collection of rows from database storage.
func (s *StorageOf[V]) List(ctx context.Context, qb ToSQL) ([]V, error) {
	var v []V

	err := s.s.Select(ctx, qb, &v)

	return v, err
}

// Get retrieves a single row from database storage.
func (s *StorageOf[V]) Get(ctx context.Context, qb ToSQL) (V, error) {
	var v V

	err := s.s.Select(ctx, qb, &v)

	return v, err
}

// SelectStmt creates query statement with table name and row columns.
func (s *StorageOf[V]) SelectStmt(options ...func(*Options)) squirrel.SelectBuilder {
	if len(options) == 0 {
		options = []func(*Options){
			s.ColumnsOf(s.R),
		}
	}

	return s.s.SelectStmt(s.tableName, s.R, options...)
}

// DeleteStmt creates delete statement with table name.
func (s *StorageOf[V]) DeleteStmt() squirrel.DeleteBuilder {
	return s.s.DeleteStmt(s.tableName)
}

// UpdateStmt creates update statement with table name and updated value (can be nil).
func (s *StorageOf[V]) UpdateStmt(value any, options ...func(*Options)) squirrel.UpdateBuilder {
	return s.s.UpdateStmt(s.tableName, value, options...)
}

// InsertRow inserts single row database table.
func (s *StorageOf[V]) InsertRow(ctx context.Context, row V, options ...func(o *Options)) (int64, error) {
	q := s.s.InsertStmt(s.tableName, row, options...)

	if mapper(s.s.Mapper).Dialect == DialectPostgres && s.id != "" {
		q = q.Suffix("RETURNING " + s.id)

		query, args, err := q.ToSql()
		if err != nil {
			return 0, fmt.Errorf("building insert statement: %w", err)
		}

		var id int64

		if err = s.s.db.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
			return 0, fmt.Errorf("insert: %w", err)
		}

		return id, nil
	}

	res, err := s.s.Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("insert: %w", err)
	}

	if s.id == "" {
		return 0, nil
	}

	id, err := res.LastInsertId()
	if err != nil {
		return id, fmt.Errorf("insert last id: %w", err)
	}

	return id, nil
}

// InsertRows inserts multiple rows in database table.
func (s *StorageOf[V]) InsertRows(ctx context.Context, rows []V, options ...func(o *Options)) (sql.Result, error) {
	q := s.s.InsertStmt(s.tableName, rows, options...)

	res, err := s.s.Exec(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	return res, nil
}

// JSON is a generic container to a serialized db column.
type JSON[V any] struct {
	Val V
}

// UnmarshalJSON decodes JSON into container.
func (s *JSON[V]) UnmarshalJSON(bytes []byte) error {
	return json.Unmarshal(bytes, &s.Val)
}

// MarshalJSON encodes container value as JSON.
func (s JSON[V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Val)
}

// Scan decodes json value from a db column.
func (s *JSON[V]) Scan(src any) error {
	if src == nil {
		return nil
	}

	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, &s.Val)
	case string:
		return json.Unmarshal([]byte(v), &s.Val)
	default:
		return fmt.Errorf("unsupported type %T", src) //nolint:goerr113
	}
}

// Value encodes value as json for a db column.
func (s JSON[V]) Value() (driver.Value, error) {
	j, err := json.Marshal(s.Val)

	return string(j), err
}
