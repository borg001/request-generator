package actions

import (
	"errors"
	"net/http"
)

// StatusError is a refusal that names the HTTP status it answers with: a
// record the reader may not see is not found (404), not a bad request.
type StatusError struct {
	status  int
	message string
}

// NewStatusError refuses with a 4xx status; anything else answers 400, as a
// refusal did before it could name one.
func NewStatusError(status int, message string) error {
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		status = http.StatusBadRequest
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &StatusError{status: status, message: message}
}

func (err *StatusError) Error() string   { return err.message }
func (err *StatusError) StatusCode() int { return err.status }

// ErrorStatus is the 4xx status an error names - a StatusError or an
// AtomicCommittedRejection - or fallback for any other error.
func ErrorStatus(err error, fallback int) int {
	var coded interface{ StatusCode() int }
	if errors.As(err, &coded) {
		if status := coded.StatusCode(); status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			return status
		}
	}
	return fallback
}
