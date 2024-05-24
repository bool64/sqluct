package sqluct_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	dumpConnector struct{}
	dumpDriver    struct{}
	dumpConn      struct{}
	dumpStmt      struct{ query string }
)

func (d dumpStmt) Close() error {
	return nil
}

func (d dumpStmt) NumInput() int {
	return -1
}

func (d dumpStmt) Exec(args []driver.Value) (driver.Result, error) {
	fmt.Println("exec", d.query, args)

	return nil, errors.New("skip")
}

func (d dumpStmt) Query(args []driver.Value) (driver.Rows, error) {
	fmt.Println("query", d.query, args)

	return nil, errors.New("skip")
}

func (d dumpConn) Prepare(query string) (driver.Stmt, error) {
	return dumpStmt{query: query}, nil
}

func (d dumpConn) Close() error {
	return nil
}

func (d dumpConn) Begin() (driver.Tx, error) {
	return nil, nil
}

func (d dumpDriver) Open(name string) (driver.Conn, error) {
	fmt.Println("open", name)

	return dumpConn{}, nil
}

func (d dumpConnector) Connect(_ context.Context) (driver.Conn, error) {
	return dumpConn{}, nil
}

func (d dumpConnector) Driver() driver.Driver {
	return dumpDriver{}
}

func TestStorage_InTx_FailToStart(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "a")
	errReceived := false
	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	st.OnError = func(ctx context.Context, err error) {
		errReceived = true

		assert.Equal(t, "a", ctx.Value(ctxKey{}))
		require.EqualError(t, err, "failed to begin tx: begin error")
	}

	mock.ExpectBegin().WillReturnError(errors.New("begin error"))

	err = st.InTx(ctx, nil)

	require.EqualError(t, err, "failed to begin tx: begin error")
	require.NoError(t, mock.ExpectationsWereMet())
	assert.True(t, errReceived)
}

func TestStorage_InTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = st.InTx(context.TODO(), func(_ context.Context) error {
		return nil
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_ReuseTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	dbx := sqlx.NewDb(db, "mock")
	st := sqluct.NewStorage(dbx)

	mock.ExpectBegin()
	// We don't expect COMMIT because it's not the beginner.

	// Manually start a transaction.
	tx, err := dbx.BeginTxx(context.TODO(), nil)
	require.NoError(t, err)

	ctx := sqluct.TxToContext(context.TODO(), tx)
	counter := 0

	// Start using transaction.
	err = st.InTx(ctx, func(_ context.Context) error {
		counter++

		return nil
	})

	assert.Equal(t, 1, counter)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_RollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectRollback()

	// Start using transaction.
	err = st.InTx(context.TODO(), func(_ context.Context) error {
		return errors.New("error")
	})

	require.EqualError(t, err, "error")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_ErrorOnRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("rollback error"))

	// Start using transaction.
	err = st.InTx(context.TODO(), func(_ context.Context) error {
		return errors.New("error")
	})

	require.EqualError(t, err, "failed to rollback: rollback error")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_FailToCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit error"))

	// Start using transaction.
	err = st.InTx(context.TODO(), func(_ context.Context) error {
		return nil
	})

	require.EqualError(t, err, "failed to commit: commit error")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_SelectContext_slice(t *testing.T) {
	type row struct {
		One   int `db:"one"`
		Two   int `db:"two"`
		Three int `db:"three"`
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	traceStarted := false
	traceFinished := false

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))
	st.Trace = func(ctx context.Context, stmt string, args []interface{}) (newCtx context.Context, onFinish func(error)) {
		traceStarted = true

		assert.Equal(t, "SELECT one, two, three FROM table", stmt)
		assert.Empty(t, args)

		return ctx, func(err error) {
			traceFinished = true

			require.NoError(t, err)
		}
	}

	qb := st.QueryBuilder().Select("one", "two", "three").From("table")
	rows := make([]row, 0, 100)
	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})

	for i := 0; i < 100; i++ {
		mockedRows.AddRow(i, 2*i, 3*i)
	}
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	err = st.Select(ctx, qb, &rows)
	require.NoError(t, err)

	i := 0

	for _, item := range rows {
		assert.Equal(t, row{One: i, Two: 2 * i, Three: 3 * i}, item)

		i++
	}

	assert.Equal(t, 100, i)
	assert.True(t, traceStarted)
	assert.True(t, traceFinished)
}

func TestStorage_SelectContext_row(t *testing.T) {
	type row struct {
		One   int `db:"one"`
		Two   int `db:"two"`
		Three int `db:"three"`
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	qb := st.SelectStmt("table", row{})

	var item row

	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})
	mockedRows.AddRow(1, 2, 3)
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	err = st.Select(ctx, qb, &item)
	require.NoError(t, err)

	assert.Equal(t, row{One: 1, Two: 2, Three: 3}, item)
}

func TestStorage_ExecContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	traceStarted := false
	traceFinished := false

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))
	st.Trace = func(ctx context.Context, stmt string, args []interface{}) (newCtx context.Context, onFinish func(error)) {
		traceStarted = true

		assert.Equal(t, "DELETE FROM table", stmt)
		assert.Empty(t, args)

		return ctx, func(err error) {
			traceFinished = true

			require.NoError(t, err)
		}
	}

	mock.ExpectExec("DELETE FROM table").WillReturnResult(sqlmock.NewResult(0, 1))

	qb := st.DeleteStmt("table")

	_, err = st.Exec(context.Background(), qb)
	require.NoError(t, err)
	assert.True(t, traceStarted)
	assert.True(t, traceFinished)
}

func TestStorage_DeleteStmt_backticks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))
	st.Format = squirrel.Dollar
	st.IdentifierQuoter = sqluct.QuoteBackticks

	mock.ExpectExec("DELETE FROM `table`").
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = st.DeleteStmt("table").Exec()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InsertStmt_backticks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))
	st.Format = squirrel.Dollar
	st.IdentifierQuoter = sqluct.QuoteBackticks

	mock.ExpectExec("INSERT INTO `table` \\(`order_id`,`amount`\\) VALUES \\(\\$1,\\$2\\)").
		WithArgs(10, 20).WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = st.InsertStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{10, 20}).Exec()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_UpdateStmt(t *testing.T) {
	st := sqluct.Storage{
		Format: squirrel.Dollar,
	}

	query, args, err := st.UpdateStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{10, 20}).ToSql()
	require.NoError(t, err)
	assert.Equal(t, "UPDATE table SET order_id = $1, amount = $2", query)
	assert.Equal(t, []interface{}{10, 20}, args)
}

func TestStorage_UpdateStmt_ansi(t *testing.T) {
	st := sqluct.Storage{
		Format: squirrel.Dollar,
	}
	st.IdentifierQuoter = sqluct.QuoteANSI

	query, args, err := st.UpdateStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{10, 20}).ToSql()
	require.NoError(t, err)
	assert.Equal(t, `UPDATE "table" SET "order_id" = $1, "amount" = $2`, query)
	assert.Equal(t, []interface{}{10, 20}, args)
}

func TestStorage_SelectStmt(t *testing.T) {
	st := sqluct.NewStorage(nil)

	query, _, err := st.SelectStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{}).ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT order_id, amount FROM table", query)
}

func TestStorage_SelectStmt_backticks(t *testing.T) {
	st := sqluct.NewStorage(nil)
	st.IdentifierQuoter = sqluct.QuoteBackticks

	query, _, err := st.SelectStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{}).ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT `order_id`, `amount` FROM `table`", query)
}

func TestStorage_DB(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)

	dbx := sqlx.NewDb(db, "test")
	st := sqluct.NewStorage(dbx)

	assert.Equal(t, dbx, st.DB())
}

func TestStmt_ToSql(t *testing.T) {
	s, a, err := sqluct.Stmt("SELECT * FROM foo WHERE id=? AND name=?", 1, "bar").ToSql()
	assert.Equal(t, "SELECT * FROM foo WHERE id=? AND name=?", s)
	assert.Equal(t, []interface{}{1, "bar"}, a)
	require.NoError(t, err)
}

func TestPlain_ToSql(t *testing.T) {
	s, a, err := sqluct.Plain("SELECT * FROM foo WHERE id=1 AND name='bar'").ToSql()
	assert.Equal(t, "SELECT * FROM foo WHERE id=1 AND name='bar'", s)
	assert.Nil(t, a)
	require.NoError(t, err)
}
