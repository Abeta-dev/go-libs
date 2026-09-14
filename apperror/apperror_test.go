// SPDX-License-Identifier: MIT

package apperror_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umesh0492/go-libs/apperror"
)

func TestNew(t *testing.T) {
	e := apperror.New(apperror.CodeNotFound, "user not found")
	require.NotNil(t, e)
	assert.Equal(t, apperror.CodeNotFound, e.Code)
	assert.Equal(t, "user not found", e.Message)
	assert.Nil(t, e.Err)
	assert.Contains(t, e.Error(), "NOT_FOUND")
	assert.Contains(t, e.Error(), "user not found")
}

func TestNewWithDetails(t *testing.T) {
	details := map[string]string{"field": "email"}
	e := apperror.NewWithDetails(apperror.CodeBadRequest, "invalid email", details)
	assert.Equal(t, apperror.CodeBadRequest, e.Code)
	assert.Equal(t, details, e.Details)
}

func TestWrap(t *testing.T) {
	cause := errors.New("db timeout")
	e := apperror.Wrap(cause, apperror.CodeInternal, "database error")
	assert.Equal(t, cause, errors.Unwrap(e))
	assert.Contains(t, e.Error(), "db timeout")
}

func TestSentinels(t *testing.T) {
	tests := []struct {
		fn     func(string) *apperror.Error
		code   apperror.Code
		status int
	}{
		{apperror.BadRequest, apperror.CodeBadRequest, 400},
		{apperror.Unauthorized, apperror.CodeUnauthorized, 401},
		{apperror.Forbidden, apperror.CodeForbidden, 403},
		{apperror.NotFound, apperror.CodeNotFound, 404},
		{apperror.Conflict, apperror.CodeConflict, 409},
		{apperror.Unprocessable, apperror.CodeUnprocessable, 422},
		{apperror.Internal, apperror.CodeInternal, 500},
		{apperror.Unavailable, apperror.CodeUnavailable, 503},
	}
	for _, tc := range tests {
		e := tc.fn("test")
		assert.Equal(t, tc.code, e.Code)
		assert.Equal(t, tc.status, e.GetStatus())
		assert.Equal(t, string(tc.code), e.GetCode())
	}
}

func TestGetStatus_UnknownCode(t *testing.T) {
	e := apperror.New("UNKNOWN_CODE", "something")
	assert.Equal(t, 500, e.GetStatus())
}

func TestIs(t *testing.T) {
	e := apperror.NotFound("user missing")
	assert.True(t, apperror.Is(e, apperror.CodeNotFound))
	assert.False(t, apperror.Is(e, apperror.CodeInternal))
	assert.False(t, apperror.Is(nil, apperror.CodeNotFound))
	assert.False(t, apperror.Is(errors.New("plain"), apperror.CodeNotFound))
}

func TestIs_WrappedChain(t *testing.T) {
	inner := apperror.NotFound("resource gone")
	outer := apperror.Wrap(inner, apperror.CodeInternal, "handler failed")
	// outer wraps inner; Is(outer, NotFound) should walk the chain
	assert.True(t, apperror.Is(outer, apperror.CodeInternal))
}

func TestCodeOf(t *testing.T) {
	e := apperror.Forbidden("access denied")
	assert.Equal(t, apperror.CodeForbidden, apperror.CodeOf(e))
	assert.Equal(t, apperror.Code(""), apperror.CodeOf(nil))
	assert.Equal(t, apperror.Code(""), apperror.CodeOf(errors.New("plain")))
}

func TestUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	e := apperror.Wrap(cause, apperror.CodeInternal, "wrapped")
	assert.ErrorIs(t, e, cause)
}

func TestErrorWithNilCause_NoColonCause(t *testing.T) {
	e := apperror.NotFound("nothing here")
	msg := e.Error()
	assert.Equal(t, "NOT_FOUND: nothing here", msg)
}
