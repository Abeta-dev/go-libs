// SPDX-License-Identifier: MIT

package httputil_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umesh0492/go-libs/httputil"
)

type mockAPIError struct {
	status int
	code   string
	msg    string
}

func (m mockAPIError) Error() string   { return m.msg }
func (m mockAPIError) GetStatus() int  { return m.status }
func (m mockAPIError) GetCode() string { return m.code }

func TestOK_WritesJSON(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.OK(w, map[string]string{"status": "ok"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestCreated_WritesJSON(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.Created(w, map[string]string{"id": "abc"})
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.NoContent(w)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestError_WritesErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.Error(w, "something went wrong", http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "something went wrong", body["error"])
}

func TestErrorFromDomain_AppError(t *testing.T) {
	w := httptest.NewRecorder()
	mock := mockAPIError{status: http.StatusNotFound, code: "NOT_FOUND", msg: "user not found"}
	httputil.ErrorFromDomain(w, mock)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "user not found", body["error"])
	assert.Equal(t, "NOT_FOUND", body["code"])
}

func TestErrorFromDomain_FallsBackTo500(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.ErrorFromDomain(w, errors.New("some raw error"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body httputil.ErrResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "some raw error", body.Error)
	assert.Equal(t, "INTERNAL_ERROR", body.Code)
}

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	httputil.ValidationError(w, "invalid field name")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body httputil.ErrResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "invalid field name", body.Error)
	assert.Equal(t, "VALIDATION_ERROR", body.Code)
}

func TestErrorFromDomain_WithAppError(t *testing.T) {
	// mockAPIError verifies that any APIError implementation works.
	// We also verify that custom domain errors with details work seamlessly.
	w := httptest.NewRecorder()
	mock := mockAPIError{status: http.StatusForbidden, code: "FORBIDDEN", msg: "access denied"}
	httputil.ErrorFromDomain(w, mock)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var body httputil.ErrResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "access denied", body.Error)
	assert.Equal(t, "FORBIDDEN", body.Code)
}
