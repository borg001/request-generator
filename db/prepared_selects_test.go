package db

import (
	"testing"
	"time"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	pg "github.com/go-jet/jet/v2/postgres"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestPreparedSelectsPostgresListMatchesPlain(t *testing.T) {
	pool := preparedTestPool(t)
	_, err := pool.Exec(`CREATE TEMP TABLE prepared_list_items (id bigint PRIMARY KEY, owner_id bigint, label text);
	 INSERT INTO prepared_list_items VALUES (1,10,'first'),(2,20,'second'),(3,10,'third');`)
	require.NoError(t, err)
	id, owner, label := pg.IntegerColumn("id"), pg.IntegerColumn("owner_id"), pg.StringColumn("label")
	table := pg.NewTable("pg_temp", "prepared_list_items", "", id, owner, label)
	plain, prepared := NewDB(pool), NewDBWithPreparedSelects(pool, 8)
	defer prepared.ClosePreparedViews()
	cached := NewDBWithPreparedSelects(pool, 8)
	defer cached.ClosePreparedViews()
	cached.EnableListSelectCache(ListSelectCacheOptions{TTL: 2 * time.Second, MaxEntries: 64, MaxBytes: 1 << 20, MaxResultBytes: 1 << 16})
	read := func(executor *DB, actor, page int64) ([]interface{}, int64) {
		rows, count, err := executor.List(log.NewEntry(log.New()), table, id,
			[]fields.ModuleField{{Column: label, Type: fields.ModuleFieldTypeString}}, nil,
			page, 1, nil, "", nil, owner.EQ(pg.Int(actor)), nil,
			&actions.SortOption{Column: id}, nil)
		require.NoError(t, err)
		return rows, count
	}
	for i := 0; i < 12; i++ {
		for _, actor := range []int64{10, 20, 30} {
			for _, page := range []int64{0, 1, 2} {
				want, total := read(plain, actor, page)
				got, count := read(prepared, actor, page)
				require.Equal(t, want, got)
				require.Equal(t, total, count)
				cachedRows, cachedCount := read(cached, actor, page)
				require.Equal(t, want, cachedRows)
				require.Equal(t, total, cachedCount)
			}
		}
	}
	require.Positive(t, cached.ListSelectCacheStats().Hits)
	require.Len(t, prepared.preparedViews.statements, 2, "List and COUNT must both reuse SQL across parameters")
	_, err = pool.Exec("UPDATE prepared_list_items SET owner_id=30 WHERE id=1")
	require.NoError(t, err)
	for _, actor := range []int64{10, 30} {
		want, total := read(plain, actor, 0)
		got, count := read(prepared, actor, 0)
		require.Equal(t, want, got)
		require.Equal(t, total, count)
	}
}
