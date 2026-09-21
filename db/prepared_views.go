package db

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

type preparedViewCache struct {
	mu         sync.Mutex
	limit      int
	statements map[string]*sql.Stmt
	pending    map[string]chan struct{}
}

// NewDBWithPreparedViews opts this executor into reusing prepared View queries.
// It shares the supplied connection pool; other operations are unchanged.
// Keys are the exact parameterized SQL, never argument values or query results.
// At most maxStatements statements are retained. Additional SQL shapes use the
// ordinary query path without evicting statements that may be in use.
// A nonpositive limit disables preparation. The executor should be long-lived;
// call ClosePreparedViews after its requests have drained to release statements.
func NewDBWithPreparedViews(pool *sql.DB, maxStatements int) *DB {
	db := NewDB(pool)
	if maxStatements > 0 {
		db.preparedViews = &preparedViewCache{limit: maxStatements, statements: make(map[string]*sql.Stmt), pending: make(map[string]chan struct{})}
	}
	return db
}

// NewDBWithPreparedSelects reuses View, List and List COUNT statements. It does
// not cache data, affect mutations or prepare arbitrary SQL passed to Query.
// Keep the executor alive across requests and call ClosePreparedViews after
// draining requests. The same bounded cache is shared by all generated reads.
func NewDBWithPreparedSelects(pool *sql.DB, maxStatements int) *DB {
	db := NewDBWithPreparedViews(pool, maxStatements)
	db.prepareLists = true
	return db
}

func (db *DB) queryList(query string, args ...interface{}) (selectRows, error) {
	// Cached List queries are shared across requests. Keep their existing
	// lifetime independent of the first HTTP caller; never cache its cancellation.
	if db.listSelectCache != nil {
		shared := *db
		shared.readContext = nil
		return shared.queryCachedList(query, args...)
	}
	return db.queryListSQL(query, args...)
}

func (db *DB) queryListSQL(query string, args ...interface{}) (*sql.Rows, error) {
	if db.prepareLists {
		return db.queryView(query, args...)
	}
	return db.sql.QueryContext(db.queryContext(), query, args...)
}

func (db *DB) queryView(query string, args ...interface{}) (*sql.Rows, error) {
	if db.preparedViews == nil {
		return db.sql.QueryContext(db.queryContext(), query, args...)
	}
	stmt, err := db.preparedViews.statement(db.queryContext(), db.sql, query)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return db.sql.QueryContext(db.queryContext(), query, args...)
	}
	// sql.Stmt prepared on sql.DB handles concurrent use and preparation on
	// additional/replacement pool connections. Never bind it to one sql.Conn.
	return stmt.QueryContext(db.queryContext(), args...)
}

func (cache *preparedViewCache) statement(ctx context.Context, pool *sql.DB, query string) (*sql.Stmt, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cache.mu.Lock()
		if stmt := cache.statements[query]; stmt != nil {
			cache.mu.Unlock()
			return stmt, nil
		}
		if ready := cache.pending[query]; ready != nil {
			cache.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ready:
			}
			continue
		}
		if len(cache.statements)+len(cache.pending) >= cache.limit {
			cache.mu.Unlock()
			return nil, nil
		}
		ready := make(chan struct{})
		cache.pending[query] = ready
		cache.mu.Unlock()
		// A cold SQL shape must not hold the cache mutex while acquiring a pool
		// connection. Only requests for the same shape wait for its preparation.
		stmt, err := pool.PrepareContext(ctx, query)
		cache.mu.Lock()
		delete(cache.pending, query)
		if err == nil {
			cache.statements[query] = stmt
		}
		close(ready)
		cache.mu.Unlock()
		return stmt, err
	}
}

// ClosePreparedViews releases retained statements and disables further caching.
// Call after stopping this executor's requests. The shared sql.DB stays open.
func (db *DB) ClosePreparedViews() error {
	cache := db.preparedViews
	if cache == nil {
		return nil
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	var result error
	for query, stmt := range cache.statements {
		result = errors.Join(result, stmt.Close())
		delete(cache.statements, query)
	}
	cache.limit = 0
	return result
}

// WithContext returns an immutable request-scoped copy sharing the pool and
// statement cache. Generated View and uncached List/COUNT honor cancellation.
// Cached List execution keeps its shared lifetime; writes are unchanged.
func (db *DB) WithContext(ctx context.Context) DBExecutor {
	scoped := *db
	scoped.readContext = ctx
	return &scoped
}

func (db *DB) queryContext() context.Context {
	if db.readContext != nil {
		return db.readContext
	}
	return context.Background()
}
