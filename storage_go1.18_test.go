//go:build go1.18
// +build go1.18

package sqluct_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {
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
	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})

	for i := 0; i < 100; i++ {
		mockedRows.AddRow(i, 2*i, 3*i)
	}
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	rows, err := sqluct.List[row](ctx, st, qb)
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

func TestGet(t *testing.T) {
	type row struct {
		One   int `db:"one"`
		Two   int `db:"two"`
		Three int `db:"three"`
	}

	db, mock, err := sqlmock.New()
	assert.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	qb := st.SelectStmt("table", row{})
	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})
	mockedRows.AddRow(1, 2, 3)
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	item, err := sqluct.Get[row](ctx, st, qb)
	assert.NoError(t, err)

	assert.Equal(t, row{One: 1, Two: 2, Three: 3}, item)
}
