package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	pg "github.com/go-jet/jet/v2/postgres"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestReadCancellationWhileWaitingForPool(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "prepared"}[prepared], func(t *testing.T) {
			pool, _, err := sqlmock.New()
			require.NoError(t, err)
			defer pool.Close()
			pool.SetMaxOpenConns(1)
			held, err := pool.Conn(context.Background())
			require.NoError(t, err)
			defer held.Close()
			executor := NewDB(pool)
			if prepared {
				executor = NewDBWithPreparedViews(pool, 8)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			scoped := executor.WithContext(ctx).(*DB)
			_, err = scoped.queryView("SELECT 1")
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Nil(t, executor.readContext)
			require.EqualValues(t, 1, pool.Stats().WaitCount)
			if prepared {
				require.Empty(t, executor.preparedViews.pending)
			}
		})
	}
}

func TestPreparedWaiterCancellationDoesNotCancelOwner(t *testing.T) {
	pool, _, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	executor := NewDBWithPreparedViews(pool, 8)
	ready := make(chan struct{})
	executor.preparedViews.pending["SELECT 1"] = ready
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := executor.WithContext(ctx).(*DB).queryView("SELECT 1"); done <- err }()
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("waiter did not cancel")
	}
	require.Equal(t, ready, executor.preparedViews.pending["SELECT 1"])
}

func TestReadCancellationPostgres(t *testing.T) {
	dsn := os.Getenv("PREPARED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PREPARED_TEST_DATABASE_URL not set")
	}
	pool, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	for _, prepared := range []bool{false, true} {
		executor := NewDB(pool)
		if prepared {
			executor = NewDBWithPreparedViews(pool, 8)
		}
		for _, view := range []bool{false, true} {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			scoped := executor.WithContext(ctx).(*DB)
			start := time.Now()
			if view {
				_, err = scoped.queryView("SELECT pg_sleep(2)")
			} else {
				_, err = pool.QueryContext(scoped.queryContext(), "SELECT pg_sleep(2)")
			}
			cancel()
			require.Error(t, err)
			require.Less(t, time.Since(start), time.Second)
			require.True(t, errors.Is(ctx.Err(), context.DeadlineExceeded))
			var value int
			require.NoError(t, pool.QueryRow("SELECT 42").Scan(&value))
			require.Equal(t, 42, value)
		}
		require.NoError(t, executor.ClosePreparedViews())
	}
}

// A cancelled stream/count must never become a successful partial page.
func TestReadErrorsAreNotSuccessfulPages(t *testing.T) {
	for _, stage := range []string{"list-stream", "count-query", "count-stream", "count-scan", "view-stream"} {
		t.Run(stage, func(t *testing.T) {
			pool, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer pool.Close()
			executor := NewDB(pool)
			id := pg.IntegerColumn("id")
			table := pg.NewTable("", "items", "", id)
			failure := context.Canceled
			main := sqlmock.NewRows([]string{"id"}).AddRow(1)
			if stage == "list-stream" || stage == "view-stream" {
				main.RowError(0, failure)
			}
			mock.ExpectQuery("SELECT").WillReturnRows(main).RowsWillBeClosed()
			if stage == "view-stream" {
				result, err := executor.View(log.NewEntry(log.New()), table, id, nil, nil, nil, nil)
				require.ErrorIs(t, err, failure)
				require.Nil(t, result)
			} else {
				if stage == "count-query" {
					mock.ExpectQuery("SELECT").WillReturnError(failure)
				}
				if stage == "count-stream" {
					mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1).RowError(0, failure))
				}
				if stage == "count-scan" {
					mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow("invalid"))
				}
				result, _, err := executor.List(log.NewEntry(log.New()), table, id, nil, nil, 1, 10, nil, "", nil, nil, nil, nil, nil)
				require.Error(t, err)
				if stage != "count-scan" {
					require.ErrorIs(t, err, failure)
				}
				require.Nil(t, result)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
