package db

import "testing"

func TestSafeSQLIdentifier(t *testing.T) {
	good := []string{"profiles", "agency", "chat_actor_participant", "_x", "a1"}
	for _, name := range good {
		if !SafeSQLIdentifier(name) {
			t.Fatalf("%q must be accepted", name)
		}
	}
	// A dotted filter key whose table part is an expression must be refused,
	// so it can never be pasted into SQL.
	bad := []string{
		"CAST((SELECT token FROM sessions) AS int)=1 OR profiles",
		"(1/0)=1 OR profiles",
		"profiles;DROP TABLE users",
		"pro files",
		"pro\"files",
		"",
		"1abc",
	}
	for _, name := range bad {
		if SafeSQLIdentifier(name) {
			t.Fatalf("%q must be refused", name)
		}
	}
}
