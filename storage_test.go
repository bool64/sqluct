package sqluct_test

import (
	"context"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage_InTx_FailToStart(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "a")
	errReceived := false
	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	st.OnError = func(ctx context.Context, err error) {
		errReceived = true

		assert.Equal(t, "a", ctx.Value(ctxKey{}))
		assert.EqualError(t, err, "failed to begin tx: begin error")
	}

	mock.ExpectBegin().WillReturnError(fmt.Errorf("begin error"))

	err = st.InTx(ctx, nil)

	assert.EqualError(t, err, "failed to begin tx: begin error")
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.True(t, errReceived)
}

func TestStorage_InTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = st.InTx(context.TODO(), func(ctx context.Context) error {
		return nil
	})

	assert.Nil(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_ReuseTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	dbx := sqlx.NewDb(db, "mock")
	st := sqluct.NewStorage(dbx)

	mock.ExpectBegin()
	// We don't expect COMMIT because it's not the beginner.

	// Manually start a transaction.
	tx, err := dbx.BeginTxx(context.TODO(), nil)
	assert.NoError(t, err)

	ctx := sqluct.TxToContext(context.TODO(), tx)
	counter := 0

	// Start using transaction.
	err = st.InTx(ctx, func(ctx context.Context) error {
		counter++

		return nil
	})

	assert.Equal(t, 1, counter)
	assert.Nil(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_RollbackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectRollback()

	// Start using transaction.
	err = st.InTx(context.TODO(), func(ctx context.Context) error {
		return fmt.Errorf("error")
	})

	assert.EqualError(t, err, "error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_ErrorOnRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(fmt.Errorf("rollback error"))

	// Start using transaction.
	err = st.InTx(context.TODO(), func(ctx context.Context) error {
		return fmt.Errorf("error")
	})

	assert.EqualError(t, err, "failed to rollback: rollback error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_InTx_FailToCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(fmt.Errorf("commit error"))

	// Start using transaction.
	err = st.InTx(context.TODO(), func(ctx context.Context) error {
		return nil
	})

	assert.EqualError(t, err, "failed to commit: commit error")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_SelectContext_slice(t *testing.T) {
	type row struct {
		One   int `db:"one"`
		Two   int `db:"two"`
		Three int `db:"three"`
	}

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	traceStarted := false
	traceFinished := false

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))
	st.Trace = func(ctx context.Context, stmt string, args []interface{}) (newCtx context.Context, onFinish func(error)) {
		traceStarted = true

		assert.Equal(t, "SELECT one, two, three FROM table", stmt)
		assert.Empty(t, args)

		return ctx, func(err error) {
			traceFinished = true

			assert.NoError(t, err)
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
	assert.NoError(t, err)

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
	assert.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	qb := st.SelectStmt("table", row{})

	var item row

	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})
	mockedRows.AddRow(1, 2, 3)
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	err = st.Select(ctx, qb, &item)
	assert.NoError(t, err)

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

			assert.NoError(t, err)
		}
	}

	mock.ExpectExec("DELETE FROM table").WillReturnResult(sqlmock.NewResult(0, 1))

	qb := st.DeleteStmt("table")

	_, err = st.Exec(context.Background(), qb)
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
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
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStorage_UpdateStmt(t *testing.T) {
	st := sqluct.Storage{
		Format: squirrel.Dollar,
	}

	query, args, err := st.UpdateStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{10, 20}).ToSql()
	assert.NoError(t, err)
	assert.Equal(t, query, "UPDATE table SET order_id = $1, amount = $2")
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
	assert.NoError(t, err)
	assert.Equal(t, query, `UPDATE "table" SET "order_id" = $1, "amount" = $2`)
	assert.Equal(t, []interface{}{10, 20}, args)
}

func TestStorage_SelectStmt(t *testing.T) {
	st := sqluct.NewStorage(nil)

	query, _, err := st.SelectStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{}).ToSql()
	assert.NoError(t, err)
	assert.Equal(t, query, "SELECT order_id, amount FROM table")
}

func TestStorage_SelectStmt_backticks(t *testing.T) {
	st := sqluct.NewStorage(nil)
	st.IdentifierQuoter = sqluct.QuoteBackticks

	query, _, err := st.SelectStmt("table", struct {
		OrderID int `db:"order_id"`
		Amount  int `db:"amount"`
	}{}).ToSql()
	assert.NoError(t, err)
	assert.Equal(t, query, "SELECT `order_id`, `amount` FROM `table`")
}

func TestStorage_DB(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)

	dbx := sqlx.NewDb(db, "test")
	st := sqluct.NewStorage(dbx)

	assert.Equal(t, dbx, st.DB())
}

func TestStmt_ToSql(t *testing.T) {
	s, a, err := sqluct.Stmt("SELECT * FROM foo WHERE id=? AND name=?", 1, "bar").ToSql()
	assert.Equal(t, "SELECT * FROM foo WHERE id=? AND name=?", s)
	assert.Equal(t, []interface{}{1, "bar"}, a)
	assert.NoError(t, err)
}

func TestPlain_ToSql(t *testing.T) {
	s, a, err := sqluct.Plain("SELECT * FROM foo WHERE id=1 AND name='bar'").ToSql()
	assert.Equal(t, "SELECT * FROM foo WHERE id=1 AND name='bar'", s)
	assert.Nil(t, a)
	assert.NoError(t, err)
}
