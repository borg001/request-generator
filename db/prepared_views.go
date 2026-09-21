package db

import (
	"database/sql"
	"errors"
	"sync"
)

type preparedViewCache struct {
	mu         sync.Mutex
	limit      int
	statements map[string]*sql.Stmt
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
		db.preparedViews = &preparedViewCache{limit: maxStatements, statements: make(map[string]*sql.Stmt)}
	}
	return db
}

func (db *DB) queryView(query string, args ...interface{}) (*sql.Rows, error) {
	if db.preparedViews == nil {
		return db.sql.Query(query, args...)
	}
	stmt, err := db.preparedViews.statement(db.sql, query)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return db.sql.Query(query, args...)
	}
	// sql.Stmt prepared on sql.DB handles concurrent use and preparation on
	// additional/replacement pool connections. Never bind it to one sql.Conn.
	return stmt.Query(args...)
}

func (cache *preparedViewCache) statement(pool *sql.DB, query string) (*sql.Stmt, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if stmt := cache.statements[query]; stmt != nil {
		return stmt, nil
	}
	if len(cache.statements) >= cache.limit {
		return nil, nil
	}
	// Serialize initial preparation so simultaneous cold requests cannot create
	// duplicate retained statements. Failed preparations are not cached.
	stmt, err := pool.Prepare(query)
	if err != nil {
		return nil, err
	}
	cache.statements[query] = stmt
	return stmt, nil
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
