package db

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

type countedDebugValue struct{ calls *int }

func (v countedDebugValue) String() string { *v.calls++; return "value" }

func TestDisabledSQLDebugDoesNotFormatArguments(t *testing.T) {
	var output bytes.Buffer
	logger := log.New()
	logger.SetOutput(&output)
	entry := log.NewEntry(logger)
	calls := 0
	db := &DB{}
	db.debugQuery(entry, "QUERY", "SELECT $1", []interface{}{countedDebugValue{&calls}})
	if calls != 0 || output.Len() != 0 {
		t.Fatal("disabled SQL debug must neither format values nor emit a log")
	}
	db.Debug = true
	db.debugQuery(entry, "QUERY", "SELECT $1", []interface{}{countedDebugValue{&calls}})
	if calls != 1 || !strings.Contains(output.String(), "SELECT 'value'") {
		t.Fatal("enabled SQL debug must retain the formatted query")
	}
}

func BenchmarkDisabledSQLDebug(b *testing.B) {
	var query strings.Builder
	args := make([]interface{}, 100)
	for i := range args {
		fmt.Fprintf(&query, "SELECT payload FROM records WHERE owner=$%d UNION ALL ", i+1)
		args[i] = i
	}
	sql := query.String()
	db, entry := &DB{}, log.NewEntry(log.New())
	b.Run("previous_eager_format", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			db.debugLog(entry, "QUERY", interpolateQuery(sql, args))
		}
	})
	b.Run("disabled_skips_format", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			db.debugQuery(entry, "QUERY", sql, args)
		}
	})
}
