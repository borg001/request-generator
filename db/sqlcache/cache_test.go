package sqlcache

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestReadOnly(t *testing.T) {
	for _, q := range []string{"SELECT * FROM x WHERE id=$1", "/* x /* nested */ */ WITH x AS (SELECT 1) SELECT * FROM x; -- end", "SELECT 'UPDATE; nextval', \"value\" FROM x"} {
		if !readOnly(q) {
			t.Errorf("read rejected: %s", q)
		}
	}
	for _, q := range []string{"UPDATE x SET a=1 RETURNING *", "WITH changed AS (DELETE FROM x RETURNING *) SELECT * FROM changed", "SELECT * FROM x FOR UPDATE", "SELECT * FROM x FOR SHARE", "SELECT 1; DELETE FROM x", "SELECT nextval('seq')", "SELECT \"nextval\"('seq')", "SELECT pg_advisory_lock(1)", "SELECT set_config('role','x',false)", "SELECT 1 INTO new_table", "SELECT $$ a $$", "SELECT 'unterminated", "SELECT random()"} {
		if readOnly(q) {
			t.Errorf("unsafe query accepted: %s", q)
		}
	}
}

type testRows struct {
	n      int
	err    error
	closed bool
}

func (*testRows) Columns() []string { return []string{"value"} }
func (r *testRows) Close() error    { r.closed = true; return nil }
func (r *testRows) Next(dest []driver.Value) error {
	if r.err != nil {
		return r.err
	}
	if r.n == 0 {
		return io.EOF
	}
	r.n--
	dest[0] = []byte("payload")
	return nil
}
func testCache() *Cache {
	return New(Options{TTL: time.Second, MaxEntries: 16, MaxBytes: 1 << 20, MaxResultBytes: 1 << 16})
}

func TestCoalescingCancellationCopiesAndErrors(t *testing.T) {
	c := testCache()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	run := func() (driver.Rows, error) { calls.Add(1); close(started); <-release; return &testRows{n: 1}, nil }
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := c.query(context.Background(), "SELECT value", nil, true, run)
		if err != nil {
			t.Error(err)
			return
		}
		r.Close()
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.query(ctx, "SELECT value", nil, true, run); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("canceled waiter: %v", err)
	}
	close(release)
	wg.Wait()
	for i := 0; i < 2; i++ {
		r, err := c.query(context.Background(), "SELECT value", nil, true, run)
		if err != nil {
			t.Fatal(err)
		}
		dest := make([]driver.Value, 1)
		if err := r.Next(dest); err != nil {
			t.Fatal(err)
		}
		b := dest[0].([]byte)
		if string(b) != "payload" {
			t.Fatalf("shared bytes mutated: %q", b)
		}
		b[0] = 'X'
		r.Close()
	}
	if calls.Load() != 1 {
		t.Fatal("identical query executed again")
	}
	for i := 0; i < 2; i++ {
		if _, err := c.query(context.Background(), "SELECT broken", nil, true, func() (driver.Rows, error) { return &testRows{err: io.ErrUnexpectedEOF}, nil }); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
	}
	if c.Stats().Misses != 3 {
		t.Fatal(c.Stats())
	}
}

func TestBoundsStreamingAndDisabledTTL(t *testing.T) {
	c := New(Options{TTL: time.Second, MaxEntries: 1, MaxBytes: 1024, MaxResultBytes: 300})
	for i := 0; i < 2; i++ {
		underlying := &testRows{n: 20}
		r, err := c.query(context.Background(), "SELECT value", nil, true, func() (driver.Rows, error) { return underlying, nil })
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for r.Next(make([]driver.Value, 1)) == nil {
			count++
		}
		r.Close()
		if count != 20 || !underlying.closed || c.Stats().Entries != 0 {
			t.Fatalf("streaming failed: count=%d stats=%+v", count, c.Stats())
		}
	}
	c = testCache()
	run := func() (driver.Rows, error) { return &testRows{n: 1}, nil }
	r, _ := c.query(context.Background(), "SELECT value", nil, true, run)
	r.Close()
	c.SetTTL(0)
	r, _ = c.query(context.Background(), "SELECT value", nil, true, run)
	r.Close()
	if c.Stats().Hits != 0 || c.Stats().Bypassed != 1 {
		t.Fatal(c.Stats())
	}
}

func TestPostgresPoolReadsWritesTransactionsAndTypes(t *testing.T) {
	dsn := os.Getenv("PREPARED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PREPARED_TEST_DATABASE_URL not set")
	}
	base, err := pq.NewConnector(dsn)
	if err != nil {
		t.Fatal(err)
	}
	c := New(Options{TTL: 250 * time.Millisecond, MaxEntries: 64, MaxBytes: 1 << 20, MaxResultBytes: 1 << 16})
	pool := sql.OpenDB(c.Connector(base))
	defer pool.Close()
	pool.SetMaxOpenConns(4)
	table := fmt.Sprintf("sqlcache_test_%d", time.Now().UnixNano())
	if _, err := pool.Exec("CREATE TABLE " + table + " (id bigint primary key, value text, data bytea, nums bigint[], stamp timestamptz, amount numeric(8,2))"); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec("DROP TABLE " + table)
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := pool.Exec("INSERT INTO "+table+" VALUES ($1,$2,$3,$4,$5,$6), (2,'second',NULL,NULL,NULL,NULL)", 1, "before", []byte("abc"), pq.Array([]int64{1, 2}), stamp, "12.34"); err != nil {
		t.Fatal(err)
	}
	q := "SELECT value FROM " + table + " WHERE id=$1"
	read := func(want string, row *sql.Row) {
		t.Helper()
		var value string
		if err := row.Scan(&value); err != nil || value != want {
			t.Fatalf("want %s got %s err=%v", want, value, err)
		}
	}
	read("before", pool.QueryRow(q, 1))
	read("second", pool.QueryRowContext(context.Background(), q, 2))
	prepared, err := pool.Prepare(q)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	read("before", prepared.QueryRow(1))
	if _, err := pool.Exec("UPDATE " + table + " SET value='after' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	read("before", pool.QueryRow(q, 1)) // No invalidation after a write.
	tx, err := pool.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	read("after", tx.QueryRow(q, 1))
	read("after", tx.Stmt(prepared).QueryRow(1))
	if _, err := tx.Exec("UPDATE " + table + " SET value='uncommitted' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	read("uncommitted", tx.QueryRow(q, 1))
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	read("before", pool.QueryRow(q, 1))
	if c.Stats().Hits < 3 {
		t.Fatal(c.Stats())
	}
	time.Sleep(270 * time.Millisecond)
	read("after", prepared.QueryRow(1))
	for i := 0; i < 2; i++ {
		var data []byte
		var nums []int64
		var gotTime time.Time
		var amount string
		var value sql.NullString
		r, err := pool.Query("SELECT value,data,nums,stamp,amount FROM " + table + " WHERE id=1")
		if err != nil {
			t.Fatal(err)
		}
		types, err := r.ColumnTypes()
		if err != nil || types[1].DatabaseTypeName() != "BYTEA" {
			t.Fatalf("metadata: %v %v", types, err)
		}
		if !r.Next() {
			t.Fatal(r.Err())
		}
		if err := r.Scan(&value, &data, pq.Array(&nums), &gotTime, &amount); err != nil {
			t.Fatal(err)
		}
		r.Close()
		if value.String != "after" || string(data) != "abc" || len(nums) != 2 || !gotTime.Equal(stamp) || amount != "12.34" {
			t.Fatal(value, data, nums, gotTime, amount)
		}
		data[0] = 'X'
	}
	for i := 0; i < 2; i++ {
		var id int
		if err := pool.QueryRow("INSERT INTO "+table+" (id,value) VALUES ($1,'new') RETURNING id", 10+i).Scan(&id); err != nil || id != 10+i {
			t.Fatal(id, err)
		}
	}
	for i := 0; i < 2; i++ {
		var value string
		if err := pool.QueryRow(q, 999).Scan(&value); !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
	}
	c.SetTTL(0)
	read("after", pool.QueryRow(q, 1))
}

// A read containing a regex must not permanently disable caching on the pool
// connection. Feed queries use this ordinary PostgreSQL string syntax.
func TestRegexReadDoesNotChangeSession(t *testing.T) {
	for _, query := range []string{
		`SELECT value FROM records WHERE value ~ '^[0-9]+(\\.[0-9]+)?$'`,
		`SELECT E'escaped\'quote'`,
	} {
		if !readOnly(query) {
			t.Fatalf("read bypassed: %s", query)
		}
		c := &conn{}
		c.trackSession(query)
		c.trackSession("UPDATE records SET value=$1")
		if c.sessionChanged {
			t.Fatalf("read/write marked session dirty: %s", query)
		}
	}
	if readOnly(`SELECT E'escaped\'quote'; DELETE FROM records`) {
		t.Fatal("escaped literal hid a second statement")
	}
	c := &conn{}
	c.trackSession(`SELECT set_config('search_path','other',false)`)
	if !c.sessionChanged {
		t.Fatal("session change not tracked")
	}
}
