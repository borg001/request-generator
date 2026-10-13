package sqlcache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
)

func postgresPool(t *testing.T, limit int) (*Cache, *sql.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("PREPARED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PREPARED_TEST_DATABASE_URL not set")
	}
	base, err := pq.NewConnector(dsn)
	if err != nil {
		t.Fatal(err)
	}
	backend := sql.OpenDB(base)
	backend.SetMaxOpenConns(limit)
	backend.SetMaxIdleConns(limit)
	c := New(Options{TTL: 3 * time.Second, MaxEntries: 64, MaxBytes: 1 << 20, MaxResultBytes: 1 << 16})
	front := c.WrapDB(backend)
	t.Cleanup(func() { front.Close(); backend.Close() })
	return c, front, backend
}

func TestPoolHitsDoNotWaitForPhysicalConnection(t *testing.T) {
	c, front, backend := postgresPool(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stmt, err := front.PrepareContext(ctx, "SELECT $1::bigint")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	var got int
	if err := stmt.QueryRowContext(ctx, 42).Scan(&got); err != nil {
		t.Fatal(err)
	}
	held, err := backend.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	before := backend.Stats()
	for _, read := range []func(context.Context) *sql.Row{
		func(ctx context.Context) *sql.Row { return front.QueryRowContext(ctx, "SELECT $1::bigint", int64(42)) },
		func(ctx context.Context) *sql.Row { return stmt.QueryRowContext(ctx, int64(42)) },
	} {
		short, stop := context.WithTimeout(ctx, 200*time.Millisecond)
		err := read(short).Scan(&got)
		stop()
		if err != nil || got != 42 {
			t.Fatalf("cached read waited for backend: got=%d err=%v", got, err)
		}
	}
	if c.Stats().Hits < 2 || backend.Stats().WaitCount != before.WaitCount {
		t.Fatalf("cache hit borrowed backend: %+v %+v", c.Stats(), backend.Stats())
	}
	miss, stop := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stop()
	if err := front.QueryRowContext(miss, "SELECT 99").Scan(&got); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestPoolCoalescedWaitersDoNotOccupyBackend(t *testing.T) {
	c, front, backend := postgresPool(t, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A volatile SQL function deliberately represents one read in flight. Its
	// non-session query is cacheable under the caller's existing policy contract.
	_, err := backend.ExecContext(ctx, `CREATE OR REPLACE FUNCTION cache_pool_slow() RETURNS integer LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.25); RETURN 42; END $$`)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Exec("DROP FUNCTION cache_pool_slow()")
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var n int
			if err := front.QueryRowContext(ctx, "SELECT cache_pool_slow()").Scan(&n); err != nil || n != 42 {
				t.Errorf("read %d: %v", n, err)
			}
		}()
	}
	deadline := time.Now().Add(time.Second)
	for c.Stats().Shared < 10 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if c.Stats().Shared < 10 {
		t.Fatal("did not observe waiting readers")
	}
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	var n int
	err = front.QueryRowContext(short, "SELECT 7").Scan(&n)
	stop()
	if err != nil || n != 7 {
		t.Errorf("coalesced followers occupied backend: %v", err)
	}
	wg.Wait()
	if backend.Stats().WaitCount != 0 || backend.Stats().OpenConnections > 2 {
		t.Fatal(backend.Stats())
	}
}

func TestPoolSessionStateAndTransactionsStayPinned(t *testing.T) {
	_, front, backend := postgresPool(t, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := front.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "SET application_name='cache_pool_private'"); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := conn.QueryRowContext(ctx, "SELECT current_setting('application_name')").Scan(&name); err != nil || name != "cache_pool_private" {
		t.Fatal(name, err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "CREATE TEMP TABLE cache_pool_temp (n int)"); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO cache_pool_temp VALUES (7)"); err != nil {
		t.Fatal(err)
	}
	stmt, err := tx.PrepareContext(ctx, "SELECT n FROM cache_pool_temp")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := stmt.QueryRowContext(ctx).Scan(&n); err != nil || n != 7 {
		t.Fatal(n, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, "SELECT n FROM cache_pool_temp").Scan(&n); err != nil || n != 7 {
		t.Fatal(n, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := backend.QueryRowContext(ctx, "SELECT current_setting('application_name')").Scan(&name); err != nil || name == "cache_pool_private" {
		t.Fatal(name, err)
	}
	if err := backend.QueryRowContext(ctx, "SELECT n FROM cache_pool_temp").Scan(&n); err == nil {
		t.Fatal("temporary session leaked into backend pool")
	}
}

func TestPoolStreamsLargeAndMultipleResults(t *testing.T) {
	c, front, _ := postgresPool(t, 1)
	c.options.MaxResultBytes = 256
	for pass := 0; pass < 2; pass++ {
		rows, err := front.Query("SELECT repeat('x',1024),n FROM generate_series(1,8) n")
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for rows.Next() {
			var s string
			var n int
			if err := rows.Scan(&s, &n); err != nil || len(s) != 1024 {
				t.Fatal(err)
			}
			count++
		}
		if err := rows.Err(); err != nil || count != 8 {
			t.Fatal(count, err)
		}
		rows.Close()
		rows, err = front.Query("SELECT 1 AS first; SELECT 2 AS second; SELECT 3 AS third")
		if err != nil {
			t.Fatal(err)
		}
		for want := 1; want <= 3; want++ {
			if !rows.Next() {
				t.Fatal("missing row", want, rows.Err())
			}
			var got int
			if err := rows.Scan(&got); err != nil || got != want {
				t.Fatal(got, err)
			}
			if rows.Next() {
				t.Fatal("extra row")
			}
			if next := rows.NextResultSet(); next != (want < 3) {
				t.Fatal("result set", want, next, rows.Err())
			}
		}
		rows.Close()
	}
	if c.Stats().Entries != 0 {
		t.Fatal("oversized result retained")
	}
	if _, err := front.Prepare("SELECT missing_column FROM definitely_missing_table"); err == nil {
		t.Fatal("Prepare lost validation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	var n int
	if err := front.QueryRowContext(ctx, "SELECT 1 FROM pg_sleep(1)").Scan(&n); err == nil {
		t.Fatal("cancellation lost")
	}
	if err := front.QueryRow("SELECT 8").Scan(&n); err != nil || n != 8 {
		t.Fatal(fmt.Sprint(n), err)
	}
}

func TestPoolCanceledLeaderRetriesOnLiveConnection(t *testing.T) {
	cache, front, backend := postgresPool(t, 1)
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	_, err := backend.ExecContext(ctx, `CREATE OR REPLACE FUNCTION cache_cancel_slow() RETURNS integer LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.3); RETURN 42; END $$`)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Exec("DROP FUNCTION cache_cancel_slow()")
	leader, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan error, 9)
	read := func(ctx context.Context) {
		var n int
		err := front.QueryRowContext(ctx, "SELECT cache_cancel_slow()").Scan(&n)
		if err == nil && n != 42 {
			err = fmt.Errorf("unexpected value %d", n)
		}
		finished <- err
	}
	go read(leader)
	deadline := time.Now().Add(time.Second)
	for cache.Stats().Misses < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 8; i++ {
		go read(ctx)
	}
	for cache.Stats().Shared < 8 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	shared := cache.Stats().Shared
	cancel()
	failures := 0
	for i := 0; i < 9; i++ {
		if err := <-finished; err != nil {
			failures++
		}
	}
	if shared != 8 || failures != 1 {
		t.Fatalf("shared=%d failures=%d; only the canceled caller must fail", shared, failures)
	}
	if cache.Stats().Misses != 2 || backend.Stats().OpenConnections > 1 {
		t.Fatalf("retry did not coalesce within physical limit: %+v %+v", cache.Stats(), backend.Stats())
	}
}
