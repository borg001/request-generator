package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func testListCache(db *DB, ttl time.Duration, limit int64) {
	db.EnableListSelectCache(ListSelectCacheOptions{TTL: ttl, MaxEntries: 4, MaxBytes: 4096, MaxResultBytes: limit})
}

func TestListSelectCacheTTLStartsAfterSlowResult(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	db := NewDB(pool)
	testListCache(db, 200*time.Millisecond, 1024)
	read := func() int64 {
		rows, err := db.queryList("SELECT slow_result")
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var value int64
		require.NoError(t, rows.Scan(&value))
		return value
	}
	// The query itself takes longer than TTL; its result must still be cached.
	mock.ExpectQuery("SELECT slow_result").WillDelayFor(250 * time.Millisecond).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1)).RowsWillBeClosed()
	require.EqualValues(t, 1, read())
	require.EqualValues(t, 1, read())
	require.EqualValues(t, 1, db.ListSelectCacheStats().Hits)
	// Reuse must stop after TTL from completion, without extending on a hit.
	time.Sleep(210 * time.Millisecond)
	mock.ExpectQuery("SELECT slow_result").
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2)).RowsWillBeClosed()
	require.EqualValues(t, 2, read())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSelectCacheParametersTTLAndCopies(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	db := NewDB(pool)
	testListCache(db, 60*time.Millisecond, 1024)
	read := func(actor int) []byte {
		rows, err := db.queryList("SELECT payload WHERE owner=$1", actor)
		require.NoError(t, err)
		defer rows.Close()
		require.True(t, rows.Next())
		var result interface{}
		require.NoError(t, rows.Scan(&result))
		return result.([]byte)
	}
	mock.ExpectQuery("SELECT payload").WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow([]byte("abc")))
	first := read(10)
	first[0] = 'x'
	require.Equal(t, []byte("abc"), read(10))
	mock.ExpectQuery("SELECT payload").WithArgs(20).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow([]byte("def")))
	require.Equal(t, []byte("def"), read(20))
	time.Sleep(70 * time.Millisecond)
	mock.ExpectQuery("SELECT payload").WithArgs(10).WillReturnRows(sqlmock.NewRows([]string{"payload"}).AddRow([]byte("updated")))
	require.Equal(t, []byte("updated"), read(10))
	require.EqualValues(t, 1, db.ListSelectCacheStats().Hits)
	require.NoError(t, mock.ExpectationsWereMet())
	stringKey, err := selectCacheKey("SELECT $1", []interface{}{"abc"})
	require.NoError(t, err)
	bytesKey, err := selectCacheKey("SELECT $1", []interface{}{[]byte("abc")})
	require.NoError(t, err)
	require.NotEqual(t, stringKey, bytesKey)
}

func TestListSelectCacheCoalescesAndDoesNotRetainErrors(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	db := NewDB(pool)
	testListCache(db, time.Second, 1024)
	mock.ExpectQuery("SELECT 1").WillDelayFor(80 * time.Millisecond).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r, e := db.queryList("SELECT 1")
			if e == nil {
				e = r.Close()
			}
			errs <- e
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	require.Positive(t, db.ListSelectCacheStats().Shared)
	queryErr := errors.New("query failed")
	mock.ExpectQuery("SELECT 2").WillReturnError(queryErr)
	_, err = db.queryList("SELECT 2")
	require.ErrorIs(t, err, queryErr)
	mock.ExpectQuery("SELECT 2").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2))
	r, err := db.queryList("SELECT 2")
	require.NoError(t, err)
	r.Close()
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSelectCacheOversizeStreamingAndScanTypes(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	db := NewDB(pool)
	testListCache(db, time.Second, 1)
	for attempt := 0; attempt < 2; attempt++ {
		mock.ExpectQuery("SELECT data").WillReturnRows(sqlmock.NewRows([]string{"n", "s", "j"}).AddRow(1, "one", []byte(`[1]`)).AddRow(2, nil, []byte(`[2]`))).RowsWillBeClosed()
		r, err := db.queryList("SELECT data")
		require.NoError(t, err)
		for i := 1; i <= 2; i++ {
			require.True(t, r.Next())
			var n int64
			var s sql.NullString
			var j json.RawMessage
			require.NoError(t, r.Scan(&n, &s, &j))
			require.EqualValues(t, i, n)
			require.Equal(t, i == 1, s.Valid)
		}
		require.False(t, r.Next())
		require.NoError(t, r.Close())
	}
	require.Zero(t, db.ListSelectCacheStats().Entries)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSelectCacheRowErrorsAndViewBypass(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	db := NewDB(pool)
	testListCache(db, time.Second, 1024)
	for i := 0; i < 2; i++ {
		mock.ExpectQuery("SELECT broken").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1).RowError(0, errors.New("read failed"))).RowsWillBeClosed()
		_, err = db.queryList("SELECT broken")
		require.Error(t, err)
		mock.ExpectQuery("SELECT view").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
		r, err := db.queryView("SELECT view")
		require.NoError(t, err)
		r.Close()
	}
	require.Zero(t, db.ListSelectCacheStats().Entries)
	require.NoError(t, mock.ExpectationsWereMet())
}
