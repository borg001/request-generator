package db

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/darkrain/request-generator/actions"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"
)

func TestAtomicExecutorSelectOneReturnsTypedValues(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	id := pg.IntegerColumn("id")
	userID := pg.IntegerColumn("user_id")
	locationIDs := pg.StringColumn("location_ids")
	profiles := pg.NewTable("public", "profiles", "", id, userID, locationIDs)

	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "location_ids"}).AddRow(17, "{101,102}"))
	mock.ExpectRollback()

	record, err := NewAtomicExecutor(tx).SelectOne(context.Background(), actions.AtomicSelect{
		Table: profiles,
		Fields: []actions.AtomicSelectField{
			{Name: "agency_id", Column: id, Kind: actions.AtomicValueKindInt},
			{Name: "location_ids", Column: locationIDs, Kind: actions.AtomicValueKindInts},
		},
		Where: userID.EQ(pg.Int(7)),
	})
	require.NoError(t, err)
	agency, ok := record.Field("agency_id")
	require.True(t, ok)
	require.Equal(t, int64(17), *agency.Int)
	locations, ok := record.Field("location_ids")
	require.True(t, ok)
	require.Equal(t, []int64{101, 102}, locations.Ints)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorSelectOneRequiresWhere(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	id := pg.IntegerColumn("id")
	table := pg.NewTable("public", "profiles", "", id)

	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = NewAtomicExecutor(tx).SelectOne(context.Background(), actions.AtomicSelect{
		Table:  table,
		Fields: []actions.AtomicSelectField{{Name: "id", Column: id, Kind: actions.AtomicValueKindInt}},
	})
	require.Error(t, err)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorSelectOneMapsNullableStringToEmptyValue(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	email := pg.StringColumn("email")
	users := pg.NewTable("public", "users", "", id, email)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow(nil))
	mock.ExpectRollback()

	record, err := NewAtomicExecutor(tx).SelectOne(context.Background(), actions.AtomicSelect{
		Table:  users,
		Fields: []actions.AtomicSelectField{{Name: "email", Column: email, Kind: actions.AtomicValueKindNullableString}},
		Where:  id.EQ(pg.Int(17)),
	})
	require.NoError(t, err)
	value, ok := record.String("email")
	require.True(t, ok)
	require.Empty(t, value)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorSelectManyReturnsTypedRows(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	name := pg.StringColumn("name")
	rating := pg.FloatColumn("rating")
	active := pg.BoolColumn("active")
	memberIDs := pg.StringColumn("member_ids")
	tags := pg.StringColumn("tags")
	createdAt := pg.TimestampzColumn("created_at")
	profiles := pg.NewTable("public", "profiles", "", id, name, rating, active, memberIDs, tags, createdAt)
	firstCreatedAt := time.Date(2026, time.August, 14, 17, 30, 0, 0, time.UTC)

	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectQuery("ORDER BY").WillReturnRows(sqlmock.NewRows([]string{"name", "id", "rating", "active", "member_ids", "tags", "created_at"}).
		AddRow("Ada", 17, 4.5, true, "{101,102}", "{vip,verified}", firstCreatedAt).
		AddRow("Lin", 19, 3.25, false, "{}", "{}", firstCreatedAt.Add(time.Minute)))
	mock.ExpectRollback()

	records, err := NewAtomicExecutor(tx).SelectMany(context.Background(), actions.AtomicSelectMany{
		AtomicSelect: actions.AtomicSelect{
			Table: profiles,
			Fields: []actions.AtomicSelectField{
				{Name: "name", Column: name, Kind: actions.AtomicValueKindString},
				{Name: "id", Column: id, Kind: actions.AtomicValueKindInt},
				{Name: "rating", Column: rating, Kind: actions.AtomicValueKindFloat},
				{Name: "active", Column: active, Kind: actions.AtomicValueKindBool},
				{Name: "member_ids", Column: memberIDs, Kind: actions.AtomicValueKindInts},
				{Name: "tags", Column: tags, Kind: actions.AtomicValueKindStrings},
				{Name: "created_at", Column: createdAt, Kind: actions.AtomicValueKindTime},
			},
			Where: active.IS_TRUE(),
		},
		OrderBy: []pg.OrderByClause{id.ASC()},
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, records, 2)

	firstID, ok := records[0].Int("id")
	require.True(t, ok)
	require.Equal(t, int64(17), firstID)
	firstName, ok := records[0].String("name")
	require.True(t, ok)
	require.Equal(t, "Ada", firstName)
	ratingValue, ok := records[0].Field("rating")
	require.True(t, ok)
	require.Equal(t, 4.5, *ratingValue.Float)
	activeValue, ok := records[0].Field("active")
	require.True(t, ok)
	require.True(t, *activeValue.Bool)
	memberIDsValue, ok := records[0].Field("member_ids")
	require.True(t, ok)
	require.Equal(t, []int64{101, 102}, memberIDsValue.Ints)
	tagsValue, ok := records[0].Field("tags")
	require.True(t, ok)
	require.Equal(t, []string{"vip", "verified"}, tagsValue.Strings)
	createdAtValue, ok := records[0].Field("created_at")
	require.True(t, ok)
	require.Equal(t, firstCreatedAt, *createdAtValue.Time)

	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorSelectManyReturnsEmptySlice(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	profiles := pg.NewTable("public", "profiles", "", id)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()

	records, err := NewAtomicExecutor(tx).SelectMany(context.Background(), actions.AtomicSelectMany{
		AtomicSelect: actions.AtomicSelect{
			Table:  profiles,
			Fields: []actions.AtomicSelectField{{Name: "id", Column: id, Kind: actions.AtomicValueKindInt}},
			Where:  id.GT(pg.Int(0)),
		},
		OrderBy: []pg.OrderByClause{id.ASC()},
		Limit:   5,
	})
	require.NoError(t, err)
	require.NotNil(t, records)
	require.Empty(t, records)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorSelectManyRequiresBoundedOrder(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	profiles := pg.NewTable("public", "profiles", "", id)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = NewAtomicExecutor(tx).SelectMany(context.Background(), actions.AtomicSelectMany{
		AtomicSelect: actions.AtomicSelect{
			Table:  profiles,
			Fields: []actions.AtomicSelectField{{Name: "id", Column: id, Kind: actions.AtomicValueKindInt}},
			Where:  id.GT(pg.Int(0)),
		},
		Limit: 5,
	})
	require.EqualError(t, err, "atomic select many requires order by")

	_, err = NewAtomicExecutor(tx).SelectMany(context.Background(), actions.AtomicSelectMany{
		AtomicSelect: actions.AtomicSelect{
			Table:  profiles,
			Fields: []actions.AtomicSelectField{{Name: "id", Column: id, Kind: actions.AtomicValueKindInt}},
			Where:  id.GT(pg.Int(0)),
		},
		OrderBy: []pg.OrderByClause{id.ASC()},
	})
	require.EqualError(t, err, "atomic select many requires a positive limit")

	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorUpdateSetAndIncrement(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	name := pg.StringColumn("name")
	unreadCount := pg.IntegerColumn("unread_count")
	arrivalDate := pg.DateColumn("arrival_date")
	date := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	profiles := pg.NewTable("public", "profiles", "", id, name, unreadCount, arrivalDate)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectExec("UPDATE").WithArgs("Renamed", int64(1), date, int64(17)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	updated, err := NewAtomicExecutor(tx).Update(context.Background(), actions.AtomicUpdate{
		Table: profiles,
		Fields: []actions.AtomicUpdateField{
			{Column: name, Operation: actions.AtomicUpdateSet, Value: actions.AtomicString("Renamed")},
			{Column: unreadCount, Operation: actions.AtomicUpdateIncrement, Value: actions.AtomicInt(1)},
			{Column: arrivalDate, Operation: actions.AtomicUpdateSet, Value: actions.AtomicTime(date)},
		},
		Where: id.EQ(pg.Int(17)),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), updated)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorUpdateRequiresFieldsAndWhereBeforeExecutingSQL(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		update actions.AtomicUpdate
	}{
		{
			name: "fields",
			update: actions.AtomicUpdate{
				Table: pg.NewTable("public", "profiles", "", pg.IntegerColumn("id")),
				Where: pg.IntegerColumn("id").EQ(pg.Int(17)),
			},
		},
		{
			name: "where",
			update: actions.AtomicUpdate{
				Table: pg.NewTable("public", "profiles", "", pg.IntegerColumn("id"), pg.StringColumn("name")),
				Fields: []actions.AtomicUpdateField{{
					Column:    pg.StringColumn("name"),
					Operation: actions.AtomicUpdateSet,
					Value:     actions.AtomicString("Renamed"),
				}},
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })

			mock.ExpectBegin()
			tx, err := sqlDB.Begin()
			require.NoError(t, err)
			mock.ExpectRollback()

			_, err = NewAtomicExecutor(tx).Update(context.Background(), testCase.update)
			require.EqualError(t, err, "atomic update requires table, fields, and where")
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAtomicExecutorUpdateRejectsUnsupportedOperation(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	name := pg.StringColumn("name")
	profiles := pg.NewTable("public", "profiles", "", id, name)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = NewAtomicExecutor(tx).Update(context.Background(), actions.AtomicUpdate{
		Table:  profiles,
		Fields: []actions.AtomicUpdateField{{Column: name, Operation: actions.AtomicUpdateIncrement, Value: actions.AtomicInt(1)}},
		Where:  id.EQ(pg.Int(17)),
	})
	require.EqualError(t, err, `atomic update field 0: string column "name" requires string value`)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorUpdateReturnsZeroAffectedRows(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	active := pg.BoolColumn("active")
	profiles := pg.NewTable("public", "profiles", "", id, active)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectExec("UPDATE").WithArgs(false, int64(17)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	updated, err := NewAtomicExecutor(tx).Update(context.Background(), actions.AtomicUpdate{
		Table:  profiles,
		Fields: []actions.AtomicUpdateField{{Column: active, Operation: actions.AtomicUpdateSet, Value: actions.AtomicBool(false)}},
		Where:  id.EQ(pg.Int(17)),
	})
	require.NoError(t, err)
	require.Zero(t, updated)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorUpsertReturnsInsertAndConflictPaths(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		rows     *sqlmock.Rows
		inserted bool
		value    int64
		update   bool
	}{
		{
			name:     "inserted",
			rows:     sqlmock.NewRows([]string{"id", "inserted"}).AddRow(17, true),
			inserted: true,
			value:    17,
		},
		{
			name:     "conflict do nothing",
			rows:     sqlmock.NewRows([]string{"id", "inserted"}),
			inserted: false,
			value:    0,
		},
		{
			name:     "conflict update",
			rows:     sqlmock.NewRows([]string{"id", "inserted"}).AddRow(17, false),
			inserted: false,
			value:    17,
			update:   true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			id := pg.IntegerColumn("id")
			title := pg.StringColumn("title")
			items := pg.NewTable("public", "atomic_items", "", id, title)

			mock.ExpectBegin()
			tx, err := sqlDB.Begin()
			require.NoError(t, err)
			query := `INSERT INTO public."atomic_items" ("title") VALUES ($1) ON CONFLICT ("title") DO NOTHING RETURNING "id", (xmax = 0) AS inserted`
			args := []driver.Value{"unique"}
			if testCase.update {
				query = `INSERT INTO public."atomic_items" ("title") VALUES ($1) ON CONFLICT ("title") DO UPDATE SET "title" = $2 RETURNING "id", (xmax = 0) AS inserted`
				args = append(args, "updated")
			}
			mock.ExpectQuery(regexp.QuoteMeta(query)).
				WithArgs(args...).
				WillReturnRows(testCase.rows)
			mock.ExpectRollback()

			result, err := NewAtomicExecutor(tx).Upsert(context.Background(), actions.AtomicUpsert{
				Insert: actions.AtomicInsert{
					Table:      items,
					PrimaryKey: id,
					Fields:     []actions.AtomicInsertField{{Column: title, Value: actions.AtomicString("unique")}},
				},
				ConflictColumns: []pg.Column{title},
				UpdateFields: func() []actions.AtomicInsertField {
					if !testCase.update {
						return nil
					}
					return []actions.AtomicInsertField{{Column: title, Value: actions.AtomicString("updated")}}
				}(),
			})
			require.NoError(t, err)
			require.Equal(t, testCase.inserted, result.Inserted)
			require.Equal(t, testCase.value, result.Record.Value)
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAtomicExecutorDeleteRemovesSelectedRows(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	owner := pg.IntegerColumn("owner_id")
	drafts := pg.NewTable("public", "drafts", "", id, owner)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectExec("DELETE FROM public.drafts").WithArgs(int64(5), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	deleted, err := NewAtomicExecutor(tx).Delete(context.Background(), actions.AtomicDelete{
		Table: drafts,
		Where: id.EQ(pg.Int(5)).AND(owner.EQ(pg.Int(9))),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorDeleteRequiresWhereBeforeExecutingSQL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = NewAtomicExecutor(tx).Delete(context.Background(), actions.AtomicDelete{
		Table: pg.NewTable("public", "drafts", "", pg.IntegerColumn("id")),
	})
	require.EqualError(t, err, "atomic delete requires table and where")
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAtomicExecutorUpdateSetsADocumentAsAnUntypedParameter(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	id := pg.IntegerColumn("id")
	data := pg.StringColumn("data")
	drafts := pg.NewTable("public", "drafts", "", id, data)
	mock.ExpectBegin()
	tx, err := sqlDB.Begin()
	require.NoError(t, err)
	mock.ExpectExec(`UPDATE public\.drafts SET data = \(\$1\) WHERE`).WithArgs(`{"price":"700"}`, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	updated, err := NewAtomicExecutor(tx).Update(context.Background(), actions.AtomicUpdate{
		Table:  drafts,
		Fields: []actions.AtomicUpdateField{{Column: data, Operation: actions.AtomicUpdateSet, Value: actions.AtomicValue{JSON: []byte(`{"price":"700"}`)}}},
		Where:  id.EQ(pg.Int(5)),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), updated)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
