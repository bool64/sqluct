//go:build go1.18
// +build go1.18

package sqluct

import (
	"context"
	"database/sql"
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
	ar.Referencer.AddTableAlias(ar.R, tableName)

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
	return s.s.SelectStmt(s.tableName, s.R, options...)
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
