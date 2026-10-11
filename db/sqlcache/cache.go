// Package sqlcache caches complete PostgreSQL read results below database/sql.
// It preserves database/sql scanning and the application's *sql.DB API. Enable
// only on pools with invariant session state whose reads tolerate TTL staleness.
// SQL functions used in cacheable queries must be side-effect free. Transactions,
// locking reads, mutations and known session/sequence functions bypass the cache.
package sqlcache

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Options struct {
	TTL                      time.Duration
	MaxEntries               int
	MaxBytes, MaxResultBytes int64
}

type Stats struct {
	TTL                            time.Duration
	Hits, Misses, Shared, Bypassed uint64
	Entries                        int
	Bytes                          int64
}

type bypassKey struct{}

// WithoutCache is for control-plane reads (for example the cache TTL itself).
func WithoutCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassKey{}, true)
}

type entry struct {
	data    *snapshot
	expires time.Time
}
type flight struct {
	done chan struct{}
	data *snapshot
	err  error
}

// Cache is shared by connections in one pool, never across databases.
type Cache struct {
	mu      sync.Mutex
	options Options
	entries map[[32]byte]entry
	flights map[[32]byte]*flight
	stats   Stats
}

func New(options Options) *Cache {
	return &Cache{options: options, entries: make(map[[32]byte]entry), flights: make(map[[32]byte]*flight)}
}

func (c *Cache) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.stats
	s.TTL = c.options.TTL
	s.Entries = len(c.entries)
	return s
}

// SetTTL changes the global window. Existing entries keep their original expiry;
// a shorter TTL also caps their age at lookup. Zero disables reuse immediately.
// Writes never invalidate entries.
func (c *Cache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	c.options.TTL = ttl
	c.mu.Unlock()
}

func key(query string, args []driver.NamedValue) ([32]byte, error) {
	type argument struct {
		Name, Type string
		Ordinal    int
		Value      interface{}
	}
	values := make([]argument, len(args))
	for i, arg := range args {
		values[i] = argument{arg.Name, fmt.Sprintf("%T", arg.Value), arg.Ordinal, arg.Value}
	}
	encoded, err := json.Marshal(struct {
		SQL  string
		Args []argument
	}{query, values})
	return sha256.Sum256(encoded), err
}

func (c *Cache) query(ctx context.Context, query string, args []driver.NamedValue, eligible bool, run func() (driver.Rows, error)) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	k, keyErr := key(query, args)
	c.mu.Lock()
	opts := c.options
	if !eligible || ctx.Value(bypassKey{}) == true || keyErr != nil || opts.TTL <= 0 || opts.MaxEntries <= 0 || opts.MaxBytes <= 0 || opts.MaxResultBytes <= 0 {
		c.stats.Bypassed++
		c.mu.Unlock()
		return run()
	}
	now := time.Now()
	for k, e := range c.entries {
		if !now.Before(e.expires) || !now.Before(e.data.completed.Add(opts.TTL)) {
			delete(c.entries, k)
			c.stats.Bytes -= e.data.bytes
		}
	}
	if e, ok := c.entries[k]; ok {
		c.stats.Hits++
		c.mu.Unlock()
		return e.data.reader(), nil
	}
	if f := c.flights[k]; f != nil {
		c.stats.Shared++
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
		}
		if f.err != nil {
			return nil, f.err
		}
		if f.data != nil {
			return f.data.reader(), nil
		}
		return run()
	}
	if len(c.flights) >= opts.MaxEntries {
		c.stats.Bypassed++
		c.mu.Unlock()
		return run()
	}
	f := &flight{done: make(chan struct{})}
	c.flights[k] = f
	c.stats.Misses++
	c.mu.Unlock()
	rows, err := run()
	var data *snapshot
	if err == nil {
		data, rows, err = capture(ctx, rows, opts.MaxResultBytes)
	}
	c.mu.Lock()
	delete(c.flights, k)
	f.data, f.err = data, err
	if err == nil && data != nil {
		if len(c.entries) < opts.MaxEntries && c.stats.Bytes+data.bytes <= opts.MaxBytes {
			c.entries[k] = entry{data, data.completed.Add(opts.TTL)}
			c.stats.Bytes += data.bytes
		} else {
			c.stats.Bypassed++
		}
	} else if err == nil {
		c.stats.Bypassed++
	}
	close(f.done)
	c.mu.Unlock()
	return rows, err
}
