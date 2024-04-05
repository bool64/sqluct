//go:build go1.18
// +build go1.18

package sqluct_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
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
	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})

	for i := 0; i < 100; i++ {
		mockedRows.AddRow(i, 2*i, 3*i)
	}
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	rows, err := sqluct.List[row](ctx, st, qb)
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

func TestGet(t *testing.T) {
	type row struct {
		One   int `db:"one"`
		Two   int `db:"two"`
		Three int `db:"three"`
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	qb := st.SelectStmt("table", row{})
	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three"})
	mockedRows.AddRow(1, 2, 3)
	mock.ExpectQuery("SELECT one, two, three FROM table").WillReturnRows(mockedRows)

	item, err := sqluct.Get[row](ctx, st, qb)
	require.NoError(t, err)

	assert.Equal(t, row{One: 1, Two: 2, Three: 3}, item)
}

func TestJSON_Value(t *testing.T) {
	type nested struct {
		A int  `json:"a"`
		B bool `json:"b"`
	}

	type row struct {
		One   int                 `db:"one" json:"one"`
		Two   int                 `db:"two" json:"two"`
		Three int                 `db:"three" json:"three"`
		Four  sqluct.JSON[nested] `db:"four" json:"four"`
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	st := sqluct.NewStorage(sqlx.NewDb(db, "mock"))

	qb := st.SelectStmt("table", row{})
	ctx := context.Background()

	mockedRows := sqlmock.NewRows([]string{"one", "two", "three", "four"})
	mockedRows.AddRow(1, 2, 3, `{"a":123,"b":true}`)
	mock.ExpectQuery("SELECT one, two, three, four FROM table").WillReturnRows(mockedRows)

	item, err := sqluct.Get[row](ctx, st, qb)
	require.NoError(t, err)

	expected := row{One: 1, Two: 2, Three: 3}
	expected.Four.Val = nested{A: 123, B: true}
	assert.Equal(t, expected, item)

	j, err := json.Marshal(item)
	require.NoError(t, err)

	assert.Equal(t, `{"one":1,"two":2,"three":3,"four":{"a":123,"b":true}}`, string(j))

	var r row

	require.NoError(t, json.Unmarshal(j, &r))
	assert.Equal(t, expected, r)

	mock.ExpectExec("INSERT INTO table \\(one,two,three,four\\) VALUES \\(\\$1,\\$2\\,\\$3,\\$4\\)").
		WithArgs(1, 2, 3, `{"a":123,"b":true}`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = st.InsertStmt("table", r).ExecContext(ctx)
	require.NoError(t, err)
}
