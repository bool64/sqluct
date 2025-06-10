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

// QuoteRequiredBackticks quotes symbol names that need quoting with backticks.
//
// Suitable for MySQL, SQLite statements.
// See also https://dev.mysql.com/doc/refman/8.4/en/identifiers.html.
func QuoteRequiredBackticks(tableAndColumn ...string) string {
	res := strings.Builder{}

	for i, item := range tableAndColumn {
		if i != 0 {
			res.WriteString(".")
		}

		needsQuote := false
		onlyDigits := true

		for _, r := range item {
			if r >= '0' && r <= '9' {
				continue
			}

			onlyDigits = false

			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '$' || r == '_' {
				continue
			}

			if r >= 0x0080 && r <= 0xFFFF {
				continue
			}

			needsQuote = true
		}

		// Identifiers may begin with a digit but unless quoted may not consist solely of digits.
		if !needsQuote && !onlyDigits {
			res.WriteString(item)

			continue
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

	refs        map[interface{}]Quoted
	quotedCols  map[interface{}]Quoted
	columnNames map[interface{}]string
	structRefs  map[interface{}][]string
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

// QuotedNoTable is a container of field pointer that should be referenced without table.
type QuotedNoTable struct {
	ptr interface{}
}

// NoTable enables references without table prefix.
// So that `my_table`.`my_column` would be rendered as `my_column`.
//
//		r.Ref(sqluct.NoTable(&row.MyColumn))
//	 r.Fmt("%s = 1", sqluct.NoTable(&row.MyColumn))
//
// Such references may be useful for INSERT/UPDATE column expressions.
func NoTable(ptr interface{}) QuotedNoTable {
	return QuotedNoTable{ptr: ptr}
}

// NoTableAll enables references without table prefix for all field pointers.
// It can be useful to prepare multiple variadic arguments.
//
//	 r.Fmt("ON CONFLICT(%s) DO UPDATE SET %s = excluded.%s, %s = excluded.%s",
//		sqluct.NoTableAll(&row.ID, &row.F1, &row.F1, &row.F2, &row.F3)...)
func NoTableAll(ptrs ...interface{}) []interface{} {
	res := make([]interface{}, 0, len(ptrs))
	for _, ptr := range ptrs {
		res = append(res, NoTable(ptr))
	}

	return res
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

	if r.quotedCols == nil {
		r.quotedCols = make(map[interface{}]Quoted, len(f)+1)
	}

	if r.columnNames == nil {
		r.columnNames = make(map[interface{}]string, len(f))
	}

	if r.structRefs == nil {
		r.structRefs = make(map[interface{}][]string)
	}

	if alias != "" {
		r.refs[rowStructPtr] = r.Q(alias)
	}

	refs := make([]string, 0, len(f))

	for ptr, fieldName := range f {
		var ref Quoted

		if alias == "" {
			ref = r.Q(fieldName)
		} else {
			ref = r.Q(alias, fieldName)
		}

		refs = append(refs, string(ref))
		r.refs[ptr] = ref
		r.quotedCols[ptr] = r.Q(fieldName)
		r.columnNames[ptr] = fieldName
	}

	sort.Strings(refs)

	r.structRefs[rowStructPtr] = refs
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
	s, err := r.ref(ptr)
	if err != nil {
		panic(err)
	}

	return s
}

func (r *Referencer) ref(ptr interface{}) (string, error) {
	if q, ok := ptr.(Quoted); ok {
		return string(q), nil
	}

	refs := r.refs

	if nt, ok := ptr.(QuotedNoTable); ok {
		ptr = nt.ptr
		refs = r.quotedCols
	}

	if ref, found := refs[ptr]; found {
		return string(ref), nil
	}

	return "", errUnknownFieldOrRow
}

// Refs returns reference strings for multiple field pointers.
//
// It panics if pointer is unknown.
func (r *Referencer) Refs(ptrs ...interface{}) []string {
	args := make([]string, 0, len(ptrs))

	for i, fieldPtr := range ptrs {
		ref, err := r.ref(fieldPtr)
		if err != nil {
			panic(fmt.Errorf("%w at position %d", err, i))
		}

		args = append(args, ref)
	}

	return args
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
		ref, err := r.ref(fieldPtr)
		if err != nil {
			panic(fmt.Errorf("%w at position %d", err, i))
		}

		args = append(args, ref)
	}

	return fmt.Sprintf(format, args...)
}

// Cols returns column references of a row structure.
func (r *Referencer) Cols(ptr interface{}) []string {
	if cols, found := r.structRefs[ptr]; found {
		return cols
	}

	panic(errUnknownFieldOrRow)
}

// Eq is a shortcut for squirrel.Eq{r.Ref(ptr): val}.
func (r *Referencer) Eq(ptr interface{}, val interface{}) squirrel.Eq {
	return squirrel.Eq{r.Ref(ptr): val}
}
