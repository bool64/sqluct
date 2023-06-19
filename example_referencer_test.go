package sqluct_test

import (
	"fmt"
	"log"

	"github.com/Masterminds/squirrel"
	"github.com/bool64/sqluct"
)

func ExampleReferencer_Fmt() {
	type User struct {
		ID        int    `db:"id"`
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}

	type DirectReport struct {
		ManagerID  int `db:"manager_id"`
		EmployeeID int `db:"employee_id"`
	}

	rf := sqluct.Referencer{}

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

	stmt, args, err := qb.ToSql()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(stmt)
	fmt.Println(args)

	// Output:
	// SELECT dr.manager_id, dr.employee_id FROM users AS manager INNER JOIN direct_reports AS dr ON dr.manager_id = manager.id AND dr.employee_id = employee.id WHERE manager.last_name = employee.last_name AND manager.first_name != ?
	// [John]
}
