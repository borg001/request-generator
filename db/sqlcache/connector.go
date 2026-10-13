package sqlcache

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type connector struct {
	driver.Connector
	cache *Cache
}

// Connector wraps a context-aware driver (including lib/pq). The cache is shared
// across physical connections, so session-dependent SQL must use another pool.
func (c *Cache) Connector(base driver.Connector) driver.Connector { return &connector{base, c} }

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	base, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := base.(contextConn); !ok {
		base.Close()
		return nil, fmt.Errorf("sqlcache: driver must support context queries, exec, prepare and transactions")
	}
	return &conn{Conn: base, cache: c.cache}, nil
}

type contextConn interface {
	driver.QueryerContext
	driver.ExecerContext
	driver.ConnPrepareContext
	driver.ConnBeginTx
}
type conn struct {
	driver.Conn
	cache                *Cache
	inTx, sessionChanged bool
}

func (c *conn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	eligible := readOnly(q)
	if !eligible {
		c.trackSession(q)
	}
	return c.cache.query(ctx, q, args, eligible && !c.inTx && !c.sessionChanged, func() (driver.Rows, error) {
		return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
	})
}
func (c *conn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.trackSession(q)
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}
func (c *conn) trackSession(q string) {
	first := true
	valid := scanSQLTokens(q, func(token string) {
		flags := tokenFlags(token)
		if flags&sessionToken != 0 || first && flags&sessionHead != 0 {
			c.sessionChanged = true
		}
		first = false
	})
	if !valid {
		c.sessionChanged = true
	}
}
func (c *conn) Prepare(q string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), q)
}
func (c *conn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	s, err := c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, q)
	if err != nil {
		return nil, err
	}
	return &stmt{Stmt: s, conn: c, query: q, eligible: readOnly(q)}, nil
}
func (c *conn) Begin() (driver.Tx, error) { return c.BeginTx(context.Background(), driver.TxOptions{}) }
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	t, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	c.inTx = true
	return &tx{Tx: t, conn: c}, nil
}
func (c *conn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}
func (c *conn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}
func (c *conn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}
func (c *conn) CheckNamedValue(v *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(v)
	}
	return driver.ErrSkip
}

type tx struct {
	driver.Tx
	conn *conn
}

func (t *tx) Commit() error   { defer func() { t.conn.inTx = false }(); return t.Tx.Commit() }
func (t *tx) Rollback() error { defer func() { t.conn.inTx = false }(); return t.Tx.Rollback() }

type stmt struct {
	driver.Stmt
	conn     *conn
	query    string
	eligible bool
}

func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if !s.eligible {
		s.conn.trackSession(s.query)
	}
	return s.conn.cache.query(ctx, s.query, args, s.eligible && !s.conn.inTx && !s.conn.sessionChanged, func() (driver.Rows, error) {
		if query, ok := s.Stmt.(driver.StmtQueryContext); ok {
			return query.QueryContext(ctx, args)
		}
		values, err := legacyArgs(ctx, args)
		if err != nil {
			return nil, err
		}
		return s.Stmt.Query(values)
	})
}
func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	s.conn.trackSession(s.query)
	if exec, ok := s.Stmt.(driver.StmtExecContext); ok {
		return exec.ExecContext(ctx, args)
	}
	values, err := legacyArgs(ctx, args)
	if err != nil {
		return nil, err
	}
	return s.Stmt.Exec(values)
}
func named(args []driver.Value) []driver.NamedValue {
	values := make([]driver.NamedValue, len(args))
	for i, v := range args {
		values[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return values
}
func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), named(args))
}
func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), named(args))
}
func (s *stmt) CheckNamedValue(v *driver.NamedValue) error {
	if checker, ok := s.Stmt.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(v)
	}
	return s.conn.CheckNamedValue(v)
}
func (s *stmt) ColumnConverter(i int) driver.ValueConverter {
	if converter, ok := s.Stmt.(driver.ColumnConverter); ok {
		return converter.ColumnConverter(i)
	}
	return driver.DefaultParameterConverter
}

func legacyArgs(ctx context.Context, args []driver.NamedValue) ([]driver.Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			return nil, fmt.Errorf("sqlcache: named parameters unsupported by legacy statement")
		}
		values[i] = arg.Value
	}
	return values, nil
}
