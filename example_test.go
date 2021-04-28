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

	if err := s.Select(ctx, qb, &row); err != nil {
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

	if _, err := s.Exec(ctx, qb); err != nil {
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

	if _, err := s.Exec(ctx, qb); err != nil {
		log.Fatal(err)
	}
}

func ExampleStorage_Select_join() {
	var s sqluct.Storage

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

	rf := s.Ref()
	o := &Order{}
	u := &User{}

	rf.AddTableAlias(o, "orders")
	rf.AddTableAlias(u, "users")

	q := s.SelectStmt(rf.Ref(o), o, rf.ColumnsOf(o)).
		Columns(rf.Ref(&u.Name)).
		Join(rf.Fmt("%s ON %s = %s", u, &o.UserID, &u.ID)).
		Where(s.WhereEq(OrderData{
			Amount: 100,
			UserID: 123,
		}, rf.ColumnsOf(o)))

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: SELECT orders.id, orders.amount, orders.user_id, users.name FROM orders JOIN users ON orders.user_id = users.id WHERE orders.amount = $1 AND orders.user_id = $2 [100 123] <nil>
}

func ExampleSkipZeroValues() {
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

	// Output:
	// SELECT id, name, price FROM products WHERE id = $1 AND price = $2 [123 0] <nil>
	// SELECT id, name, price FROM products WHERE id = $1 AND name = $2 AND price = $3 [123  0] <nil>
	// SELECT id, name, price FROM products WHERE id = $1 [123] <nil>
}
