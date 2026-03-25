// Package dbm provides a lightweight generic database mapper.
// It wraps database/sql with three functions — Query, QueryOne, and Exec —
// that automatically scan rows into Go structs (via `db` tags) or primitives,
// and support named parameter binding from struct fields.
package dbm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ErrNotFound is returned by QueryOne when the query returns no rows.
var ErrNotFound = errors.New("not found")

// Query executes a query and scans all result rows into []T.
// When T is a struct, columns are mapped to fields via `db` tags.
// When T is a primitive type, each row's single column is scanned directly.
func Query[T any](ctx context.Context, db *sql.DB, query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("dbm.Query: %w", err)
	}
	defer rows.Close()

	var result []T
	scanner, err := newRowScanner[T](rows)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		val, err := scanner.scan(rows)
		if err != nil {
			return nil, fmt.Errorf("dbm.Query scan: %w", err)
		}
		result = append(result, val)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dbm.Query rows: %w", err)
	}
	if result == nil {
		result = []T{}
	}
	return result, nil
}

// QueryOne executes a query and scans the first result row into T.
// Returns ErrNotFound when the query returns no rows.
func QueryOne[T any](ctx context.Context, db *sql.DB, query string, args ...any) (T, error) {
	var zero T
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return zero, fmt.Errorf("dbm.QueryOne: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, fmt.Errorf("dbm.QueryOne rows: %w", err)
		}
		return zero, ErrNotFound
	}

	scanner, err := newRowScanner[T](rows)
	if err != nil {
		return zero, err
	}
	val, err := scanner.scan(rows)
	if err != nil {
		return zero, fmt.Errorf("dbm.QueryOne scan: %w", err)
	}
	return val, nil
}

// Exec executes a write statement (INSERT, UPDATE, DELETE, DDL).
// When T is a struct, named parameters (:field) in the query are replaced with
// values extracted from model fields via `db` tags. One execution per arg.
// When T is any other type, args are passed as positional ? parameters.
// Returns the sql.Result of the last successful execution.
func Exec[T any](ctx context.Context, db *sql.DB, query string, args ...T) (sql.Result, error) {
	if isStructType[T]() && strings.Contains(query, ":") {
		return execNamed[T](ctx, db, query, args...)
	}
	// Positional args: convert []T to []any
	positional := make([]any, len(args))
	for i, a := range args {
		positional[i] = a
	}
	result, err := db.ExecContext(ctx, query, positional...)
	if err != nil {
		return nil, fmt.Errorf("dbm.Exec: %w", err)
	}
	return result, nil
}

// execNamed handles struct-bound named parameter execution.
func execNamed[T any](ctx context.Context, db *sql.DB, query string, models ...T) (sql.Result, error) {
	if len(models) == 0 {
		result, err := db.ExecContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("dbm.Exec: %w", err)
		}
		return result, nil
	}

	var lastResult sql.Result
	for _, m := range models {
		rewritten, params, err := extractNamedParams(query, m)
		if err != nil {
			return nil, fmt.Errorf("dbm.Exec named params: %w", err)
		}
		result, err := db.ExecContext(ctx, rewritten, params...)
		if err != nil {
			return nil, fmt.Errorf("dbm.Exec: %w", err)
		}
		lastResult = result
	}
	return lastResult, nil
}

// --- Scanner infrastructure ---

// rowScanner handles scanning a single row into T.
type rowScanner[T any] struct {
	isStruct bool
	// struct scanning fields
	colIndexes []fieldIndex // maps column position → struct field path
	colCount   int
}

// fieldIndex describes how to reach a field in a possibly nested struct.
type fieldIndex struct {
	path     []int // index path through nested structs
	nullable bool  // true for pointer fields
	baseType reflect.Type
}

func newRowScanner[T any](rows *sql.Rows) (*rowScanner[T], error) {
	s := &rowScanner[T]{isStruct: isStructType[T]()}
	if !s.isStruct {
		return s, nil
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("dbm: columns: %w", err)
	}
	s.colCount = len(cols)

	t := reflect.TypeFor[T]()
	tagMap := buildTagMap(t, nil, "")
	s.colIndexes = make([]fieldIndex, len(cols))
	for i, col := range cols {
		if fi, ok := tagMap[col]; ok {
			s.colIndexes[i] = fi
		}
		// Unmapped columns get zero fieldIndex (path=nil), scanned into discard
	}
	return s, nil
}

func (s *rowScanner[T]) scan(rows *sql.Rows) (T, error) {
	var val T
	if !s.isStruct {
		if err := rows.Scan(&val); err != nil {
			return val, err
		}
		return val, nil
	}
	return s.scanStruct(rows)
}

func (s *rowScanner[T]) scanStruct(rows *sql.Rows) (T, error) {
	var val T
	rv := reflect.ValueOf(&val).Elem()

	// Build scan destinations
	dests := make([]any, s.colCount)
	holders := make([]nullHolder, s.colCount)

	for i, fi := range s.colIndexes {
		if fi.path == nil {
			// Unmapped column — discard
			dests[i] = &sql.RawBytes{}
			continue
		}
		field := fieldByPath(rv, fi.path)
		if fi.nullable {
			h := newNullHolder(fi.baseType)
			holders[i] = h
			dests[i] = h.scanDest()
		} else {
			dests[i] = field.Addr().Interface()
		}
	}

	if err := rows.Scan(dests...); err != nil {
		return val, err
	}

	// Assign nullable values back
	for i, fi := range s.colIndexes {
		if fi.path == nil || !fi.nullable {
			continue
		}
		field := fieldByPath(rv, fi.path)
		holders[i].assign(field)
	}

	return val, nil
}

// --- Null handling ---

type nullHolder interface {
	scanDest() any
	assign(field reflect.Value)
}

type nullTimeHolder struct {
	val sql.NullTime
}

func (h *nullTimeHolder) scanDest() any { return &h.val }
func (h *nullTimeHolder) assign(field reflect.Value) {
	if h.val.Valid {
		t := h.val.Time
		field.Set(reflect.ValueOf(&t))
	}
}

type nullInt64Holder struct {
	val sql.NullInt64
}

func (h *nullInt64Holder) scanDest() any { return &h.val }
func (h *nullInt64Holder) assign(field reflect.Value) {
	if h.val.Valid {
		v := h.val.Int64
		field.Set(reflect.ValueOf(&v))
	}
}

type nullFloat64Holder struct {
	val sql.NullFloat64
}

func (h *nullFloat64Holder) scanDest() any { return &h.val }
func (h *nullFloat64Holder) assign(field reflect.Value) {
	if h.val.Valid {
		v := h.val.Float64
		field.Set(reflect.ValueOf(&v))
	}
}

type nullStringHolder struct {
	val sql.NullString
}

func (h *nullStringHolder) scanDest() any { return &h.val }
func (h *nullStringHolder) assign(field reflect.Value) {
	if h.val.Valid {
		v := h.val.String
		field.Set(reflect.ValueOf(&v))
	}
}

func newNullHolder(baseType reflect.Type) nullHolder {
	switch baseType {
	case reflect.TypeFor[time.Time]():
		return &nullTimeHolder{}
	case reflect.TypeFor[int64]():
		return &nullInt64Holder{}
	case reflect.TypeFor[float64]():
		return &nullFloat64Holder{}
	case reflect.TypeFor[string]():
		return &nullStringHolder{}
	default:
		// Fallback: use NullString for unknown pointer types
		return &nullStringHolder{}
	}
}

// --- Reflect helpers ---

// buildTagMap builds a map of "db tag" → fieldIndex for a struct type,
// supporting nested structs via dot-separated column names.
func buildTagMap(t reflect.Type, basePath []int, prefix string) map[string]fieldIndex {
	result := make(map[string]fieldIndex)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}

		path := append(append([]int{}, basePath...), i)
		ft := f.Type
		nullable := ft.Kind() == reflect.Pointer
		if nullable {
			ft = ft.Elem()
		}

		// If the field is a struct (not time.Time) and it's a pointer to struct,
		// recurse into it for nested scanning
		if ft.Kind() == reflect.Struct && ft != reflect.TypeFor[time.Time]() {
			nested := buildTagMap(ft, path, tag+".")
			for k, v := range nested {
				// For pointer-to-struct fields, we need to handle initialization
				v.nullable = false // nested struct fields handle their own nullability
				result[k] = v
			}
			continue
		}

		colName := tag
		if prefix != "" {
			colName = prefix + tag
		}
		result[colName] = fieldIndex{
			path:     path,
			nullable: nullable,
			baseType: ft,
		}
	}
	return result
}

// fieldByPath navigates to a struct field by index path, initializing nil pointers.
func fieldByPath(v reflect.Value, path []int) reflect.Value {
	for _, idx := range path {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(idx)
	}
	return v
}

// isStructType returns true if T is a struct type (excluding time.Time).
func isStructType[T any]() bool {
	t := reflect.TypeFor[T]()
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct && t != reflect.TypeFor[time.Time]()
}

// --- Named parameter extraction ---

// extractNamedParams replaces :name placeholders with ? and extracts values from the model.
func extractNamedParams(query string, model any) (string, []any, error) {
	rv := reflect.ValueOf(model)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	rt := rv.Type()

	// Build tag → field value map
	tagValues := make(map[string]any)
	buildTagValues(rv, rt, tagValues)

	var rewritten strings.Builder
	var params []any
	i := 0
	for i < len(query) {
		if query[i] == ':' {
			// Extract param name
			j := i + 1
			for j < len(query) && isParamChar(query[j]) {
				j++
			}
			if j == i+1 {
				// Lone colon, keep as-is
				rewritten.WriteByte(query[i])
				i++
				continue
			}
			name := query[i+1 : j]
			val, ok := tagValues[name]
			if !ok {
				return "", nil, fmt.Errorf("named param :%s not found in struct tags", name)
			}
			rewritten.WriteByte('?')
			params = append(params, val)
			i = j
		} else {
			rewritten.WriteByte(query[i])
			i++
		}
	}
	return rewritten.String(), params, nil
}

func buildTagValues(rv reflect.Value, rt reflect.Type, out map[string]any) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}
		fv := rv.Field(i)
		ft := f.Type

		// For pointer fields, extract the underlying value or nil
		if ft.Kind() == reflect.Pointer {
			if fv.IsNil() {
				out[tag] = nil
			} else {
				out[tag] = fv.Elem().Interface()
			}
			continue
		}

		// For nested structs (not time.Time), skip — named params use flat tags
		if ft.Kind() == reflect.Struct && ft != reflect.TypeFor[time.Time]() {
			continue
		}

		out[tag] = fv.Interface()
	}
}

func isParamChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}
