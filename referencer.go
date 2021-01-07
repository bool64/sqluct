package sqluct

import (
	"fmt"
	"strings"
)

// QuoteANSI adds double quotes to symbols names.
//
// Suitable for PostgreSQL, MySQL in ANSI SQL_MODE, SQLite statements.
// Used in Referencer by default.
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

// AddTableAlias creates string references for row pointer and all suitable field pointers in it.
func (r *Referencer) AddTableAlias(rowStructPtr interface{}, alias string) {
	f, err := r.Mapper.FindColumnNames(rowStructPtr)
	if err != nil {
		panic(err)
	}

	if r.refs == nil {
		r.refs = make(map[interface{}]string, len(f)+1)
	}

	r.refs[rowStructPtr] = r.Q(alias)

	for ptr, fieldName := range f {
		r.refs[ptr] = r.Q(alias, fieldName)
	}
}

// Q quotes identifier.
func (r *Referencer) Q(tableAndColumn ...string) string {
	q := r.IdentifierQuoter
	if q == nil {
		q = QuoteNoop
	}

	return q(tableAndColumn...)
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
