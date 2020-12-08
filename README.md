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

## Example

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