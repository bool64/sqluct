package sqluct_test

import (
	"database/sql"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/stretchr/testify/assert"
)

type (
	Sample struct {
		A int `db:"a"`
		SampleEmbedded
	}

	SampleEmbedded struct {
		B float64 `db:"b"`
		C string  `db:"c"`
	}
)

func TestInsertValue(t *testing.T) {
	z := Sample{
		A: 1,
		SampleEmbedded: SampleEmbedded{
			B: 2.2,
			C: "3",
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Insert(ps.Insert("sample"), z)
	query, args, err := q.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,b,c) VALUES ($1,$2,$3)", query)
	assert.Equal(t, []interface{}{1, 2.2, "3"}, args)
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

func TestInsertValueSlice(t *testing.T) {
	z := []Sample{
		{
			A: 1,
			SampleEmbedded: SampleEmbedded{
				B: 2.2,
				C: "3",
			},
		},
		{
			A: 4,
			SampleEmbedded: SampleEmbedded{
				B: 5.5,
				C: "6",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := ps.Insert("sample")
	assert.Equal(t, q, sm.Insert(q, nil))
	q = sm.Insert(q, z)
	query, args, err := q.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,b,c) VALUES ($1,$2,$3),($4,$5,$6)", query)
	assert.Equal(t, []interface{}{1, 2.2, "3", 4, 5.5, "6"}, args)
}

func TestInsertValueSlicePtr(t *testing.T) {
	z := []Sample{
		{
			A: 1,
			SampleEmbedded: SampleEmbedded{
				B: 2.2,
				C: "3",
			},
		},
		{
			A: 4,
			SampleEmbedded: SampleEmbedded{
				B: 5.5,
				C: "6",
			},
		},
	}

	ps := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sm := sqluct.Mapper{}
	q := sm.Insert(ps.Insert("sample"), z)
	query, args, err := q.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "INSERT INTO sample (a,b,c) VALUES ($1,$2,$3),($4,$5,$6)", query)
	assert.Equal(t, []interface{}{1, 2.2, "3", 4, 5.5, "6"}, args)
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
	assert.NoError(t, err)
	assert.Equal(t, "UPDATE sample SET b = $1, c = $2 WHERE a = $3 AND b IN ($4,$5)", query)
	assert.Equal(t, []interface{}{2.2, "3", 1, "b1", "b2"}, args)
}

func TestMapper_Select_struct(t *testing.T) {
	z := SampleEmbedded{}

	sm := sqluct.Mapper{}
	q := sm.Select(squirrel.Select(), z)
	q = q.From("sample")

	query, args, err := q.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "SELECT b, c FROM sample", query)
	assert.Equal(t, []interface{}(nil), args)
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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE fk_customer = $1", query)
	assert.Equal(t, []interface{}{uint64(123)}, args)

	filter.Keys = []string{"k1", "k2"}
	q = ps.Select().From("sample")
	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))
	query, args, err = q.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE campaign IN ($1,$2) AND fk_customer = $3", query)
	assert.Equal(t, []interface{}{"k1", "k2", uint64(123)}, args)

	filter.CustomerID = 0
	q = ps.Select().From("sample")
	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))
	query, args, err = q.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, "SELECT campaign, variation, fk_customer, created_at FROM sample WHERE campaign IN ($1,$2)", query)
	assert.Equal(t, []interface{}{"k1", "k2"}, args)

	filter.Keys = nil
	q = ps.Select().From("sample")
	q = sm.Select(q, rows)
	q = q.Where(sm.WhereEq(filter, sqluct.SkipZeroValues))
	query, args, err = q.ToSql()
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	assert.Equal(t, "DELETE FROM sample WHERE a = $1 AND b IN ($2,$3)", query)
	assert.Equal(t, []interface{}{1, "b1", "b2"}, args)
}

func TestMapper_Order(t *testing.T) {
	sm := sqluct.Mapper{}

	type Entity struct {
		Field1 int `db:"f1"`
		Field2 int `db:"f2"`
	}

	assert.Equal(t, "f1 ASC, f2 ASC", sm.Order(Entity{}))
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
		{&s, 123, "", "could not find field value in struct"},
		{s, &s.A, "", "can not take address of structure, please pass a pointer"},
	} {
		tc := tc

		t.Run("", func(t *testing.T) {
			tagValue, err := sm.FindColumnName(tc.structPtr, tc.fieldPtr)
			assert.Equal(t, tc.tagValue, tagValue)
			if tc.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.err)
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
