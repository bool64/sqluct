package sqluct_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
)

func ExampleStorage_InTx_full() {
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
	_, err := s.Exec(ctx, s.InsertStmt(tableName, []Product{
		{
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
}

func ExampleStorage_InTx() {
	var (
		s   sqluct.Storage
		ctx context.Context
	)

	err := s.InTx(ctx, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleStorage_Select_slice() {
	var (
		db  *sqlx.DB // Setup db connection.
		ctx context.Context
	)

	s := sqluct.NewStorage(db)

	// Define your entity as a struct with `db` field tags that correspond to column names in table.
	type MyEntity struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	// Create destination for query result.
	rows := make([]MyEntity, 0, 100)

	// Create SELECT statement from fields of entity.
	qb := s.SelectStmt("my_table", MyEntity{}).
		Where(s.WhereEq(MyEntity{
			Name: "Jane",
		}, sqluct.SkipZeroValues)) // Add WHERE condition built from fields of entity.

	// Query statement would be
	// 	SELECT name, age FROM my_table WHERE name = $1
	// with argument 'Jane'.

	err := s.Select(ctx, qb, &rows)
	if err != nil {
		log.Fatal(err)
	}

	for _, row := range rows {
		fmt.Println(row)
	}
}

func ExampleStorage_Select_oneRow() {
	var (
		s   sqluct.Storage
		ctx context.Context
	)

	type MyEntity struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	var row MyEntity

	qb := s.SelectStmt("my_table", row)

	err := s.Select(ctx, qb, &row)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleStorage_InsertStmt() {
	var (
		s   sqluct.Storage
		ctx context.Context
	)

	type MyEntity struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	row := MyEntity{
		Name: "Jane",
		Age:  30,
	}

	qb := s.InsertStmt("my_table", row)

	_, err := s.Exec(ctx, qb)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleStorage_UpdateStmt() {
	var (
		s   sqluct.Storage
		ctx context.Context
	)

	type MyIdentity struct {
		ID int `db:"id"`
	}

	type MyValue struct {
		Name string `db:"name"`
		Age  int    `db:"age"`
	}

	row := MyValue{
		Name: "Jane",
		Age:  30,
	}

	qb := s.UpdateStmt("my_table", row).
		Where(s.Mapper.WhereEq(MyIdentity{ID: 123}))

	_, err := s.Exec(ctx, qb)
	if err != nil {
		log.Fatal(err)
	}
}
