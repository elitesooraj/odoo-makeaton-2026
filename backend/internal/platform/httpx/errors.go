// Package httpx defines the HTTP envelope, error mapping and shared middleware.
package httpx

import (
	"errors"
	"net/http"
)

// Error is the only error type handlers return to Fail. It carries the HTTP
// status, a stable machine-readable code, a human message and optional field
// details. Internal server errors keep the cause for logs but never expose it.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func Validation(details map[string]string) *Error {
	return &Error{
		Status:  http.StatusUnprocessableEntity,
		Code:    "VALIDATION_FAILED",
		Message: "one or more fields are invalid",
		Details: details,
	}
}

func BadRequest(message string) *Error {
	return NewError(http.StatusBadRequest, "BAD_REQUEST", message)
}

func Unauthenticated(message string) *Error {
	return NewError(http.StatusUnauthorized, "UNAUTHENTICATED", message)
}

func Forbidden(message string) *Error {
	return NewError(http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(message string) *Error {
	return NewError(http.StatusNotFound, "NOT_FOUND", message)
}

func Conflict(message string) *Error {
	return NewError(http.StatusConflict, "CONFLICT", message)
}

func RateLimited(message string) *Error {
	return NewError(http.StatusTooManyRequests, "RATE_LIMITED", message)
}

func Internal(err error) *Error {
	return &Error{
		Status:  http.StatusInternalServerError,
		Code:    "INTERNAL",
		Message: "an unexpected error occurred",
		Err:     err,
	}
}

// Is reports whether err is an application error with the given code.
func Is(err error, code string) bool {
	var appErr *Error
	return errors.As(err, &appErr) && appErr.Code == code
}
