package sqlcache

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"sync"
)

// WrapDB puts result lookup and request coalescing before backend's connection
// budget. The returned *sql.DB owns cheap logical connections only. Configure
// connection limits/lifetimes on backend; leave the logical pool unlimited.
// Close the returned DB before closing backend, which remains caller-owned.
// Both pools must use invariant session defaults. Session SQL is pinned to a
// backend connection and that connection is discarded when its checkout ends.
func (c *Cache) WrapDB(backend *sql.DB) *sql.DB {
	p := &poolConnector{backend: backend, cache: c, statements: make(map[string]*poolStatement)}
	front := sql.OpenDB(p)
	front.SetMaxIdleConns(32)
	return front
}

type poolConnector struct {
	backend    *sql.DB
	cache      *Cache
	mu         sync.Mutex
	statements map[string]*poolStatement
}
type poolStatement struct {
	ready chan struct{}
	stmt  *sql.Stmt
	err   error
	refs  int
}

func (p *poolConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &poolConn{pool: p}, nil
}
func (p *poolConnector) Driver() driver.Driver { return poolDriver{} }

type poolDriver struct{}

func (poolDriver) Open(string) (driver.Conn, error) { return nil, errors.New("sqlcache: use WrapDB") }

// Share prepared statements across logical connections. Preparing the first
// instance still validates SQL on PostgreSQL; subsequent logical connections
// reuse it without checking out a physical connection merely to prepare it.
func (p *poolConnector) prepare(ctx context.Context, query string) (*poolStatement, error) {
	p.mu.Lock()
	if s := p.statements[query]; s != nil {
		s.refs++
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			p.release(query, s)
			return nil, ctx.Err()
		case <-s.ready:
		}
		if s.err != nil {
			p.release(query, s)
			return nil, s.err
		}
		return s, nil
	}
	s := &poolStatement{ready: make(chan struct{}), refs: 1}
	p.statements[query] = s
	p.mu.Unlock()
	stmt, err := p.backend.PrepareContext(ctx, query)
	p.mu.Lock()
	s.stmt, s.err = stmt, err
	close(s.ready)
	p.mu.Unlock()
	if err != nil {
		p.release(query, s)
		return nil, err
	}
	return s, nil
}
func (p *poolConnector) release(query string, s *poolStatement) {
	p.mu.Lock()
	s.refs--
	last := s.refs == 0
	if last && p.statements[query] == s {
		delete(p.statements, query)
	}
	p.mu.Unlock()
	if last && s.stmt != nil {
		_ = s.stmt.Close()
	}
}

type poolConn struct {
	pool   *poolConnector
	pinned *sql.Conn
	tx     *sql.Tx
	dirty  bool
	closed bool
}

func (c *poolConn) pin(ctx context.Context) error {
	if c.pinned != nil {
		return nil
	}
	var err error
	c.pinned, err = c.pool.backend.Conn(ctx)
	return err
}
func (c *poolConn) track(query string) {
	// Use exactly the same session classification as the driver cache.
	probe := conn{}
	probe.trackSession(query)
	c.dirty = c.dirty || probe.sessionChanged
}
func sqlArguments(args []driver.NamedValue) []any {
	values := make([]any, len(args))
	for i, a := range args {
		values[i] = a.Value
		if a.Name != "" {
			values[i] = sql.Named(a.Name, a.Value)
		}
	}
	return values
}
func (c *poolConn) query(ctx context.Context, query string, args []driver.NamedValue, prepared *sql.Stmt, eligible bool) (driver.Rows, error) {
	if !eligible {
		if err := c.pin(ctx); err != nil {
			return nil, err
		}
		c.track(query)
	}
	return c.pool.cache.query(ctx, query, args, eligible && c.pinned == nil && c.tx == nil, func() (driver.Rows, error) {
		values := sqlArguments(args)
		var rows *sql.Rows
		var err error
		switch {
		case c.tx != nil:
			rows, err = c.tx.QueryContext(ctx, query, values...)
		case c.pinned != nil:
			rows, err = c.pinned.QueryContext(ctx, query, values...)
		case prepared != nil:
			rows, err = prepared.QueryContext(ctx, values...)
		default:
			rows, err = c.pool.backend.QueryContext(ctx, query, values...)
		}
		if err != nil {
			return nil, err
		}
		return adaptRows(rows)
	})
}
func (c *poolConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, q, args, nil, readOnly(q))
}
func (c *poolConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.pin(ctx); err != nil {
		return nil, err
	}
	c.track(q)
	if c.tx != nil {
		return c.tx.ExecContext(ctx, q, sqlArguments(args)...)
	}
	return c.pinned.ExecContext(ctx, q, sqlArguments(args)...)
}
func (c *poolConn) Prepare(q string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), q)
}
func (c *poolConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	var stmt *sql.Stmt
	var err error
	if c.tx != nil {
		stmt, err = c.tx.PrepareContext(ctx, q)
	} else if c.pinned != nil {
		stmt, err = c.pinned.PrepareContext(ctx, q)
	} else {
		s, err := c.pool.prepare(ctx, q)
		if err != nil {
			return nil, err
		}
		return &logicalStatement{conn: c, query: q, shared: s, eligible: readOnly(q)}, nil
	}
	if err != nil {
		return nil, err
	}
	return &logicalStatement{conn: c, query: q, owned: stmt, eligible: readOnly(q)}, nil
}
func (c *poolConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *poolConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.pin(ctx); err != nil {
		return nil, err
	}
	tx, err := c.pinned.BeginTx(ctx, &sql.TxOptions{Isolation: sql.IsolationLevel(opts.Isolation), ReadOnly: opts.ReadOnly})
	if err != nil {
		return nil, err
	}
	c.tx = tx
	return &logicalTransaction{conn: c, tx: tx}, nil
}

type logicalTransaction struct {
	conn *poolConn
	tx   *sql.Tx
}

func (t *logicalTransaction) Commit() error { defer func() { t.conn.tx = nil }(); return t.tx.Commit() }
func (t *logicalTransaction) Rollback() error {
	defer func() { t.conn.tx = nil }()
	return t.tx.Rollback()
}

// database/sql calls IsValid when returning a logical checkout (including
// explicit sql.Conn and Tx), not between operations on a checked-out sql.Conn.
func (c *poolConn) release() {
	if c.tx != nil {
		_ = c.tx.Rollback()
		c.tx = nil
	}
	if c.pinned != nil {
		if c.dirty {
			_ = c.pinned.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = c.pinned.Close()
		c.pinned = nil
	}
	c.dirty = false
}
func (c *poolConn) IsValid() bool                          { c.release(); return !c.closed }
func (c *poolConn) ResetSession(ctx context.Context) error { return ctx.Err() }
func (c *poolConn) Close() error                           { c.release(); c.closed = true; return nil }
func (c *poolConn) Ping(ctx context.Context) error {
	if c.pinned != nil {
		return c.pinned.PingContext(ctx)
	}
	return c.pool.backend.PingContext(ctx)
}
func (c *poolConn) CheckNamedValue(*driver.NamedValue) error { return driver.ErrSkip }

type logicalStatement struct {
	eligible bool
	conn     *poolConn
	query    string
	shared   *poolStatement
	owned    *sql.Stmt
}

func (s *logicalStatement) Close() error {
	if s.shared != nil {
		s.conn.pool.release(s.query, s.shared)
		s.shared = nil
	}
	if s.owned != nil {
		return s.owned.Close()
	}
	return nil
}
func (s *logicalStatement) NumInput() int { return -1 }
func (s *logicalStatement) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), named(args))
}
func (s *logicalStatement) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), named(args))
}
func (s *logicalStatement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	var prepared *sql.Stmt
	if s.shared != nil {
		prepared = s.shared.stmt
	}
	return s.conn.query(ctx, s.query, args, prepared, s.eligible)
}
func (s *logicalStatement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.conn.ExecContext(ctx, s.query, args)
}

type sqlRows struct {
	rows                   *sql.Rows
	columns                []string
	types                  []*sql.ColumnType
	values, targets        []any
	eof, nextChecked, next bool
	advanceErr             error
}

func adaptRows(rows *sql.Rows) (driver.Rows, error) {
	r := &sqlRows{rows: rows}
	if err := r.describe(); err != nil {
		rows.Close()
		return nil, err
	}
	return r, nil
}
func (r *sqlRows) describe() error {
	var err error
	r.columns, err = r.rows.Columns()
	if err != nil {
		return err
	}
	r.types, err = r.rows.ColumnTypes()
	if err != nil {
		return err
	}
	r.values = make([]any, len(r.columns))
	r.targets = make([]any, len(r.columns))
	for i := range r.values {
		r.targets[i] = &r.values[i]
	}
	return nil
}
func (r *sqlRows) Columns() []string { return r.columns }
func (r *sqlRows) Close() error      { return errors.Join(r.advanceErr, r.rows.Close()) }
func (r *sqlRows) Next(dest []driver.Value) error {
	if r.eof {
		return io.EOF
	}
	if !r.rows.Next() {
		r.eof = true
		if err := r.rows.Err(); err != nil {
			return err
		}
		return io.EOF
	}
	if err := r.rows.Scan(r.targets...); err != nil {
		return err
	}
	for i, v := range r.values {
		dest[i] = v
	}
	return nil
}

// sql.Rows exposes NextResultSet but no non-advancing HasNextResultSet. Probe
// once and retain the next set's metadata until the driver's advance call.
func (r *sqlRows) HasNextResultSet() bool {
	if !r.nextChecked {
		r.next = r.rows.NextResultSet()
		r.nextChecked = true
		r.advanceErr = r.rows.Err()
	}
	return r.next
}
func (r *sqlRows) NextResultSet() error {
	if !r.HasNextResultSet() {
		if r.advanceErr != nil {
			return r.advanceErr
		}
		return io.EOF
	}
	r.eof, r.nextChecked, r.next = false, false, false
	return r.describe()
}
func (r *sqlRows) ColumnTypeDatabaseTypeName(i int) string { return r.types[i].DatabaseTypeName() }
func (r *sqlRows) ColumnTypeScanType(i int) reflect.Type   { return r.types[i].ScanType() }
func (r *sqlRows) ColumnTypeLength(i int) (int64, bool)    { return r.types[i].Length() }
func (r *sqlRows) ColumnTypeNullable(i int) (bool, bool)   { return r.types[i].Nullable() }
func (r *sqlRows) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	return r.types[i].DecimalSize()
}
