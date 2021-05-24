package sqluct

import (
	"context"
	"database/sql"
	"errors"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/ctxd"
	"github.com/jmoiron/sqlx"
)

// ToSQL defines query builder.
type ToSQL interface {
	ToSql() (string, []interface{}, error)
}

// StringStatement is a plain string statement.
type StringStatement string

// ToSql implements query builder result.
func (s StringStatement) ToSql() (string, []interface{}, error) { // nolint // Method name matches ext. implementation.
	return string(s), nil, nil
}

// NewStorage creates an instance of Storage.
func NewStorage(db *sqlx.DB) *Storage {
	return &Storage{
		db: db,
	}
}

// Storage creates and executes database statements.
type Storage struct {
	db *sqlx.DB

	Mapper *Mapper

	// Format is a placeholder format, default squirrel.Dollar.
	// Other values are squirrel.Question, squirrel.AtP and squirrel.Colon.
	Format squirrel.PlaceholderFormat

	// IdentifierQuoter is formatter of column and table names.
	// Default QuoteNoop.
	IdentifierQuoter func(tableAndColumn ...string) string

	// OnError is called when error is encountered, could be useful for logging.
	OnError func(ctx context.Context, err error)

	// Trace wraps a call to database.
	// It takes statement as arguments and returns
	// instrumented context with callback to call after db call is finished.
	Trace func(ctx context.Context, stmt string, args []interface{}) (newCtx context.Context, onFinish func(error))
}

// InTx runs callback in a transaction.
//
// If transaction already exists, it will reuse that. Otherwise it starts a new transaction and commit or rollback
// (in case of error) at the end.
func (s *Storage) InTx(ctx context.Context, fn func(context.Context) error) (err error) {
	var finish func(ctx context.Context, err error) error

	if tx := TxFromContext(ctx); tx == nil {
		finish = s.submitTx

		// Start a new transaction.
		tx, err := s.db.BeginTxx(ctx, nil)
		if err != nil {
			return s.error(ctx, ctxd.WrapError(ctx, err, "failed to begin tx"))
		}

		ctx = TxToContext(ctx, tx)
	} else {
		// Do nothing because parent tx is still running and
		// this is not the beginner so it can't be the finisher.
		finish = func(ctx context.Context, err error) error {
			return err
		}
	}

	defer func() {
		err = finish(ctx, err)
	}()

	return fn(ctx)
}

func (s *Storage) submitTx(ctx context.Context, err error) error {
	tx := TxFromContext(ctx)
	if tx == nil {
		return s.error(ctx, ctxd.NewError(ctx, "no running transaction"))
	}

	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return s.error(ctx, ctxd.WrapError(ctx, rbErr, "failed to rollback",
				"error", err,
			))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return s.error(ctx, ctxd.WrapError(ctx, err, "failed to commit"))
	}

	return nil
}

// Exec executes query according to query builder.
func (s *Storage) Exec(ctx context.Context, qb ToSQL) (res sql.Result, err error) {
	var execer sqlx.ExecerContext
	if tx := TxFromContext(ctx); tx != nil {
		execer = tx
	} else {
		execer = s.db
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return nil, s.error(ctx, ctxd.WrapError(ctx, err, "failed to build query"))
	}

	if s.Trace != nil {
		ct, def := s.Trace(ctx, query, args)
		ctx = ct

		defer func() { def(err) }()
	}

	res, err = execer.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, s.error(ctx, err)
	}

	return res, nil
}

// Query queries database and returns raw result.
//
// Select is recommended to use instead of Query.
func (s *Storage) Query(ctx context.Context, qb ToSQL) (*sqlx.Rows, error) {
	query, args, err := qb.ToSql()
	if err != nil {
		return nil, s.error(ctx, ctxd.WrapError(ctx, err, "failed to build query"))
	}

	if s.Trace != nil {
		ct, def := s.Trace(ctx, query, args)
		ctx = ct

		defer func() { def(err) }()
	}

	var queryer sqlx.QueryerContext
	if tx := TxFromContext(ctx); tx != nil {
		queryer = tx
	} else {
		queryer = s.db
	}

	rows, err := queryer.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, s.error(ctx, err)
	}

	return rows, nil
}

// Select queries statement of query builder and scans result into destination.
//
// Destination can be a pointer to struct or slice, e.g. `*row` or `*[]row`.
func (s *Storage) Select(ctx context.Context, qb ToSQL, dest interface{}) (err error) {
	query, args, err := qb.ToSql()
	if err != nil {
		return s.error(ctx, ctxd.WrapError(ctx, err, "failed to build query"))
	}

	if s.Trace != nil {
		ct, def := s.Trace(ctx, query, args)
		ctx = ct

		defer func() { def(err) }()
	}

	var queryer sqlx.QueryerContext
	if tx := TxFromContext(ctx); tx != nil {
		queryer = tx
	} else {
		queryer = s.db
	}

	kind := reflect.Indirect(reflect.ValueOf(dest)).Kind()
	if kind == reflect.Slice {
		err = sqlx.SelectContext(ctx, queryer, dest, query, args...)
	} else {
		err = sqlx.GetContext(ctx, queryer, dest, query, args...)
	}

	return s.error(ctx, err)
}

// QueryBuilder returns query builder with placeholder format.
func (s *Storage) QueryBuilder() squirrel.StatementBuilderType {
	format := s.Format

	if format == nil {
		format = squirrel.Dollar
	}

	return squirrel.StatementBuilder.PlaceholderFormat(format).RunWith(s.db)
}

func (s *Storage) options(options []func(*Options)) []func(*Options) {
	if s.IdentifierQuoter != nil {
		options = append(options, func(options *Options) {
			if options.PrepareColumn == nil {
				options.PrepareColumn = func(col string) string {
					return s.IdentifierQuoter(col)
				}
			}
		})
	}

	return options
}

// SelectStmt makes a select query builder.
func (s *Storage) SelectStmt(tableName string, columns interface{}, options ...func(*Options)) squirrel.SelectBuilder {
	if s.IdentifierQuoter != nil {
		tableName = s.IdentifierQuoter(tableName)
	}

	qb := s.QueryBuilder().Select().From(tableName)

	return mapper(s.Mapper).Select(qb, columns, s.options(options)...)
}

// InsertStmt makes an insert query builder.
func (s *Storage) InsertStmt(tableName string, val interface{}, options ...func(*Options)) squirrel.InsertBuilder {
	if s.IdentifierQuoter != nil {
		tableName = s.IdentifierQuoter(tableName)
	}

	qb := s.QueryBuilder().Insert(tableName)

	return mapper(s.Mapper).Insert(qb, val, s.options(options)...)
}

// UpdateStmt makes an update query builder.
func (s *Storage) UpdateStmt(tableName string, val interface{}, options ...func(*Options)) squirrel.UpdateBuilder {
	if s.IdentifierQuoter != nil {
		tableName = s.IdentifierQuoter(tableName)
	}

	qb := s.QueryBuilder().Update(tableName)

	return mapper(s.Mapper).Update(qb, val, s.options(options)...)
}

// DeleteStmt makes a delete query builder.
func (s *Storage) DeleteStmt(tableName string) squirrel.DeleteBuilder {
	if s.IdentifierQuoter != nil {
		tableName = s.IdentifierQuoter(tableName)
	}

	return s.QueryBuilder().Delete(tableName)
}

// Col will try to find column name and will panic on error.
func (s *Storage) Col(structPtr, fieldPtr interface{}) string {
	col := mapper(s.Mapper).Col(structPtr, fieldPtr)
	if s.IdentifierQuoter != nil {
		col = s.IdentifierQuoter(col)
	}

	return col
}

// Ref creates Referencer for query builder.
func (s *Storage) Ref() *Referencer {
	return &Referencer{
		Mapper:           s.Mapper,
		IdentifierQuoter: s.IdentifierQuoter,
	}
}

// WhereEq maps struct values as conditions to squirrel.Eq.
func (s *Storage) WhereEq(conditions interface{}, options ...func(*Options)) squirrel.Eq {
	return mapper(s.Mapper).WhereEq(conditions, s.options(options)...)
}

func (s *Storage) error(ctx context.Context, err error) error {
	if err != nil && !errors.Is(err, sql.ErrNoRows) && s.OnError != nil {
		s.OnError(ctx, err)
	}

	return err
}

func mapper(m *Mapper) *Mapper {
	if m == nil {
		return defaultMapper
	}

	return m
}

// DB returns database instance.
func (s *Storage) DB() *sqlx.DB {
	return s.db
}
