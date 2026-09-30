package actions

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestStatusErrorNamesItsStatus(t *testing.T) {
	notFound := NewStatusError(http.StatusNotFound, "profiles.not_found")
	if got := ErrorStatus(notFound, http.StatusBadRequest); got != http.StatusNotFound {
		t.Fatalf("ErrorStatus(404 refusal) = %d, want 404", got)
	}
	if notFound.Error() != "profiles.not_found" {
		t.Fatalf("message = %q", notFound.Error())
	}
	wrapped := fmt.Errorf("profile view: %w", notFound)
	if got := ErrorStatus(wrapped, http.StatusBadRequest); got != http.StatusNotFound {
		t.Fatalf("ErrorStatus(wrapped 404) = %d, want 404", got)
	}
	rejection := NewAtomicCommittedRejectionWithStatus(http.StatusConflict, "taken")
	if got := ErrorStatus(rejection, http.StatusBadRequest); got != http.StatusConflict {
		t.Fatalf("ErrorStatus(409 rejection) = %d, want 409", got)
	}
}

func TestStatusErrorFallsBackToBadRequest(t *testing.T) {
	if got := ErrorStatus(errors.New("plain"), http.StatusBadRequest); got != http.StatusBadRequest {
		t.Fatalf("ErrorStatus(plain) = %d, want 400", got)
	}
	for _, status := range []int{0, http.StatusOK, http.StatusInternalServerError} {
		err := NewStatusError(status, "")
		if got := ErrorStatus(err, http.StatusBadRequest); got != http.StatusBadRequest {
			t.Fatalf("NewStatusError(%d) answers %d, want 400", status, got)
		}
		if err.Error() != http.StatusText(http.StatusBadRequest) {
			t.Fatalf("NewStatusError(%d) message = %q", status, err.Error())
		}
	}
}
