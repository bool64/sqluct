//go:build go1.18
// +build go1.18

package sqluct_test

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
)

func ExampleTable() {
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
}
