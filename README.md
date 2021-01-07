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

Field tags (`db` by default) act as a source of truth for column names to allow better maintainability and fewer errors.

## Simple CRUD

```go
var (
    s   sqluct.Storage
    ctx context.Context
)

const tableName = "products"

type Product struct {
    ID        int       `db:"id"`
    Title     string    `db:"title"`
    CreatedAt time.Time `db:"created_at"`
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
    s.UpdateStmt(tableName, Product{Title: "Bananas"}, sqluct.SkipZeroValues).
        Where(s.WhereEq(Product{ID: 2}, sqluct.SkipZeroValues)),
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