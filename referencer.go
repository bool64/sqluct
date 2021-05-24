package sqluct

import (
	"fmt"
	"strings"
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

	refs map[interface{}]string
}

// ColumnsOf makes a Mapper option to prefix columns with table alias.
//
// Argument is either a structure pointer or string alias.
func (r *Referencer) ColumnsOf(rowStructPtr interface{}) func(o *Options) {
	table, isString := rowStructPtr.(string)
	if !isString {
		t, found := r.refs[rowStructPtr]
		if !found {
			panic("row structure pointer needs to be added first with AddTableAlias")
		}

		table = t
	}

	return func(o *Options) {
		o.PrepareColumn = func(col string) string {
			return r.Q(table, col)
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
		r.refs = make(map[interface{}]string, len(f)+1)
	}

	if alias != "" {
		r.refs[rowStructPtr] = r.Q(alias)
	}

	for ptr, fieldName := range f {
		if alias == "" {
			r.refs[ptr] = r.Q(fieldName)
		} else {
			r.refs[ptr] = r.Q(alias, fieldName)
		}
	}
}

// Q quotes identifier.
func (r *Referencer) Q(tableAndColumn ...string) string {
	if r.IdentifierQuoter == nil {
		return QuoteNoop(tableAndColumn...)
	}

	return r.IdentifierQuoter(tableAndColumn...)
}

// Ref returns reference string for struct or field pointer that was previously added with AddTableAlias.
//
// It panics if pointer is unknown.
func (r *Referencer) Ref(ptr interface{}) string {
	if ref, found := r.refs[ptr]; found {
		return ref
	}

	panic(errFieldNotFound)
}

// Fmt formats according to a format specified replacing ptrs with their reference strings where possible.
//
// Values that are not available as reference string are passed to fmt.Sprintf as is.
func (r *Referencer) Fmt(format string, ptrs ...interface{}) string {
	args := make([]interface{}, 0, len(ptrs))

	for _, fieldPtr := range ptrs {
		if ref, found := r.refs[fieldPtr]; found {
			args = append(args, ref)
		} else {
			args = append(args, fieldPtr)
		}
	}

	return fmt.Sprintf(format, args...)
}
