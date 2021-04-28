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
		ID        int       `db:"order_id,omitempty"`
		CreatedAt time.Time `db:"created_at,omitempty"`
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
		ID     int `db:"order_id,omitempty"`
		Amount int `db:"amount"`
		UserID int `db:"user_id"`
	}

	o := Order{}
	o.Amount = 100
	o.UserID = 123

	q := sm.Insert(squirrel.Insert("orders"), o)

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: INSERT INTO orders (amount,user_id) VALUES (?,?) [100 123] <nil>
}

func ExampleMapper_Update() {
	sm := sqluct.Mapper{}

	type OrderData struct {
		Amount int `db:"amount"`
		UserID int `db:"user_id,omitempty"`
	}

	type Order struct {
		ID int `db:"order_id,omitempty"`
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
		UserID int `db:"user_id,omitempty"`
	}

	type Order struct {
		ID int `db:"order_id,omitempty"`
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
		UserID int `db:"user_id,omitempty"`
	}

	type Order struct {
		ID int `db:"order_id"`
		OrderData
	}

	o := Order{}
	o.Amount = 100
	o.UserID = 123

	q := sm.
		Select(squirrel.Select().From("orders"), o).
		Where(sm.WhereEq(o.OrderData))

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: SELECT order_id, amount, user_id FROM orders WHERE amount = ? AND user_id = ? [100 123] <nil>
}

func ExampleMapper_WhereEq_columnsOf() {
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

	q := sm.
		Select(squirrel.Select().From(rf.Ref(o)), o, rf.ColumnsOf(o)).
		Join(rf.Fmt("%s ON %s = %s", u, &o.UserID, &u.ID)).
		Where(sm.WhereEq(OrderData{
			Amount: 100,
			UserID: 123,
		}, rf.ColumnsOf(o)))

	query, args, err := q.ToSql()
	fmt.Println(query, args, err)

	// Output: SELECT orders.id, orders.amount, orders.user_id FROM orders JOIN users ON orders.user_id = users.id WHERE orders.amount = ? AND orders.user_id = ? [100 123] <nil>
}
