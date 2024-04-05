package sqluct_test

import (
	"database/sql"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	Sample struct {
		A              int        `db:"a,omitempty"`
		DeeplyEmbedded            // Recursively embedded fields are used as root fields.
		Meta           AnotherRow `db:"meta"` // Meta is a column, but its fields are not.
	}

	DeeplyEmbedded struct {
		SampleEmbedded
		E string `db:"e,omitempty"`
	}

	SampleEmbedded struct {
		B float64 `db:"b"`
		C string  `db:"c"`
	}

	AnotherRow struct {
		SampleEmbedded        // These embedded fields won't show up in Sample statements.
		D              string `db:"d"` // This field won't show up in Sample statements.
	}
)

func TestInsertValue(t *testing.T) {
	z := Sample{
		A: 1,
		DeeplyEmbedded: DeeplyEmbedded{
			SampleEmbedded: SampleEmbedded{
				B: 2.2,
				C: "3",
			},
			E: "e!",
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Insert(ps.Insert("sample"), z)
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,meta,e,b,c) VALUES ($1,$2,$3,$4,$5)", query)
	assert.Equal(t, []interface{}{1, AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""}, "e!", 2.2, "3"}, args)
}

func BenchmarkMapper_Insert_single(b *testing.B) {
	z := Sample{
		A: 1,
		DeeplyEmbedded: DeeplyEmbedded{
			SampleEmbedded: SampleEmbedded{
				B: 2.2,
				C: "3",
			},
			E: "e!",
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := sm.Insert(ps.Insert("sample"), z)

		_, _, err := q.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func TestInsertValue_omitempty(t *testing.T) {
	z := Sample{}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Insert(ps.Insert("sample"), z)
	query, args, err := q.ToSql()
	require.NoError(t, err)
	// a and e are missing for `omitempty`
	assert.Equal(t, "INSERT INTO sample (meta,b,c) VALUES ($1,$2,$3)", query)
	assert.Equal(t, []interface{}{AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""}, 0.0, ""}, args)
}

func BenchmarkMapper_Insert_singleOmitempty(b *testing.B) {
	z := Sample{}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := sm.Insert(ps.Insert("sample"), z)

		_, _, err := q.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func TestInsertValue_IgnoreOmitEmpty(t *testing.T) {
	z := Sample{}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Insert(ps.Insert("sample"), z, sqluct.IgnoreOmitEmpty)
	query, args, err := q.ToSql()
	require.NoError(t, err)
	// a and e are missing for `omitempty`
	assert.Equal(t, "INSERT INTO sample (a,meta,e,b,c) VALUES ($1,$2,$3,$4,$5)", query)
	assert.Equal(t, []interface{}{0, AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""}, "", 0.0, ""}, args)
}

func TestMapper_Insert_nil(t *testing.T) {
	sm := sqluct.Mapper{}
	q := squirrel.Insert("sample")
	assert.Equal(t, q, sm.Insert(q, nil))
}

func TestMapper_Update_nil(t *testing.T) {
	sm := sqluct.Mapper{}
	q := squirrel.Update("sample")
	assert.Equal(t, q, sm.Update(q, nil))
}

func TestMapper_Select_nil(t *testing.T) {
	sm := sqluct.Mapper{}
	q := squirrel.Select()
	assert.Equal(t, q, sm.Select(q, nil))
}

func TestInsertValueSlice_heterogeneous(t *testing.T) {
	z := []Sample{
		{
			A: 0,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 2.2,
					C: "3",
				},
				E: "e!",
			},
		},
		{
			A: 4,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 5.5,
					C: "6",
				},
				E: "ee!",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := ps.Insert("sample")
	assert.Equal(t, q, sm.Insert(q, nil))
	q = sm.Insert(q, z)
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,meta,e,b,c) VALUES ($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)", query)
	assert.Equal(t, []interface{}{
		0,
		AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""},
		"e!", 2.2, "3",
		4,
		AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""},
		"ee!", 5.5, "6",
	}, args)
}

func TestInsertValueSlice_homogeneous(t *testing.T) {
	z := []Sample{
		{
			A: 1,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 2.2,
					C: "3",
				},
				E: "e!",
			},
		},
		{
			A: 4,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 5.5,
					C: "6",
				},
				E: "ee!",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := ps.Insert("sample")
	assert.Equal(t, q, sm.Insert(q, nil))
	q = sm.Insert(q, z)
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,meta,e,b,c) VALUES ($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)", query)
	assert.Equal(t, []interface{}{
		1,
		AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""},
		"e!", 2.2, "3",
		4,
		AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""},
		"ee!", 5.5, "6",
	}, args)
}

func BenchmarkMapper_Insert_slice_heterogeneous(b *testing.B) {
	z := []Sample{
		{
			A: 0,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 2.2,
					C: "3",
				},
				E: "e!",
			},
		},
		{
			A: 4,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 5.5,
					C: "6",
				},
				E: "ee!",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		q := ps.Insert("sample")
		q = sm.Insert(q, z)

		_, _, err := q.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func BenchmarkMapper_Insert_slice_homogeneous(b *testing.B) {
	z := []Sample{
		{
			A: 1,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 2.2,
					C: "3",
				},
				E: "e!",
			},
		},
		{
			A: 4,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 5.5,
					C: "6",
				},
				E: "ee!",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		q := ps.Insert("sample")
		q = sm.Insert(q, z)

		_, _, err := q.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func TestInsertValueSlicePtr(t *testing.T) {
	z := []Sample{
		{
			A: 1,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 2.2,
					C: "3",
				},
				E: "e!",
			},
		},
		{
			A: 4,
			DeeplyEmbedded: DeeplyEmbedded{
				SampleEmbedded: SampleEmbedded{
					B: 5.5,
					C: "6",
				},
				E: "ee!",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Insert(ps.Insert("sample"), z)
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,meta,e,b,c) VALUES ($1,$2,$3,$4,$5),($6,$7,$8,$9,$10)", query)
	assert.Equal(t, []interface{}{
		1,
		AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""},
		"e!", 2.2, "3",
		4,
		AnotherRow{SampleEmbedded: SampleEmbedded{B: 0, C: ""}, D: ""},
		"ee!", 5.5, "6",
	}, args)
}

func TestMapper_Update(t *testing.T) {
	z := SampleEmbedded{
		B: 2.2,
		C: "3",
	}

	condition := struct {
		A int      `db:"a"`
		B []string `db:"b"`
	}{
		A: 1,
		B: []string{"b1", "b2"},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Update(ps.Update("sample"), z)
	q = q.Where(sm.WhereEq(condition))
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "UPDATE sample SET b = $1, c = $2 WHERE a = $3 AND b IN ($4,$5)", query)
	assert.Equal(t, []interface{}{2.2, "3", 1, "b1", "b2"}, args)
}

func TestMapper_Select_struct(t *testing.T) {
	z := Sample{}

	sm := sqluct.Mapper{}
	q := sm.Select(squirrel.Select(), z)
	q = q.From("sample")

	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT a, meta, e, b, c FROM sample", query)
	assert.Equal(t, []interface{}(nil), args)
}

func BenchmarkMapper_Select_struct(b *testing.B) {
	z := Sample{}
	sm := sqluct.Mapper{}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := sm.Select(squirrel.Select(), z)
		q = q.From("sample")

		_, _, err := q.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func TestMapper_Select_slice(t *testing.T) {
	z := []SampleEmbedded{}

	condition := struct {
		A int      `db:"a"`
		B []string `db:"b"`
	}{
		A: 1,
		B: []string{"b1", "b2"},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Select(ps.Select(), z)
	q = q.Where(sm.WhereEq(condition))
	q = q.From("sample")
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT b, c FROM sample WHERE a = $1 AND b IN ($2,$3)", query)
	assert.Equal(t, []interface{}{1, "b1", "b2"}, args)
}

func TestMapper_WhereEq(t *testing.T) {
	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}

	q := ps.Select().From("sample")

	type CustAlloc struct {
		CustomerID     uint64 `db:"fk_customer"`
		AllocationDate string `db:"created_at"`
	}

	rows := make([]struct {
		Key string `db:"campaign"`
		CustAlloc
		Variation sql.NullString `db:"variation"`
	}, 0)
	filter := struct {
		Keys       []string `db:"campaign"`
		CustomerID uint64   `db:"fk_customer"`
	}{
		CustomerID: 123,
	}

	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))

	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE fk_customer = $1", query)
	assert.Equal(t, []interface{}{uint64(123)}, args)

	filter.Keys = []string{"k1", "k2"}
	q = ps.Select().From("sample")
	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))
	query, args, err = q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE campaign IN ($1,$2) AND fk_customer = $3", query)
	assert.Equal(t, []interface{}{"k1", "k2", uint64(123)}, args)

	filter.CustomerID = 0
	q = ps.Select().From("sample")
	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))
	query, args, err = q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE campaign IN ($1,$2)", query)
	assert.Equal(t, []interface{}{"k1", "k2"}, args)

	filter.Keys = nil
	q = ps.Select().From("sample")
	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))
	query, args, err = q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE (1=1)", query)
	assert.Equal(t, []interface{}(nil), args)
}

func TestMapper_Delete(t *testing.T) {
	condition := struct {
		A int      `db:"a"`
		B []string `db:"b"`
		C int
	}{
		A: 1,
		B: []string{"b1", "b2"},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := ps.Delete("sample").Where(sm.WhereEq(condition, sqluct.SkipZeroValues))
	query, args, err := q.ToSql()
	require.NoError(t, err)
	assert.Equal(t, "DELETE FROM sample WHERE a = $1 AND b IN ($2,$3)", query)
	assert.Equal(t, []interface{}{1, "b1", "b2"}, args)
}

func TestMapper_Order(t *testing.T) {
	sm := sqluct.Mapper{}

	type Entity struct {
		Field1 int `db:"f1"`
		Field2 int `db:"f2"`
	}

	e := &Entity{}

	rf := sqluct.Referencer{Mapper: &sm}
	rf.AddTableAlias(e, "")
	assert.Equal(t, "f1 ASC, f2 ASC", rf.Fmt("%s ASC, %s ASC", &e.Field1, &e.Field2))
}

func TestMapper_FindColumnName(t *testing.T) {
	s := Sample{}
	sm := sqluct.Mapper{}

	for _, tc := range []struct {
		structPtr interface{}
		fieldPtr  interface{}
		tagValue  string
		err       string
	}{
		{&s, &s.A, "a", ""},
		{&s, &s.B, "b", ""},
		{&s, &s.C, "c", ""},
		{nil, nil, "", "structPtr and fieldPtr are required"},
		{&s, 123, "", "unknown field or row or not a pointer"},
		{s, &s.A, "", "can not take address of structure, please pass a pointer"},
	} {
		tc := tc

		t.Run("", func(t *testing.T) {
			tagValue, err := sm.FindColumnName(tc.structPtr, tc.fieldPtr)
			assert.Equal(t, tc.tagValue, tagValue)

			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}

func TestMapper_Col(t *testing.T) {
	s := Sample{}
	sm := sqluct.Mapper{}

	assert.Equal(t, "c", sm.Col(&s, &s.C))
	assert.Panics(t, func() {
		sm.Col(&s, 123)
	})
}

func BenchmarkMapper_Select_ref(b *testing.B) {
	sm := sqluct.Mapper{}

	type OrderData struct {
		Amount int `db:"amount"`
		UserID int `db:"user_id,omitempty"`
	}

	type Order struct {
		ID int `db:"id"`
		OrderData
	}

	type User struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	rf := sqluct.Referencer{}
	o := &Order{}
	u := &User{}

	rf.AddTableAlias(o, "orders")
	rf.AddTableAlias(u, "users")

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		q := sm.
			Select(squirrel.Select().From(rf.Ref(o)), o, rf.ColumnsOf(o)).
			Join(rf.Fmt("%s ON %s = %s", u, &o.UserID, &u.ID)).
			Where(sm.WhereEq(OrderData{
				Amount: 100,
				UserID: 123,
			}, rf.ColumnsOf(o)))

		_, _, err := q.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func TestInsertIgnore(t *testing.T) {
	s := sqluct.Storage{}

	assert.Panics(t, func() {
		s.InsertStmt("table", Sample{}, sqluct.InsertIgnore)
	})

	s.Mapper = &sqluct.Mapper{}
	s.Mapper.Dialect = sqluct.DialectMySQL
	s.Format = squirrel.Question
	assertStatement(t, "INSERT IGNORE INTO table (meta,b,c) VALUES (?,?,?)", s.InsertStmt("table", Sample{}, sqluct.InsertIgnore))

	s.Mapper.Dialect = sqluct.DialectSQLite3
	s.Format = squirrel.Question
	assertStatement(t, "INSERT OR IGNORE INTO table (meta,b,c) VALUES (?,?,?)", s.InsertStmt("table", Sample{}, sqluct.InsertIgnore))

	s.Mapper.Dialect = sqluct.DialectPostgres
	s.Format = squirrel.Dollar
	assertStatement(t, "INSERT INTO table (meta,b,c) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING", s.InsertStmt("table", Sample{}, sqluct.InsertIgnore))
}

func assertStatement(t *testing.T, s string, qb sqluct.ToSQL) {
	t.Helper()

	stmt, _, err := qb.ToSql()
	require.NoError(t, err)
	assert.Equal(t, s, stmt)
}
