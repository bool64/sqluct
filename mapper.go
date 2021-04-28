package sqluct

import (
	"errors"
	"reflect"
	"sync"

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

	mu    sync.Mutex
	types map[reflect.Type]*reflectx.StructMap
}

var (
	reflectMapper = reflectx.NewMapper("db")
	defaultMapper = &Mapper{}
)

// SkipZeroValues instructs mapper to ignore fields with zero values.
func SkipZeroValues(o *Options) {
	o.SkipZeroValues = true
}

// IgnoreOmitEmpty instructs mapper to use zero values of fields with `omitempty`.
func IgnoreOmitEmpty(o *Options) {
	o.IgnoreOmitEmpty = true
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
	// SkipZeroValues instructs mapper to ignore fields with zero values regardless of `omitempty` tag.
	SkipZeroValues bool

	// IgnoreOmitEmpty instructs mapper to use zero values of fields with `omitempty`.
	IgnoreOmitEmpty bool

	// Columns is used to control which columns from the structure should be used.
	Columns []string

	// OrderDesc instructs mapper to use DESC order in Product func.
	OrderDesc bool

	// PrepareColumn allows control of column quotation or aliasing.
	PrepareColumn func(col string) string
}

// Insert adds struct value or slice of struct values to squirrel.InsertBuilder.
func (sm *Mapper) Insert(q squirrel.InsertBuilder, val interface{}, options ...func(*Options)) squirrel.InsertBuilder {
	if val == nil {
		return q
	}

	v := reflect.Indirect(reflect.ValueOf(val))

	if v.Kind() == reflect.Slice {
		return sm.sliceInsert(q, v, options...)
	}

	cols, vals := sm.ColumnsValues(v, options...)
	q = q.Columns(cols...)
	q = q.Values(vals...)

	return q
}

func (sm *Mapper) sliceInsert(q squirrel.InsertBuilder, v reflect.Value, options ...func(*Options)) squirrel.InsertBuilder {
	var (
		hCols         = make(map[string]struct{})
		heterogeneous = false
		qq            = q
	)

	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		cols, vals := sm.ColumnsValues(item, options...)

		if i == 0 {
			for _, c := range cols {
				hCols[c] = struct{}{}
			}

			qq = qq.Columns(cols...)
		} else {
			for _, c := range cols {
				if _, found := hCols[c]; !found {
					heterogeneous = true
					hCols[c] = struct{}{}
				}
			}
		}

		if !heterogeneous {
			qq = qq.Values(vals...)
		}
	}

	if heterogeneous {
		return sm.heterogeneousInsert(q, v, hCols, options...)
	}

	return qq
}

func (sm *Mapper) heterogeneousInsert(q squirrel.InsertBuilder, v reflect.Value, hCols map[string]struct{}, options ...func(*Options)) squirrel.InsertBuilder {
	cols := make([]string, 0, len(hCols))
	for c := range hCols {
		cols = append(cols, c)
	}

	options = append(options[0:len(options):len(options)], func(options *Options) {
		options.SkipZeroValues = false
		options.IgnoreOmitEmpty = true
		options.Columns = cols
	})

	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		cols, vals := sm.ColumnsValues(item, options...)

		if i == 0 {
			q = q.Columns(cols...)
		}

		q = q.Values(vals...)
	}

	return q
}

// Update sets struct value to squirrel.UpdateBuilder.
func (sm *Mapper) Update(q squirrel.UpdateBuilder, val interface{}, options ...func(*Options)) squirrel.UpdateBuilder {
	if val == nil {
		return q
	}

	cols, vals := sm.ColumnsValues(reflect.ValueOf(val), options...)
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

	cols, _ := sm.ColumnsValues(reflect.ValueOf(columns), append(options, IgnoreOmitEmpty)...)
	q = q.Columns(cols...)

	return q
}

// WhereEq maps struct values as conditions to squirrel.Eq.
func (sm *Mapper) WhereEq(conditions interface{}, options ...func(*Options)) squirrel.Eq {
	columns, values := sm.ColumnsValues(reflect.ValueOf(conditions), options...)
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
	cols, _ := sm.ColumnsValues(reflect.ValueOf(columns), options...)
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

	tm := sm.typeMap(t)

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

// ColumnsValues extracts columns and values from provided struct value.
func (sm *Mapper) ColumnsValues(v reflect.Value, options ...func(*Options)) ([]string, []interface{}) {
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

			_, omitEmpty := fi.Options["omitempty"]

			if o.IgnoreOmitEmpty && omitEmpty {
				omitEmpty = false
			}

			if (o.SkipZeroValues || omitEmpty) && isZero(colV, val) {
				continue
			}

			values = append(values, val)
		}

		if o.PrepareColumn != nil {
			columns = append(columns, o.PrepareColumn(fi.Name))
		} else {
			columns = append(columns, fi.Name)
		}
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

	tm := sm.typeMap(t)
	for _, fi := range tm.Index {
		fv := reflectx.FieldByIndexesReadOnly(v, fi.Index)
		if fv.Addr().Interface() == fieldPtr {
			return fi.Name, nil
		}
	}

	return "", errFieldNotFound
}

func (sm *Mapper) typeMap(t reflect.Type) *reflectx.StructMap {
	if sm == nil {
		sm = defaultMapper
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	tm, found := sm.types[t]
	if found {
		return tm
	}

	tm = sm.reflectMapper().TypeMap(t)
	index := make([]*reflectx.FieldInfo, 0, len(tm.Index))

	for _, fi := range tm.Index {
		skip := false
		p := fi.Parent

		// Field is allowed to be a column if does not have a named parent (with non-empty path)
		// or all parents are embedded.
		for p != nil && p.Path != "" {
			if !p.Embedded {
				skip = true

				break
			}

			p = p.Parent
		}

		if skip {
			continue
		}

		index = append(index, fi)
	}

	tm.Index = index

	if sm.types == nil {
		sm.types = make(map[reflect.Type]*reflectx.StructMap, 1)
	}

	sm.types[t] = tm

	return tm
}

// FindColumnNames returns column names mapped by a pointer to a field.
func (sm *Mapper) FindColumnNames(structPtr interface{}) (map[interface{}]string, error) {
	if structPtr == nil {
		return nil, errNilArgument
	}

	v := reflect.Indirect(reflect.ValueOf(structPtr))
	t := v.Type()

	if !v.CanAddr() {
		return nil, errNotAPointer
	}

	res := make(map[interface{}]string)

	tm := sm.typeMap(t)
	for _, fi := range tm.Index {
		fv := reflectx.FieldByIndexesReadOnly(v, fi.Index)
		res[fv.Addr().Interface()] = fi.Name
	}

	return res, nil
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
