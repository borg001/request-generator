package sqlcache

import (
	"context"
	"database/sql/driver"
	"io"
	"reflect"
	"time"
)

type column struct {
	databaseType          string
	scanType              reflect.Type
	length                int64
	hasLength             bool
	nullable, hasNullable bool
	precision, scale      int64
	hasPrecision          bool
}
type snapshot struct {
	columns   []string
	types     []column
	rows      [][]driver.Value
	bytes     int64
	completed time.Time
}

func copyValue(v driver.Value) driver.Value {
	if b, ok := v.([]byte); ok {
		if b == nil {
			return []byte(nil)
		}
		cloned := make([]byte, len(b))
		copy(cloned, b)
		return cloned
	}
	return v
}

func describe(rows driver.Rows) *snapshot {
	s := &snapshot{columns: append([]string(nil), rows.Columns()...)}
	s.types = make([]column, len(s.columns))
	s.bytes = int64(128 + len(s.columns)*128)
	for i, name := range s.columns {
		s.bytes += int64(len(name))
		c := &s.types[i]
		c.scanType = reflect.TypeOf((*interface{})(nil)).Elem()
		if r, ok := rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
			c.databaseType = r.ColumnTypeDatabaseTypeName(i)
		}
		if r, ok := rows.(driver.RowsColumnTypeScanType); ok {
			c.scanType = r.ColumnTypeScanType(i)
		}
		if r, ok := rows.(driver.RowsColumnTypeLength); ok {
			c.length, c.hasLength = r.ColumnTypeLength(i)
		}
		if r, ok := rows.(driver.RowsColumnTypeNullable); ok {
			c.nullable, c.hasNullable = r.ColumnTypeNullable(i)
		}
		if r, ok := rows.(driver.RowsColumnTypePrecisionScale); ok {
			c.precision, c.scale, c.hasPrecision = r.ColumnTypePrecisionScale(i)
		}
	}
	return s
}

// Rows exceeding the budget retain only the prefix already read, then stream
// the rest of the original result. Neither partial results nor errors are kept.
func capture(ctx context.Context, rows driver.Rows, limit int64) (*snapshot, driver.Rows, error) {
	s := describe(rows)
	for {
		if err := ctx.Err(); err != nil {
			rows.Close()
			return nil, nil, err
		}
		values := make([]driver.Value, len(s.columns))
		err := rows.Next(values)
		if err == io.EOF {
			if r, ok := rows.(driver.RowsNextResultSet); ok && r.HasNextResultSet() {
				return nil, &cachedRows{snapshot: s, tail: rows}, nil
			}
			if err := rows.Close(); err != nil {
				return nil, nil, err
			}
			s.completed = time.Now()
			return s, s.reader(), nil
		}
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		s.bytes += int64(24 + len(values)*40)
		for i, v := range values {
			values[i] = copyValue(v)
			switch v := v.(type) {
			case []byte:
				s.bytes += int64(len(v))
			case string:
				s.bytes += int64(len(v))
			}
		}
		s.rows = append(s.rows, values)
		if s.bytes > limit {
			return nil, &cachedRows{snapshot: s, tail: rows}, nil
		}
	}
}

type cachedRows struct {
	*snapshot
	index int
	tail  driver.Rows
}

func (s *snapshot) reader() driver.Rows { return &cachedRows{snapshot: s} }
func (r *cachedRows) Columns() []string { return append([]string(nil), r.columns...) }
func (r *cachedRows) Close() error {
	if r.tail != nil {
		return r.tail.Close()
	}
	return nil
}
func (r *cachedRows) Next(dest []driver.Value) error {
	if r.index < len(r.rows) {
		for i, v := range r.rows[r.index] {
			dest[i] = copyValue(v)
		}
		r.index++
		return nil
	}
	if r.tail != nil {
		return r.tail.Next(dest)
	}
	return io.EOF
}
func (r *cachedRows) HasNextResultSet() bool {
	tail, ok := r.tail.(driver.RowsNextResultSet)
	return ok && tail.HasNextResultSet()
}
func (r *cachedRows) NextResultSet() error {
	tail, ok := r.tail.(driver.RowsNextResultSet)
	if !ok {
		return io.EOF
	}
	if err := tail.NextResultSet(); err != nil {
		return err
	}
	r.snapshot = describe(r.tail)
	r.index = 0
	return nil
}
func (r *cachedRows) ColumnTypeDatabaseTypeName(i int) string { return r.types[i].databaseType }
func (r *cachedRows) ColumnTypeScanType(i int) reflect.Type   { return r.types[i].scanType }
func (r *cachedRows) ColumnTypeLength(i int) (int64, bool) {
	c := r.types[i]
	return c.length, c.hasLength
}
func (r *cachedRows) ColumnTypeNullable(i int) (bool, bool) {
	c := r.types[i]
	return c.nullable, c.hasNullable
}
func (r *cachedRows) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	c := r.types[i]
	return c.precision, c.scale, c.hasPrecision
}
