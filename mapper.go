package sqluct

import (
	"errors"
	"reflect"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx/reflectx"
)

var (
	errFieldNotFound = errors.New("could not find field value in struct")
	errNotAPointer   = errors.New("can not take address of structure, please pass a pointer")
	errNilArgument   = errors.New("structPtr and fieldPtr are required")
)

// Mapper prepares select, insert and update statements.
type Mapper struct {
	ReflectMapper *reflectx.Mapper
}

var reflectMapper = reflectx.NewMapper("db")

// SkipZeroValues instructs mapper to ignore fields with zero values.
func SkipZeroValues(o *Options) {
	o.SkipZeroValues = true
}

// Columns is used to control which columns from the structure should be used.
func Columns(columns ...string) func(o *Options) {
	return func(o *Options) {
		o.Columns = columns
	}
}

// OrderDesc instructs mapper to use DESC order in Product func.
func OrderDesc(o *Options) {
	o.OrderDesc = true
}

// Options defines mapping parameters.
type Options struct {
	// SkipZeroValues instructs mapper to ignore fields with zero values.
	SkipZeroValues bool

	// Columns is used to control which columns from the structure should be used.
	Columns []string

	// OrderDesc instructs mapper to use DESC order in Product func.
	OrderDesc bool
}

// Insert adds struct value or slice of struct values to squirrel.InsertBuilder.
func (sm *Mapper) Insert(q squirrel.InsertBuilder, val interface{}, options ...func(*Options)) squirrel.InsertBuilder {
	if val == nil {
		return q
	}

	v := reflect.Indirect(reflect.ValueOf(val))

	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i)
			cols, vals := sm.getColumnsValues(item, options...)

			if i == 0 {
				q = q.Columns(cols...)
			}

			q = q.Values(vals...)
		}
	} else {
		cols, vals := sm.getColumnsValues(v, options...)
		q = q.Columns(cols...)
		q = q.Values(vals...)
	}

	return q
}

// Update sets struct value to squirrel.UpdateBuilder.
func (sm *Mapper) Update(q squirrel.UpdateBuilder, val interface{}, options ...func(*Options)) squirrel.UpdateBuilder {
	if val == nil {
		return q
	}

	cols, vals := sm.getColumnsValues(reflect.ValueOf(val), options...)
	for i, col := range cols {
		q = q.Set(col, vals[i])
	}

	return q
}

// Select maps struct field tags as columns to squirrel.SelectBuilder, slice of struct is also accepted.
func (sm *Mapper) Select(q squirrel.SelectBuilder, columns interface{}, options ...func(*Options)) squirrel.SelectBuilder {
	if columns == nil {
		return q
	}

	cols, _ := sm.getColumnsValues(reflect.ValueOf(columns), options...)
	q = q.Columns(cols...)

	return q
}

// WhereEq maps struct values as conditions to squirrel.Eq.
func (sm *Mapper) WhereEq(conditions interface{}, options ...func(*Options)) squirrel.Eq {
	columns, values := sm.getColumnsValues(reflect.ValueOf(conditions), options...)
	eq := make(squirrel.Eq, len(columns))

	for i, column := range columns {
		eq[column] = values[i]
	}

	if len(eq) == 0 {
		return nil
	}

	return eq
}

// Order maps struct field tags as "ORDER BY".
//
// Deprecated: use Col with DESC/ASC.
func (sm *Mapper) Order(columns interface{}, options ...func(*Options)) string {
	cols, _ := sm.getColumnsValues(reflect.ValueOf(columns), options...)
	order := ""
	orderDir := " ASC"

	o := Options{}

	for _, option := range options {
		option(&o)
	}

	if o.OrderDesc {
		orderDir = " DESC"
	}

	for _, col := range cols {
		order += ", " + col + orderDir
	}

	if len(order) > 0 {
		return order[2:]
	}

	return ""
}

func (sm *Mapper) colType(v reflect.Value, options ...func(*Options)) (*reflectx.StructMap, Options, bool) {
	v = reflect.Indirect(v)
	k := v.Kind()
	t := v.Type()
	skipValues := false
	o := Options{}

	for _, option := range options {
		option(&o)
	}

	if k == reflect.Slice || k == reflect.Array {
		t = t.Elem()
		k = t.Kind()
		skipValues = true
	}

	if k != reflect.Struct {
		panic("struct or slice/array of struct expected in sql query mapper")
	}

	tm := sm.reflectMapper().TypeMap(t)

	return tm, o, skipValues
}

func (sm *Mapper) skip(fi *reflectx.FieldInfo, columns []string) bool {
	if fi.Embedded {
		return true
	}

	if fi.Field.Tag == "" {
		return true
	}

	if len(columns) > 0 {
		pick := false

		for _, col := range columns {
			if col == fi.Name {
				pick = true

				break
			}
		}

		if !pick {
			return true
		}
	}

	return false
}

func isZero(colV reflect.Value, val interface{}) bool {
	k := colV.Kind()
	if k == reflect.Slice || k == reflect.Map {
		if colV.Len() == 0 {
			return true
		}
	} else {
		if val == nil || val == reflect.Zero(colV.Type()).Interface() {
			return true
		}
	}

	return false
}

func (sm *Mapper) getColumnsValues(v reflect.Value, options ...func(*Options)) ([]string, []interface{}) {
	tm, o, skipValues := sm.colType(v, options...)
	columns := make([]string, 0, len(tm.Index))
	values := make([]interface{}, 0, len(tm.Index))

	for _, fi := range tm.Index {
		if sm.skip(fi, o.Columns) {
			continue
		}

		if !skipValues {
			colV := reflectx.FieldByIndexesReadOnly(v, fi.Index)
			val := colV.Interface()

			if o.SkipZeroValues && isZero(colV, val) {
				continue
			}

			values = append(values, val)
		}

		columns = append(columns, fi.Name)
	}

	return columns, values
}

// FindColumnName returns column name of a database entity field.
//
// Entity field is defined by pointer to owner structure and pointer to field in that structure.
//   entity := MyEntity{}
//   name, found := sm.FindColumnName(&entity, &entity.UpdatedAt)
func (sm *Mapper) FindColumnName(structPtr, fieldPtr interface{}) (string, error) {
	if structPtr == nil || fieldPtr == nil {
		return "", errNilArgument
	}

	v := reflect.Indirect(reflect.ValueOf(structPtr))
	t := v.Type()

	if !v.CanAddr() {
		return "", errNotAPointer
	}

	tm := sm.reflectMapper().TypeMap(t)
	for _, fi := range tm.Index {
		fv := reflectx.FieldByIndexesReadOnly(v, fi.Index)
		if fv.Addr().Interface() == fieldPtr {
			return fi.Name, nil
		}
	}

	return "", errFieldNotFound
}

// Col will try to find column name and will panic on error.
func (sm *Mapper) Col(structPtr, fieldPtr interface{}) string {
	name, err := sm.FindColumnName(structPtr, fieldPtr)
	if err != nil {
		panic(err)
	}

	return name
}

func (sm *Mapper) reflectMapper() *reflectx.Mapper {
	if sm != nil && sm.ReflectMapper != nil {
		return sm.ReflectMapper
	}

	return reflectMapper
}
