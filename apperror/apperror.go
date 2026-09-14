// SPDX-License-Identifier: MIT

// Package apperror defines canonical application-level error codes and a
// structured Error type for use across microservices.
//
// Domain services return *Error values; the transport layer (httputil) maps
// them to HTTP status codes and client-safe JSON responses.
//
// Usage:
//
//	return apperror.NotFound("user not found")
//	return apperror.Wrap(dbErr, apperror.CodeInternal, "database read failed")
//
//	// In transport layer:
//	httputil.ErrorFromDomain(w, err)  // auto-maps *apperror.Error → HTTP status
package apperror

import (
	"errors"
	"fmt"
)

// Code is a machine-readable error classification string.
type Code string

// Canonical error codes. Use these in domain logic; transport adapters map
// them to appropriate HTTP status codes.
const (
	// CodeBadRequest maps to HTTP 400 — malformed or invalid request.
	CodeBadRequest Code = "BAD_REQUEST"
	// CodeUnauthorized maps to HTTP 401 — authentication required.
	CodeUnauthorized Code = "UNAUTHORIZED"
	// CodeForbidden maps to HTTP 403 — authenticated but not permitted.
	CodeForbidden Code = "FORBIDDEN"
	// CodeNotFound maps to HTTP 404 — resource does not exist.
	CodeNotFound Code = "NOT_FOUND"
	// CodeConflict maps to HTTP 409 — resource state conflict.
	CodeConflict Code = "CONFLICT"
	// CodeUnprocessable maps to HTTP 422 — semantically invalid input.
	CodeUnprocessable Code = "UNPROCESSABLE"
	// CodeInternal maps to HTTP 500 — unexpected internal failure.
	CodeInternal Code = "INTERNAL"
	// CodeUnavailable maps to HTTP 503 — dependency or capacity issue.
	CodeUnavailable Code = "UNAVAILABLE"
)

// Error is a structured application error with a machine-readable Code,
// a human-readable Message, optional Details, and an optional wrapped cause.
type Error struct {
	// Code is the canonical error classification.
	Code Code `json:"code"`
	// Message is a safe, user-facing description.
	Message string `json:"message"`
	// Details holds optional structured context (e.g. field validation map).
	Details any `json:"details,omitempty"`
	// Err is the underlying cause (not serialised).
	Err error `json:"-"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped cause so errors.Is and errors.As work correctly.
func (e *Error) Unwrap() error { return e.Err }

// GetCode satisfies the httputil.APIError interface.
func (e *Error) GetCode() string { return string(e.Code) }

// GetStatus returns the HTTP status code that corresponds to this error's Code.
// It satisfies the httputil.APIError interface.
func (e *Error) GetStatus() int {
	switch e.Code {
	case CodeBadRequest:
		return 400
	case CodeUnauthorized:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeUnprocessable:
		return 422
	case CodeUnavailable:
		return 503
	default:
		return 500
	}
}

// ── Constructors ───────────────────────────────────────────────────────────────

// New creates a new *Error with the given code and message.
func New(code Code, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// NewWithDetails creates a new *Error with structured details attached.
func NewWithDetails(code Code, msg string, details any) *Error {
	return &Error{Code: code, Message: msg, Details: details}
}

// Wrap creates a new *Error that wraps an existing error cause.
func Wrap(err error, code Code, msg string) *Error {
	return &Error{Code: code, Message: msg, Err: err}
}

// ── Sentinel helpers ───────────────────────────────────────────────────────────

// BadRequest returns a CodeBadRequest error.
func BadRequest(msg string) *Error { return New(CodeBadRequest, msg) }

// Unauthorized returns a CodeUnauthorized error.
func Unauthorized(msg string) *Error { return New(CodeUnauthorized, msg) }

// Forbidden returns a CodeForbidden error.
func Forbidden(msg string) *Error { return New(CodeForbidden, msg) }

// NotFound returns a CodeNotFound error.
func NotFound(msg string) *Error { return New(CodeNotFound, msg) }

// Conflict returns a CodeConflict error.
func Conflict(msg string) *Error { return New(CodeConflict, msg) }

// Unprocessable returns a CodeUnprocessable error.
func Unprocessable(msg string) *Error { return New(CodeUnprocessable, msg) }

// Internal returns a CodeInternal error.
func Internal(msg string) *Error { return New(CodeInternal, msg) }

// Unavailable returns a CodeUnavailable error.
func Unavailable(msg string) *Error { return New(CodeUnavailable, msg) }

// ── Introspection ─────────────────────────────────────────────────────────────

// Is reports whether err (or any error in its chain) is an *Error with the
// given code.
func Is(err error, code Code) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code == code
}

// CodeOf returns the Code of err if it is (or wraps) an *Error, or an empty
// string otherwise.
func CodeOf(err error) Code {
	var e *Error
	if !errors.As(err, &e) {
		return ""
	}
	return e.Code
}
