// Package misc contains miscellaneous helpers.
package misc //nolint:revive

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/bool64/sqluct"
)

type pgColumn struct {
	Name          string
	DataType      string
	MaxLength     *int
	Precision     *int
	Scale         *int
	IsNullable    string
	ColumnDefault *string
}

type pgConstraint struct {
	Name        string
	Type        string
	Columns     string
	RefTable    *string
	RefColumns  *string
	CheckClause *string
}

// BuildPostgresCreateTable builds a CREATE TABLE statement for Postgres DB.
func BuildPostgresCreateTable(db *sql.DB, schema, table string) (string, error) {
	// Get columns
	columns, err := getColumns(db, schema, table)
	if err != nil {
		return "", err
	}

	// Get constraints
	constraints, err := getConstraints(db, schema, table)
	if err != nil {
		return "", err
	}

	// Build CREATE TABLE statement
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("CREATE TABLE %s.%s (\n", quoteIdentifier(schema), quoteIdentifier(table)))

	// Add columns
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(",\n")
		}

		sb.WriteString("    ")
		sb.WriteString(quoteIdentifier(col.Name))
		sb.WriteString(" ")
		sb.WriteString(formatDataType(col))

		if col.IsNullable == "NO" {
			sb.WriteString(" NOT NULL")
		}

		if col.ColumnDefault != nil {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(*col.ColumnDefault)
		}
	}

	// Add constraints
	for _, cons := range constraints {
		sb.WriteString(",\n    ")

		switch cons.Type {
		case "PRIMARY KEY":
			sb.WriteString(fmt.Sprintf("CONSTRAINT %s PRIMARY KEY (%s)", quoteIdentifier(cons.Name), cons.Columns))
		case "UNIQUE":
			sb.WriteString(fmt.Sprintf("CONSTRAINT %s UNIQUE (%s)", quoteIdentifier(cons.Name), cons.Columns))
		case "FOREIGN KEY":
			sb.WriteString(fmt.Sprintf("CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
				quoteIdentifier(cons.Name), cons.Columns, quoteIdentifier(*cons.RefTable), *cons.RefColumns))
		case "CHECK":
			sb.WriteString(fmt.Sprintf("CONSTRAINT %s CHECK %s", quoteIdentifier(cons.Name), *cons.CheckClause))
		}
	}

	sb.WriteString("\n);")

	return sb.String(), nil
}

func getColumns(db *sql.DB, schema, table string) ([]pgColumn, error) {
	rows, err := db.Query(`
        SELECT
            column_name,
            data_type,
            character_maximum_length,
            numeric_precision,
            numeric_scale,
            is_nullable,
            column_default
        FROM information_schema.columns
        WHERE table_schema = $1 AND table_name = $2
        ORDER BY ordinal_position
    `, schema, table)
	if err != nil {
		return nil, err
	}

	defer rows.Close() //nolint:errcheck

	var columns []pgColumn

	for rows.Next() {
		var (
			col                         pgColumn
			maxLength, precision, scale sql.NullInt64
			colDefault                  sql.NullString
		)

		if err := rows.Scan(&col.Name, &col.DataType, &maxLength, &precision, &scale, &col.IsNullable, &colDefault); err != nil {
			return nil, err
		}

		if maxLength.Valid {
			v := int(maxLength.Int64)
			col.MaxLength = &v
		}

		if precision.Valid {
			v := int(precision.Int64)
			col.Precision = &v
		}

		if scale.Valid {
			v := int(scale.Int64)
			col.Scale = &v
		}

		if colDefault.Valid {
			col.ColumnDefault = &colDefault.String
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func getConstraints(db *sql.DB, schema, table string) ([]pgConstraint, error) { //nolint:funlen
	var constraints []pgConstraint

	// Primary Key and Unique Constraints
	rows, err := db.Query(`
        SELECT
            tc.constraint_name,
            tc.constraint_type,
            string_agg(kcu.column_name, ', ') AS columns
        FROM information_schema.table_constraints tc
        JOIN information_schema.constraint_column_usage kcu
            ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            AND tc.table_name = kcu.table_name
        WHERE tc.table_schema = $1 AND tc.table_name = $2
            AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
        GROUP BY tc.constraint_name, tc.constraint_type
    `, schema, table)
	if err != nil {
		return nil, err
	}

	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var cons pgConstraint
		if err := rows.Scan(&cons.Name, &cons.Type, &cons.Columns); err != nil {
			return nil, err
		}

		constraints = append(constraints, cons)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Foreign Key Constraints
	rows, err = db.Query(`
        SELECT
            tc.constraint_name,
            string_agg(kcu.column_name, ', ') AS columns,
            ccu.table_name AS ref_table,
            string_agg(ccu.column_name, ', ') AS ref_columns
        FROM information_schema.table_constraints tc
        JOIN information_schema.constraint_column_usage ccu
            ON tc.constraint_name = ccu.constraint_name
            AND tc.table_schema = ccu.table_schema
        JOIN information_schema.key_column_usage kcu
            ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            AND tc.table_name = kcu.table_name
        WHERE tc.table_schema = $1 AND tc.table_name = $2
            AND tc.constraint_type = 'FOREIGN KEY'
        GROUP BY tc.constraint_name, ccu.table_schema, ccu.table_name
    `, schema, table)
	if err != nil {
		return nil, err
	}

	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var (
			cons                 pgConstraint
			refTable, refColumns string
		)

		cons.Type = "FOREIGN KEY"

		if err := rows.Scan(&cons.Name, &cons.Columns, &refTable, &refColumns); err != nil {
			return nil, err
		}

		cons.RefTable = &refTable
		cons.RefColumns = &refColumns
		constraints = append(constraints, cons)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Check Constraints
	rows, err = db.Query(`
        SELECT
            cc.constraint_name,
            cc.check_clause
        FROM information_schema.check_constraints cc
        JOIN information_schema.constraint_table_usage ctu
            ON cc.constraint_name = ctu.constraint_name
            AND ctu.table_schema = cc.constraint_schema
        WHERE ctu.table_schema = $1 AND ctu.table_name = $2
    `, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	for rows.Next() {
		var (
			cons        pgConstraint
			checkClause string
		)

		cons.Type = "CHECK"
		if err := rows.Scan(&cons.Name, &checkClause); err != nil {
			return nil, err
		}

		cons.CheckClause = &checkClause
		constraints = append(constraints, cons)
	}

	return constraints, rows.Err()
}

func formatDataType(col pgColumn) string {
	switch strings.ToLower(col.DataType) {
	case "character varying", "varchar":
		if col.MaxLength != nil {
			return fmt.Sprintf("VARCHAR(%d)", *col.MaxLength)
		}

		return "VARCHAR"
	case "numeric", "decimal":
		if col.Precision != nil && col.Scale != nil {
			return fmt.Sprintf("NUMERIC(%d,%d)", *col.Precision, *col.Scale)
		}

		return "NUMERIC"
	default:
		return strings.ToUpper(col.DataType)
	}
}

func quoteIdentifier(name string) string {
	return sqluct.QuoteRequiredANSI(name)
}
