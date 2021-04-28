package sqluct_test

import (
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/stretchr/testify/assert"
)

func TestReferencer_Fmt(t *testing.T) {
	rf := sqluct.Referencer{}

	type User struct {
		ID        int    `db:"id,omitempty"`
		FirstName string `db:"first_name,omitempty"`
		LastName  string `db:"last_name,omitempty"`
	}

	type DirectReport struct {
		ManagerID  int `db:"manager_id"`
		EmployeeID int `db:"employee_id"`
	}

	manager := &User{}
	rf.AddTableAlias(manager, "manager")

	employee := &User{}
	rf.AddTableAlias(employee, "employee")

	dr := &DirectReport{}
	rf.AddTableAlias(dr, "dr")

	m := sqluct.Mapper{}

	// Find direct reports that share same last name and manager is not named John.
	qb := squirrel.StatementBuilder.Select(rf.Fmt("%s, %s", &dr.ManagerID, &dr.EmployeeID)).
		From(rf.Fmt("%s AS %s", rf.Q("users"), manager)).
		InnerJoin(rf.Fmt("%s AS %s ON %s = %s AND %s = %s",
			rf.Q("direct_reports"), dr,
			&dr.ManagerID, &manager.ID,
			&dr.EmployeeID, &employee.ID)).
		Where(rf.Fmt("%s = %s", &manager.LastName, &employee.LastName)).
		Where(rf.Fmt("%s != ?", &manager.FirstName), "John").
		Where(m.WhereEq(User{FirstName: "Larry", LastName: "Page"}, rf.ColumnsOf(employee))).
		Where(squirrel.NotEq(m.WhereEq(User{FirstName: "Sergey", LastName: "Brin"}, rf.ColumnsOf("manager"))))

	stmt, args, err := qb.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, `SELECT dr.manager_id, dr.employee_id `+
		`FROM users AS manager `+
		`INNER JOIN direct_reports AS dr ON dr.manager_id = manager.id AND dr.employee_id = employee.id `+
		`WHERE manager.last_name = employee.last_name AND manager.first_name != ? `+
		`AND employee.first_name = ? AND employee.last_name = ? `+
		`AND manager.first_name <> ? AND manager.last_name <> ?`, stmt)
	assert.Equal(t, []interface{}{"John", "Larry", "Page", "Sergey", "Brin"}, args)
}

func TestReferencer_Ref(t *testing.T) {
	rf := sqluct.Referencer{}
	rf.IdentifierQuoter = sqluct.QuoteANSI

	row := &struct {
		ID int `db:"id,omitempty"`
	}{}

	row2 := &struct {
		ID int `db:"id,omitempty"`
	}{}

	rf.AddTableAlias(row, "some_table")
	rf.AddTableAlias(row2, "")
	assert.Equal(t, `"some_table"`, rf.Ref(row))
	assert.Equal(t, `"some_table"."id"`, rf.Ref(&row.ID))
	assert.Panics(t, func() {
		rf.Ref(row2)
	})
	assert.Equal(t, `"id"`, rf.Ref(&row2.ID))
	assert.Panics(t, func() {
		rf.Ref(nil)
	})
	assert.Panics(t, func() {
		// Must not be nil.
		rf.AddTableAlias(nil, "some_table")
	})
	assert.Panics(t, func() {
		// Must be a pointer.
		rf.AddTableAlias(*row, "some_table")
	})
}

func TestQuoteNoop(t *testing.T) {
	assert.Equal(t, "one.two", sqluct.QuoteNoop("one", "two"))
	assert.Equal(t, "", sqluct.QuoteNoop())
}

func TestQuoteBackticks(t *testing.T) {
	assert.Equal(t, "`one`.`two`", sqluct.QuoteBackticks("one", "two"))
	assert.Equal(t, "", sqluct.QuoteBackticks())
	assert.Equal(t, "`spacy id`.`back``ticky`.`quo\"ty`", sqluct.QuoteBackticks("spacy id", "back`ticky", `quo"ty`))
}

func TestQuoteANSI(t *testing.T) {
	assert.Equal(t, `"one"."two"`, sqluct.QuoteANSI("one", "two"))
	assert.Equal(t, "", sqluct.QuoteANSI())
	assert.Equal(t, `"spacy id"."back`+"`"+`ticky"."quo""ty"`, sqluct.QuoteANSI("spacy id", "back`ticky", `quo"ty`))
}

// Three benchmarks show different scenarios:
//  * full - referencer is recreated for each iteration, formatting is done in each iteration,
//  * lite - referencer is reused in all iterations, formatting is done in each iteration,
//  * raw - referencer is not used, squirrel uses manually prepared template.
//
// Performance overhead seems affordable, especially in case of reusable referencer.
//
// Sample benchmark result:
// goos: darwin
// goarch: amd64
// pkg: github.com/bool64/sqluct
// cpu: Intel(R) Core(TM) i7-8559U CPU @ 2.70GHz
// BenchmarkReferencer_Fmt_full-8   	   78751	     15938 ns/op	    8472 B/op	     130 allocs/op
// BenchmarkReferencer_Fmt_lite-8   	  151257	      7939 ns/op	    4785 B/op	     102 allocs/op
// BenchmarkReferencer_Fmt_raw-8    	  169131	      5986 ns/op	    4040 B/op	      75 allocs/op

func BenchmarkReferencer_Fmt_full(b *testing.B) {
	rf := sqluct.Referencer{}

	type User struct {
		ID        int    `db:"id,omitempty"`
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}

	type DirectReport struct {
		ManagerID  int `db:"manager_id"`
		EmployeeID int `db:"employee_id"`
	}

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		manager := &User{}
		rf.AddTableAlias(manager, "manager")

		employee := &User{}
		rf.AddTableAlias(employee, "employee")

		dr := &DirectReport{}
		rf.AddTableAlias(dr, "dr")

		// Find direct reports that share same last name and manager is not named John.
		qb := squirrel.StatementBuilder.Select(rf.Fmt("%s, %s", &dr.ManagerID, &dr.EmployeeID)).
			From(rf.Fmt("%s AS %s", rf.Q("users"), manager)).
			InnerJoin(rf.Fmt("%s AS %s ON %s = %s AND %s = %s",
				rf.Q("direct_reports"), dr,
				&dr.ManagerID, &manager.ID,
				&dr.EmployeeID, &employee.ID)).
			Where(rf.Fmt("%s = %s", &manager.LastName, &employee.LastName)).
			Where(rf.Fmt("%s != ?", &manager.FirstName), "John")

		_, _, err := qb.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func BenchmarkReferencer_Fmt_lite(b *testing.B) {
	rf := sqluct.Referencer{}

	type User struct {
		ID        int    `db:"id"`
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}

	type DirectReport struct {
		ManagerID  int `db:"manager_id"`
		EmployeeID int `db:"employee_id"`
	}

	manager := &User{}
	rf.AddTableAlias(manager, "manager")

	employee := &User{}
	rf.AddTableAlias(employee, "employee")

	dr := &DirectReport{}
	rf.AddTableAlias(dr, "dr")

	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Find direct reports that share same last name and manager is not named John.
		qb := squirrel.StatementBuilder.Select(rf.Fmt("%s, %s", &dr.ManagerID, &dr.EmployeeID)).
			From(rf.Fmt("%s AS %s", rf.Q("users"), manager)).
			InnerJoin(rf.Fmt("%s AS %s ON %s = %s AND %s = %s",
				rf.Q("direct_reports"), dr,
				&dr.ManagerID, &manager.ID,
				&dr.EmployeeID, &employee.ID)).
			Where(rf.Fmt("%s = %s", &manager.LastName, &employee.LastName)).
			Where(rf.Fmt("%s != ?", &manager.FirstName), "John")

		_, _, err := qb.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}

func BenchmarkReferencer_Fmt_raw(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Find direct reports that share same last name and manager is not named John.
		qb := squirrel.StatementBuilder.Select(`"dr"."manager_id", "dr"."employee_id"`).
			From(`"users" AS "manager"`).
			InnerJoin(`"direct_reports" AS "dr" ON "dr"."manager_id" = "manager"."id" AND "dr"."employee_id" = "employee"."id"`).
			Where(`"manager"."last_name" = "employee"."last_name"`).
			Where(`"manager"."first_name" != ?`, "John")

		_, _, err := qb.ToSql()
		if err != nil {
			b.Fail()
		}
	}
}
