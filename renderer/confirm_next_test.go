package renderer

import (
	"testing"
)

// A confirmation can ask a second question; it is cloned apart from the
// first and checked like it.
func TestConfirmNextIsClonedAndChecked(t *testing.T) {
	first := &Confirm{Title: "a", Message: "b", CancelLabel: "c", ConfirmLabel: "d", Next: &Confirm{Title: "e", Message: "f", CancelLabel: "g", ConfirmLabel: "h"}}
	copied := cloneConfirm(first)
	if copied.Next == first.Next {
		t.Fatal("the second question is shared with the declaration")
	}
	copied.Next.Title = "changed"
	if first.Next.Title != "e" {
		t.Fatal("changing the copy changed the declaration")
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("a full chain is valid: %v", err)
	}
	first.Next.ConfirmLabel = ""
	if err := first.Validate(); err == nil {
		t.Fatal("a second question without a confirm label must be refused")
	}
}
