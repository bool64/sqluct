package sqluct

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/squirrel"
)

// QuoteANSI adds double quotes to symbols names.
//
// Suitable for PostgreSQL, MySQL in ANSI SQL_MODE, SQLite statements.
func QuoteANSI(tableAndColumn ...string) string {
	res := strings.Builder{}

	for i, item := range tableAndColumn {
		if i != 0 {
			res.WriteString(".")
		}

		res.WriteString(`"`)
		res.WriteString(strings.ReplaceAll(item, `"`, `""`))
		res.WriteString(`"`)
	}

	return res.String()
}

// QuoteBackticks quotes symbol names with backticks.
//
// Suitable for MySQL, SQLite statements.
func QuoteBackticks(tableAndColumn ...string) string {
	res := strings.Builder{}

	for i, item := range tableAndColumn {
		if i != 0 {
			res.WriteString(".")
		}

		res.WriteString("`")
		res.WriteString(strings.ReplaceAll(item, "`", "``"))
		res.WriteString("`")
	}

	return res.String()
}

// QuoteNoop does not add any quotes to symbol names.
//
// Used in Referencer by default.
func QuoteNoop(tableAndColumn ...string) string {
	return strings.Join(tableAndColumn, ".")
}

// Referencer maintains a list of string references to fields and table aliases.
type Referencer struct {
	Mapper *Mapper

	// IdentifierQuoter is formatter of column and table names.
	// Default QuoteNoop.
	IdentifierQuoter func(tableAndColumn ...string) string

	refs          map[interface{}]Quoted
	columnNames   map[interface{}]string
	structColumns map[interface{}][]string
}

// ColumnsOf makes a Mapper option to prefix columns with table alias.
//
// Argument is either a structure pointer or string alias.
func (r *Referencer) ColumnsOf(rowStructPtr interface{}) func(o *Options) {
	var table Quoted

	switch v := rowStructPtr.(type) {
	case string:
		table = r.Q(v)
	case Quoted:
		table = v
	default:
		t, found := r.refs[rowStructPtr]
		if !found {
			panic("row structure pointer needs to be added first with AddTableAlias")
		}

		table = t
	}

	return func(o *Options) {
		o.PrepareColumn = func(col string) string {
			return string(table + "." + r.Q(col))
		}
	}
}

// AddTableAlias creates string references for row pointer and all suitable field pointers in it.
//
// Empty alias is not added to column reference.
func (r *Referencer) AddTableAlias(rowStructPtr interface{}, alias string) {
	f, err := mapper(r.Mapper).FindColumnNames(rowStructPtr)
	if err != nil {
		panic(err)
	}

	if r.refs == nil {
		r.refs = make(map[interface{}]Quoted, len(f)+1)
	}

	if r.columnNames == nil {
		r.columnNames = make(map[interface{}]string, len(f))
	}

	if r.structColumns == nil {
		r.structColumns = make(map[interface{}][]string)
	}

	if alias != "" {
		r.refs[rowStructPtr] = r.Q(alias)
	}

	columns := make([]string, 0, len(f))

	for ptr, fieldName := range f {
		var col string

		if alias == "" {
			col = string(r.Q(fieldName))
		} else {
			col = string(r.Q(alias, fieldName))
		}

		columns = append(columns, col)
		r.refs[ptr] = Quoted(col)
		r.columnNames[ptr] = fieldName
	}

	sort.Strings(columns)

	r.structColumns[rowStructPtr] = columns
}

// Quoted is a string that can be interpolated into an SQL statement as is.
type Quoted string

// Q quotes identifier.
func (r *Referencer) Q(tableAndColumn ...string) Quoted {
	if r.IdentifierQuoter == nil {
		return Quoted(QuoteNoop(tableAndColumn...))
	}

	return Quoted(r.IdentifierQuoter(tableAndColumn...))
}

// Ref returns reference string for struct or field pointer that was previously added with AddTableAlias.
//
// It panics if pointer is unknown.
func (r *Referencer) Ref(ptr interface{}) string {
	if ref, found := r.refs[ptr]; found {
		return string(ref)
	}

	panic(errUnknownFieldOrRow)
}

// Col returns unescaped column name for field pointer that was previously added with AddTableAlias.
//
// It panics if pointer is unknown.
// Might be used with Options.Columns.
func (r *Referencer) Col(ptr interface{}) string {
	if col, found := r.columnNames[ptr]; found {
		return col
	}

	panic(errUnknownFieldOrRow)
}

// Fmt formats according to a format specified replacing ptrs with their reference strings where possible.
//
// It panics if pointer is unknown or is not a Quoted string.
func (r *Referencer) Fmt(format string, ptrs ...interface{}) string {
	args := make([]interface{}, 0, len(ptrs))

	for i, fieldPtr := range ptrs {
		if q, ok := fieldPtr.(Quoted); ok {
			args = append(args, string(q))

			continue
		}

		if ref, found := r.refs[fieldPtr]; found {
			args = append(args, ref)
		} else {
			panic(fmt.Errorf("%w at position %d", errUnknownFieldOrRow, i))
		}
	}

	return fmt.Sprintf(format, args...)
}

// Cols returns column references of a row structure.
func (r *Referencer) Cols(ptr interface{}) []string {
	if cols, found := r.structColumns[ptr]; found {
		return cols
	}

	panic(errUnknownFieldOrRow)
}

// Eq is a shortcut for squirrel.Eq{r.Ref(ptr): val}.
func (r *Referencer) Eq(ptr interface{}, val interface{}) squirrel.Eq {
	return squirrel.Eq{r.Ref(ptr): val}
}
