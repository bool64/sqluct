package sqluct_test

import (
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
)

func ExampleMapper_Col() {
	sm := sqluct.Mapper{}

	type Order struct {
		ID        int       `db:"order_id"`
		CreatedAt time.Time `db:"created_at"`
	}

	o := Order{
		ID: 123,
	}

	q := sm.
		Select(squirrel.Select(), o).
		From("orders").
		Where(squirrel.Eq{
			sm.Col(&o, &o.ID): o.ID, // Col returns "order_id" defined in field tag.
		})
	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: SELECT order_id, created_at FROM orders WHERE order_id = ? [123] <nil>
}

func ExampleMapper_Insert() {
	sm := sqluct.Mapper{}

	type Order struct {
		ID     int `db:"order_id"`
		Amount int `db:"amount"`
		UserID int `db:"user_id"`
	}

	o := Order{}
	o.Amount = 100
	o.UserID = 123

	q := sm.Insert(squirrel.Insert("orders"), o, sqluct.SkipZeroValues)

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: INSERT INTO orders (amount,user_id) VALUES (?,?) [100 123] <nil>
}

func ExampleMapper_Update() {
	sm := sqluct.Mapper{}

	type OrderData struct {
		Amount int `db:"amount"`
		UserID int `db:"user_id"`
	}

	type Order struct {
		ID int `db:"order_id"`
		OrderData
	}

	o := Order{}
	o.ID = 321
	o.Amount = 100
	o.UserID = 123

	q := sm.
		Update(squirrel.Update("orders"), o.OrderData).
		Where(squirrel.Eq{sm.Col(&o, &o.ID): o.ID})

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: UPDATE orders SET amount = ?, user_id = ? WHERE order_id = ? [100 123 321] <nil>
}

func ExampleMapper_Select() {
	sm := sqluct.Mapper{}

	type OrderData struct {
		Amount int `db:"amount"`
		UserID int `db:"user_id"`
	}

	type Order struct {
		ID int `db:"order_id"`
		OrderData
	}

	o := Order{}
	o.ID = 321

	q := sm.
		Select(squirrel.Select(), o).
		Where(squirrel.Eq{sm.Col(&o, &o.ID): o.ID})

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: SELECT order_id, amount, user_id WHERE order_id = ? [321] <nil>
}

func ExampleMapper_WhereEq() {
	sm := sqluct.Mapper{}

	type OrderData struct {
		Amount int `db:"amount"`
		UserID int `db:"user_id"`
	}

	type Order struct {
		ID int `db:"order_id"`
		OrderData
	}

	o := Order{}
	o.Amount = 100
	o.UserID = 123

	q := sm.
		Select(squirrel.Select(), o).
		Where(sm.WhereEq(o.OrderData))

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: SELECT order_id, amount, user_id WHERE amount = ? AND user_id = ? [100 123] <nil>
}
