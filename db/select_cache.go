package db

import (
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ListSelectCacheOptions explicitly permits stale SQL results for generated
// List SELECT and COUNT only. View, arbitrary Query, transactions and writes
// are never cached. The producer must opt in only when ALL selected expressions
// and policies tolerate TTL staleness, and database session state is invariant.
// There is no cross-process invalidation or read-after-write guarantee.
type ListSelectCacheOptions struct {
	// TTL starts once the complete successful SQL result has been read.
	TTL            time.Duration
	MaxEntries     int
	MaxBytes       int64
	MaxResultBytes int64
}

type SelectCacheStats struct {
	Hits, Misses, Shared, Bypassed uint64
	Entries                        int
	Bytes                          int64
}

type cachedSelect struct {
	rows    [][]interface{}
	bytes   int64
	expires time.Time
}

type selectFlight struct {
	done   chan struct{}
	result *cachedSelect
	err    error
}

type listSelectCache struct {
	mu      sync.Mutex
	options ListSelectCacheOptions
	entries map[[32]byte]*cachedSelect
	flights map[[32]byte]*selectFlight
	stats   SelectCacheStats
}

// EnableListSelectCache must be called before serving requests. Results are
// keyed by exact SQL and typed, driver-normalized parameters inside this DB
// executor. Identical misses are coalesced; errors are never retained.
func (db *DB) EnableListSelectCache(options ListSelectCacheOptions) {
	if options.TTL <= 0 || options.MaxEntries <= 0 || options.MaxBytes <= 0 || options.MaxResultBytes <= 0 {
		return
	}
	db.listSelectCache = &listSelectCache{options: options, entries: make(map[[32]byte]*cachedSelect), flights: make(map[[32]byte]*selectFlight)}
}

func (db *DB) ListSelectCacheStats() SelectCacheStats {
	if db.listSelectCache == nil {
		return SelectCacheStats{}
	}
	c := db.listSelectCache
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.stats
	s.Entries = len(c.entries)
	return s
}

type selectRows interface {
	Err() error
	Next() bool
	Scan(...interface{}) error
	Close() error
}

// Only destinations used by generated List are supported. Nullable field
// types delegate conversion to database/sql's Scanner implementations.
type bufferedSelectRows struct {
	rows  [][]interface{}
	index int
}

func (r *bufferedSelectRows) Err() error   { return nil }
func (r *bufferedSelectRows) Next() bool   { r.index++; return r.index <= len(r.rows) }
func (r *bufferedSelectRows) Close() error { r.rows = nil; return nil }
func (r *bufferedSelectRows) Scan(dest ...interface{}) error {
	if r.index < 1 || r.index > len(r.rows) {
		return fmt.Errorf("Scan without Next")
	}
	row := r.rows[r.index-1]
	if len(row) != len(dest) {
		return fmt.Errorf("Scan destination count mismatch")
	}
	for i, dst := range dest {
		value := cloneSQLValue(row[i])
		if scanner, ok := dst.(sql.Scanner); ok {
			if err := scanner.Scan(value); err != nil {
				return err
			}
			continue
		}
		switch d := dst.(type) {
		case *interface{}:
			*d = value
		case *int64:
			var n sql.NullInt64
			if err := n.Scan(value); err != nil {
				return err
			}
			if !n.Valid {
				return fmt.Errorf("NULL count")
			}
			*d = n.Int64
		case *json.RawMessage:
			switch v := value.(type) {
			case nil:
				*d = nil
			case []byte:
				*d = v
			case string:
				*d = []byte(v)
			default:
				return fmt.Errorf("unsupported JSON scan value %T", value)
			}
		default:
			return fmt.Errorf("unsupported cached scan destination %T", dst)
		}
	}
	return nil
}

func cloneSQLValue(v interface{}) interface{} {
	if b, ok := v.([]byte); ok {
		return append([]byte(nil), b...)
	}
	return v
}

func selectCacheKey(query string, args []interface{}) ([32]byte, error) {
	type argument struct {
		Type  string
		Value interface{}
	}
	values := make([]argument, len(args))
	for i, arg := range args {
		v, err := driver.DefaultParameterConverter.ConvertValue(arg)
		if err != nil {
			return [32]byte{}, err
		}
		values[i] = argument{fmt.Sprintf("%T", v), v}
	}
	encoded, err := json.Marshal(struct {
		SQL  string
		Args []argument
	}{query, values})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func (db *DB) queryCachedList(query string, args ...interface{}) (selectRows, error) {
	c := db.listSelectCache
	if c == nil {
		return db.queryListSQL(query, args...)
	}
	key, err := selectCacheKey(query, args)
	if err != nil {
		return db.queryListSQL(query, args...)
	}
	now := time.Now()
	c.mu.Lock()
	// Lazy cleanup keeps both memory and entry counts bounded without a worker.
	for k, entry := range c.entries {
		if !now.Before(entry.expires) {
			delete(c.entries, k)
			c.stats.Bytes -= entry.bytes
		}
	}
	if entry := c.entries[key]; entry != nil {
		c.stats.Hits++
		c.mu.Unlock()
		return &bufferedSelectRows{rows: entry.rows}, nil
	}
	if flight := c.flights[key]; flight != nil {
		c.stats.Shared++
		c.mu.Unlock()
		<-flight.done
		if flight.err != nil {
			return nil, flight.err
		}
		if flight.result != nil {
			return &bufferedSelectRows{rows: flight.result.rows}, nil
		}
		return db.queryListSQL(query, args...)
	}
	// Bound concurrent materialization as well as retained results.
	if len(c.flights) >= c.options.MaxEntries {
		c.stats.Bypassed++
		c.mu.Unlock()
		return db.queryListSQL(query, args...)
	}
	flight := &selectFlight{done: make(chan struct{})}
	c.flights[key] = flight
	c.stats.Misses++
	c.mu.Unlock()
	result, rows, err := db.captureListSQL(c.options.MaxResultBytes, query, args...)
	completedAt := time.Now()
	c.mu.Lock()
	delete(c.flights, key)
	flight.err = err
	flight.result = result
	if err == nil && result != nil {
		// Give even a slow query a full reuse window after receiving its result.
		result.expires = completedAt.Add(c.options.TTL)
		if time.Now().Before(result.expires) && len(c.entries) < c.options.MaxEntries && c.stats.Bytes+result.bytes <= c.options.MaxBytes {
			c.entries[key] = result
			c.stats.Bytes += result.bytes
		} else {
			c.stats.Bypassed++
		}
	} else if err == nil {
		c.stats.Bypassed++
	}
	close(flight.done)
	c.mu.Unlock()
	return rows, err
}

// An oversized result falls back to streaming its remaining rows, retaining
// only a bounded prefix for this caller and no cache entry.
type prefixedSelectRows struct {
	prefix   *bufferedSelectRows
	tail     *sql.Rows
	inPrefix bool
}

func (r *prefixedSelectRows) Next() bool {
	r.inPrefix = r.prefix.Next()
	if r.inPrefix {
		return true
	}
	return r.tail.Next()
}
func (r *prefixedSelectRows) Scan(dest ...interface{}) error {
	if r.inPrefix {
		return r.prefix.Scan(dest...)
	}
	return r.tail.Scan(dest...)
}
func (r *prefixedSelectRows) Err() error   { return r.tail.Err() }
func (r *prefixedSelectRows) Close() error { r.prefix.Close(); return r.tail.Close() }

func (db *DB) captureListSQL(limit int64, query string, args ...interface{}) (*cachedSelect, selectRows, error) {
	rows, err := db.queryListSQL(query, args...)
	if err != nil {
		return nil, nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, nil, err
	}
	result := &cachedSelect{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		dest := make([]interface{}, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return nil, nil, err
		}
		result.bytes += int64(24 + len(columns)*16)
		for i, v := range values {
			values[i] = cloneSQLValue(v)
			switch v := v.(type) {
			case []byte:
				result.bytes += int64(len(v))
			case string:
				result.bytes += int64(len(v))
			default:
				result.bytes += 24
			}
		}
		result.rows = append(result.rows, values)
		if result.bytes > limit {
			return nil, &prefixedSelectRows{prefix: &bufferedSelectRows{rows: result.rows}, tail: rows}, nil
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	return result, &bufferedSelectRows{rows: result.rows}, nil
}
