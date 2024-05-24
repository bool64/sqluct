# Struct-based database access layer for Go

[![test-unit](https://github.com/bool64/sqluct/actions/workflows/test-unit.yml/badge.svg)](https://github.com/bool64/sqluct/actions/workflows/test-unit.yml)
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

`StorageOf[V any]` typed query builder and scanner for specific table(s).

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

// Or if you already have an *sql.DB or *sqlx.DB instances, you can use them:
// 	 db, _ := sql.Open("postgres", "postgres://pqgotest:password@localhost/pqgotest?sslmode=disable")
//   s := sqluct.NewStorage(sqlx.NewDb(db, "postgres"))

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

// You can also use generic sqluct.Get and sqluct.List in go1.18 or later.
//
// SELECT id, title, created_at FROM products WHERE id != 3 AND created_at <= <now>
result, err = sqluct.List[Product](ctx,
	s,
    s.SelectStmt(tableName, row).
        Where(squirrel.NotEq(s.WhereEq(Product{ID: 3}, sqluct.SkipZeroValues))).
        Where(squirrel.LtOrEq{s.Col(&row, &row.CreatedAt): time.Now()}),
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

## Typed Storage

`sqluct.Table[RowType](storageInstance, tableName)` creates a type-safe storage accessor to a table with `RowType`.
This accessor can help to retrieve or store data. Columns from multiple tables can be joined using field pointers.

Please check features overview in an example below.

```go
var (
    st  = sqluct.NewStorage(sqlx.NewDb(sql.OpenDB(dumpConnector{}), "postgres"))
    ctx = context.Background()
)

st.IdentifierQuoter = sqluct.QuoteANSI

type User struct {
    ID     int    `db:"id"`
    RoleID int    `db:"role_id"`
    Name   string `db:"name"`
}

// Users repository.
ur := sqluct.Table[User](st, "users")

// Pointer to row, that can be used to reference columns via struct fields.
_ = ur.R

// Single user record can be inserted, last insert id (if available) and error are returned.
fmt.Println("Insert single user.")
_, _ = ur.InsertRow(ctx, User{Name: "John Doe", ID: 123})

// Multiple user records can be inserted with sql.Result and error returned.
fmt.Println("Insert two users.")
_, _ = ur.InsertRows(ctx, []User{{Name: "Jane Doe", ID: 124}, {Name: "Richard Roe", ID: 125}})

// Update statement for a single user with condition.
fmt.Println("Update a user with new name.")
_, _ = ur.UpdateStmt(User{Name: "John Doe, Jr.", ID: 123}).Where(ur.Eq(&ur.R.ID, 123)).ExecContext(ctx)

// Delete statement for a condition.
fmt.Println("Delete a user with id 123.")
_, _ = ur.DeleteStmt().Where(ur.Eq(&ur.R.ID, 123)).ExecContext(ctx)

fmt.Println("Get single user with id = 123.")
user, _ := ur.Get(ctx, ur.SelectStmt().Where(ur.Eq(&ur.R.ID, 123)))

// Squirrel expression can be formatted with %s reference(s) to column pointer.
fmt.Println("Get multiple users with names starting with 'John '.")
users, _ := ur.List(ctx, ur.SelectStmt().Where(ur.Fmt("%s LIKE ?", &ur.R.Name), "John %"))

// Squirrel expressions can be applied.
fmt.Println("Get multiple users with id != 123.")
users, _ = ur.List(ctx, ur.SelectStmt().Where(squirrel.NotEq(ur.Eq(&ur.R.ID, 123))))

fmt.Println("Get all users.")
users, _ = ur.List(ctx, ur.SelectStmt())

// More complex statements can be made with references to other tables.

type Role struct {
    ID   int    `db:"id"`
    Name string `db:"name"`
}

// Roles repository.
rr := sqluct.Table[Role](st, "roles")

// To be able to resolve "roles" columns, we need to attach roles repo to users repo.
ur.AddTableAlias(rr.R, "roles")

fmt.Println("Get users with role 'admin'.")
users, _ = ur.List(ctx, ur.SelectStmt().
    LeftJoin(ur.Fmt("%s ON %s = %s", rr.R, &rr.R.ID, &ur.R.RoleID)).
    Where(ur.Fmt("%s = ?", &rr.R.Name), "admin"),
)

_ = user
_ = users

// Output:
// Insert single user.
// exec INSERT INTO "users" ("id","role_id","name") VALUES ($1,$2,$3) [123 0 John Doe]
// Insert two users.
// exec INSERT INTO "users" ("id","role_id","name") VALUES ($1,$2,$3),($4,$5,$6) [124 0 Jane Doe 125 0 Richard Roe]
// Update a user with new name.
// exec UPDATE "users" SET "id" = $1, "role_id" = $2, "name" = $3 WHERE "users"."id" = $4 [123 0 John Doe, Jr. 123]
// Delete a user with id 123.
// exec DELETE FROM "users" WHERE "users"."id" = $1 [123]
// Get single user with id = 123.
// query SELECT "users"."id", "users"."role_id", "users"."name" FROM "users" WHERE "users"."id" = $1 [123]
// Get multiple users with names starting with 'John '.
// query SELECT "users"."id", "users"."role_id", "users"."name" FROM "users" WHERE "users"."name" LIKE $1 [John %]
// Get multiple users with id != 123.
// query SELECT "users"."id", "users"."role_id", "users"."name" FROM "users" WHERE "users"."id" <> $1 [123]
// Get all users.
// query SELECT "users"."id", "users"."role_id", "users"."name" FROM "users" []
// Get users with role 'admin'.
// query SELECT "users"."id", "users"."role_id", "users"."name" FROM "users" LEFT JOIN "roles" ON "roles"."id" = "users"."role_id" WHERE "roles"."name" = $1 [admin]
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
