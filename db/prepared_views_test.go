package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/darkrain/request-generator/fields"
	pg "github.com/go-jet/jet/v2/postgres"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestPreparedViewsBoundedReuseAndFailures(t *testing.T) {
	pool, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	db := NewDBWithPreparedViews(pool, 1)
	query := "SELECT $1::bigint"
	expected := regexp.QuoteMeta(query)
	prepareErr := errors.New("prepare failed")
	mock.ExpectPrepare(expected).WillReturnError(prepareErr)
	_, err = db.queryView(query, 1)
	require.ErrorIs(t, err, prepareErr)

	stmt := mock.ExpectPrepare(expected).WillBeClosed()
	for _, value := range []int64{12, 34} {
		stmt.ExpectQuery().WithArgs(value).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(value)).RowsWillBeClosed()
		rows, err := db.queryView(query, value)
		require.NoError(t, err)
		require.True(t, rows.Next())
		var actual int64
		require.NoError(t, rows.Scan(&actual))
		require.Equal(t, value, actual)
		require.NoError(t, rows.Close())
	}
	queryErr := errors.New("execution failed")
	stmt.ExpectQuery().WithArgs(56).WillReturnError(queryErr)
	_, err = db.queryView(query, 56)
	require.ErrorIs(t, err, queryErr)
	// A new shape beyond the cap executes normally, without preparing or
	// evicting the existing statement.
	mock.ExpectQuery("SELECT 2").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
	rows, err := db.queryView("SELECT 2")
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	require.Len(t, db.preparedViews.statements, 1)
	require.NoError(t, db.ClosePreparedViews())
	require.NoError(t, db.ClosePreparedViews())
	// Cleanup leaves the shared pool usable and disables further preparation.
	mock.ExpectQuery(expected).WithArgs(78).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(78))
	rows, err = db.queryView(query, 78)
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPreparedViewsOptIn(t *testing.T) {
	for _, limit := range []int{-1, 0} {
		db := NewDBWithPreparedViews(nil, limit)
		require.Nil(t, db.preparedViews)
		require.NoError(t, db.ClosePreparedViews())
	}
	require.Nil(t, NewDB(nil).preparedViews)
}

func preparedTestPool(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("PREPARED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PREPARED_TEST_DATABASE_URL for PostgreSQL prepared-statement tests")
	}
	pool, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	return pool
}

func TestPreparedViewsPostgresReadsFreshData(t *testing.T) {
	pool := preparedTestPool(t)
	// Session-local data only; the single pool connection owns this table.
	_, err := pool.Exec(`CREATE TEMP TABLE prepared_view_items (id bigint PRIMARY KEY, owner_id bigint, mode text);
	 INSERT INTO prepared_view_items VALUES (1,10,'blur'),(2,20,'blur_faces');`)
	require.NoError(t, err)
	db := NewDBWithPreparedViews(pool, 2)
	defer db.ClosePreparedViews()
	id, owner, mode := pg.IntegerColumn("id"), pg.IntegerColumn("owner_id"), pg.StringColumn("mode")
	table := pg.NewTable("pg_temp", "prepared_view_items", "", id, owner, mode)
	view := func(viewer, link int64) string {
		projection := pg.CASE().WHEN(owner.EQ(pg.Int(viewer))).THEN(pg.String("original")).ELSE(mode).AS("mode")
		result, err := db.View(log.NewEntry(log.New()), table, id,
			[]fields.ModuleField{{Column: mode, Type: fields.ModuleFieldTypeString, SelectExpression: projection}},
			id.EQ(pg.Int(link)), nil, nil)
		require.NoError(t, err)
		return result.(map[string]interface{})["mode"].(string)
	}
	for i := 0; i < 12; i++ {
		require.Equal(t, "original", view(10, 1))
		require.Equal(t, "blur", view(20, 1))
		require.Equal(t, "blur_faces", view(10, 2))
	}
	require.Len(t, db.preparedViews.statements, 1)
	var query string
	for query = range db.preparedViews.statements {
	}
	var generic int
	require.NoError(t, pool.QueryRow("SELECT generic_plans FROM pg_prepared_statements WHERE statement=$1", query).Scan(&generic))
	require.Positive(t, generic, "must exercise a reused generic plan")
	// Updating visibility and ownership must take effect immediately even
	// after PostgreSQL has switched to a generic execution plan.
	_, err = pool.Exec("UPDATE prepared_view_items SET owner_id=30, mode='blur_faces' WHERE id=1")
	require.NoError(t, err)
	require.Equal(t, "blur_faces", view(10, 1))
	require.Equal(t, "original", view(30, 1))
}

func TestPreparedViewsPostgresConcurrentAndReconnect(t *testing.T) {
	pool := preparedTestPool(t)
	pool.SetMaxOpenConns(4)
	pool.SetMaxIdleConns(4)
	db := NewDBWithPreparedViews(pool, 1)
	defer db.ClosePreparedViews()
	const query = "SELECT $1::bigint, $2::text, pg_sleep(0.002)"
	read := func(id int64) error {
		rows, err := db.queryView(query, id, fmt.Sprint(id))
		if err != nil {
			return err
		}
		defer rows.Close()
		if !rows.Next() {
			return fmt.Errorf("no row: %v", rows.Err())
		}
		var actual int64
		var role string
		var ignored interface{}
		if err := rows.Scan(&actual, &role, &ignored); err != nil {
			return err
		}
		if actual != id || role != fmt.Sprint(id) {
			return fmt.Errorf("arguments mixed: %d/%s, expected %d", actual, role, id)
		}
		return rows.Err()
	}
	var wg sync.WaitGroup
	errors := make(chan error, 32)
	for i := int64(0); i < 32; i++ {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); errors <- read(id) }(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Len(t, db.preparedViews.statements, 1)
	// Drop all idle connections; sql.Stmt must prepare again on a new one.
	pool.SetMaxIdleConns(0)
	require.Zero(t, pool.Stats().OpenConnections)
	pool.SetMaxIdleConns(4)
	require.NoError(t, read(999))
}
