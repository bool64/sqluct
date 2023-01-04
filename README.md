# Struct-based database access layer for Go

[![Build Status](https://github.com/bool64/sqluct/workflows/test/badge.svg)](https://github.com/bool64/sqluct/actions?query=branch%3Amaster+workflow%3Atest)
[![Coverage Status](https://codecov.io/gh/bool64/sqluct/branch/master/graph/badge.svg)](https://codecov.io/gh/bool64/sqluct)
[![GoDevDoc](https://img.shields.io/badge/dev-doc-00ADD8?logo=go)](https://pkg.go.dev/github.com/bool64/sqluct)
[![Time Tracker](https://wakatime.com/badge/github/bool64/sqluct.svg)](https://wakatime.com/badge/github/bool64/sqluct)
![Code lines](https://sloc.xyz/github/bool64/sqluct/?category=code)
![Comments](https://sloc.xyz/github/bool64/sqluct/?category=comments)

This module integrates [`github.com/Masterminds/squirrel`](https://github.com/Masterminds/squirrel) query builder
and [`github.com/jmoiron/sqlx`](https://github.com/jmoiron/sqlx) to allow seamless operation based on field tags of row
structure.

All three libraries collaborate with standard `database/sql` and do not take away low level control from user.

This library helps to eliminate literal string column references (e.g. `"created_at"`) and use field references
instead (e.g. `rf.Ref(&row.CreatedAt)` and other mapping functions).

Field tags (`db` by default) act as a source of truth for column names to allow better maintainability and fewer errors.

## Components

`Storage` is a high level service that provides query building, query executing and result fetching facilities
as easy to use facades.

`Mapper` is a lower level tool that focuses on managing `squirrel` query builder with row structures.

`Referencer` helps to build complex statements by providing fully qualified and properly escaped names for 
participating columns.

## Simple CRUD

```go
// Open DB connection.
s, _ := sqluct.Open(
    "postgres",
    "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable",
)

ctx := context.TODO()

const tableName = "products"

type Product struct {
    ID        int       `db:"id,omitempty"`
    Title     string    `db:"title"`
    CreatedAt time.Time `db:"created_at,omitempty"`
}

// INSERT INTO products (id, title, created_at) VALUES (1, 'Apples', <now>), (2, 'Oranges', <now>)
_, err := s.Exec(ctx, s.InsertStmt(tableName, []Product{{
    ID:        1,
    Title:     "Apples",
    CreatedAt: time.Now(),
}, {
    ID:        2,
    Title:     "Oranges",
    CreatedAt: time.Now(),
},
}))
if err != nil {
    log.Fatal(err)
}

// UPDATE products SET title = 'Bananas' WHERE id = 2
_, err = s.Exec(
    ctx,
    s.UpdateStmt(tableName, Product{Title: "Bananas"}).
        Where(s.WhereEq(Product{ID: 2})),
)
if err != nil {
    log.Fatal(err)
}

var (
    result []Product
    row    Product
)
// SELECT id, title, created_at FROM products WHERE id != 3 AND created_at <= <now>
err = s.Select(ctx,
    s.SelectStmt(tableName, row).
        Where(squirrel.NotEq(s.WhereEq(Product{ID: 3}, sqluct.SkipZeroValues))).
        Where(squirrel.LtOrEq{s.Col(&row, &row.CreatedAt): time.Now()}),
    &result,
)
if err != nil {
    log.Fatal(err)
}

// DELETE FROM products WHERE id = 2
_, err = s.Exec(ctx, s.DeleteStmt(tableName).Where(Product{ID: 2}, sqluct.SkipZeroValues))
if err != nil {
    log.Fatal(err)
}
```

## Referencing Fields In Complex Statements

```go
type User struct {
    ID        int    `db:"id"`
    FirstName string `db:"first_name"`
    LastName  string `db:"last_name"`
}

type DirectReport struct {
    ManagerID  int `db:"manager_id"`
    EmployeeID int `db:"employee_id"`
}

var s sqluct.Storage

rf := s.Ref()

// Add aliased tables as pointers to structs.
manager := &User{}
rf.AddTableAlias(manager, "manager")

employee := &User{}
rf.AddTableAlias(employee, "employee")

dr := &DirectReport{}
rf.AddTableAlias(dr, "dr")

// Find direct reports that share same last name and manager is not named John.
qb := squirrel.StatementBuilder.Select(rf.Fmt("%s, %s", &dr.ManagerID, &dr.EmployeeID)).
    From(rf.Fmt("%s AS %s", rf.Q("users"), manager)). // Quote literal name and alias it with registered struct pointer.
    InnerJoin(rf.Fmt("%s AS %s ON %s = %s AND %s = %s",
        rf.Q("direct_reports"), dr,
        &dr.ManagerID, &manager.ID, // Identifiers are resolved using row field pointers.
        &dr.EmployeeID, &employee.ID)).
    Where(rf.Fmt("%s = %s", &manager.LastName, &employee.LastName)).
    Where(rf.Fmt("%s != ?", &manager.FirstName), "John") // Regular binds work same way as in standard squirrel.

stmt, args, err := qb.ToSql()
if err != nil {
    log.Fatal(err)
}

fmt.Println(stmt)
fmt.Println(args)

// SELECT dr.manager_id, dr.employee_id 
// FROM users AS manager 
// INNER JOIN direct_reports AS dr ON dr.manager_id = manager.id AND dr.employee_id = employee.id 
// WHERE manager.last_name = employee.last_name AND manager.first_name != ?
//
// [John]
```

## Omitting Zero Values

When building `WHERE` conditions from row structure it is often needed skip empty fields from condition. 

Behavior with empty fields (zero values) can be controlled via `omitempty` field tag flag and `sqluct.IgnoreOmitEmpty`,
`sqluct.SkipZeroValues` options.

Please check example below to learn about behavior differences.

```go
var s sqluct.Storage

type Product struct {
    ID    int    `db:"id,omitempty"`
    Name  string `db:"name,omitempty"`
    Price int    `db:"price"`
}

query, args, err := s.SelectStmt("products", Product{}).Where(s.WhereEq(Product{
    ID:    123,
    Price: 0,
})).ToSql()
fmt.Println(query, args, err)
// This query skips `name` in where condition for its zero value and `omitempty` flag.
//   SELECT id, name, price FROM products WHERE id = $1 AND price = $2 [123 0] <nil>

query, args, err = s.SelectStmt("products", Product{}).Where(s.WhereEq(Product{
    ID:    123,
    Price: 0,
}, sqluct.IgnoreOmitEmpty)).ToSql()
fmt.Println(query, args, err)
// This query adds `name` in where condition because IgnoreOmitEmpty is applied and `omitempty` flag is ignored.
//   SELECT id, name, price FROM products WHERE id = $1 AND name = $2 AND price = $3 [123  0] <nil>

query, args, err = s.SelectStmt("products", Product{}).Where(s.WhereEq(Product{
    ID:    123,
    Price: 0,
}, sqluct.SkipZeroValues)).ToSql()
fmt.Println(query, args, err)
// This query adds skips both price and name from where condition because SkipZeroValues option is applied.
//   SELECT id, name, price FROM products WHERE id = $1 [123] <nil>
```