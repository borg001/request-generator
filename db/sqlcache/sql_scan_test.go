package sqlcache

import (
	"strings"
	"testing"
)

func compareScanners(t *testing.T, query string) {
	t.Helper()
	if got, want := readOnly(query), legacyReadOnly(query); got != want {
		t.Fatalf("readOnly(%q): got %v want %v", query, got, want)
	}
	tokens, valid := legacySQLTokens(query)
	want := !valid
	for i, token := range tokens {
		if i == 0 {
			switch token {
			case "SET", "RESET", "DISCARD", "BEGIN", "START", "COMMIT", "ROLLBACK", "DO":
				want = true
			}
		}
		if token == "SET_CONFIG" || token == "TEMP" || token == "TEMPORARY" {
			want = true
		}
	}
	c := &conn{}
	c.trackSession(query)
	if c.sessionChanged != want {
		t.Fatalf("session classification differs for %q", query)
	}
}

func FuzzSQLScannerCompatibility(f *testing.F) {
	for _, query := range []string{
		`SELECT "value", 'a''b', E'c\'d' FROM records WHERE id=$1`,
		`/* outer /* nested */ */ WITH x AS (SELECT 1) SELECT * FROM x; -- end`,
		`SELECT "ne""xtval"('sequence')`, `SELECT "nextval"('sequence')`,
		`SELECT "pg_advisory_lock"(1)`, `SELECT U&'abc'`, `SELECT $$txt$$`,
		`SELECT 'unterminated`, `SELECT 1; DELETE FROM records`,
		`SELECT "ſelect", "Слово", "PG_very_long_function_name_beyond_32_bytes"`,
		`SET role = 'reader'`, `SELECT set_config('role','reader',false)`,
	} {
		f.Add(query)
	}
	f.Fuzz(compareScanners)
}

func TestSQLScannerAllocations(t *testing.T) {
	query := "SELECT " + strings.Repeat(`CASE WHEN "value"=$1 THEN 'literal contents' ELSE NULL END, `, 512) + "id FROM records"
	compareScanners(t, query)
	if n := testing.AllocsPerRun(100, func() {
		if !readOnly(query) {
			panic("read rejected")
		}
	}); n != 0 {
		t.Fatalf("scanner allocated %.0f objects", n)
	}
}

func BenchmarkSQLReadOnly(b *testing.B) {
	query := "SELECT " + strings.Repeat(`CASE WHEN "value"=$1 THEN 'literal contents' ELSE NULL END, `, 512) + "id FROM records"
	for name, scan := range map[string]func(string) bool{"before": legacyReadOnly, "streaming": readOnly} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if !scan(query) {
					b.Fatal("read rejected")
				}
			}
		})
	}
}
