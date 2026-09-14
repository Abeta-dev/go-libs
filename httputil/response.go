// SPDX-License-Identifier: MIT

// Package httputil provides reusable HTTP response helpers suitable for use
// across multiple microservices. It has no dependencies on the core-platform
// domain — import it as a standalone library module.
package httputil

import (
	"encoding/json"
	"errors"
	"net/http"
)

// APIError is an interface that domain errors can implement to be recognized by the response layer natively.
type APIError interface {
	Error() string
	GetStatus() int
	GetCode() string
}

// ErrResponse represents a standard error response body format.
// This is used globally to ensure consistency and for Swagger documentation.
type ErrResponse struct {
	Error string `json:"error" example:"error message"`
	Code  string `json:"code,omitempty" example:"ERROR_CODE"`
}

// WriteJSON serialises v to JSON and writes it with the given HTTP status.
// It is the low-level primitive; prefer the typed helpers below.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// OK sends 200 OK with the payload.
func OK(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

// Created sends 201 Created with the payload.
func Created(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, data)
}

// NoContent sends 204 No Content.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error sends a JSON error response with the given message and HTTP status.
func Error(w http.ResponseWriter, msg string, status int) {
	WriteJSON(w, status, ErrResponse{Error: msg})
}

// ErrorFromDomain translates an APIError (or any error in its chain) into
// the correct HTTP status and client-safe message automatically.
// Falls back to 500 for unknown error types.
func ErrorFromDomain(w http.ResponseWriter, err error) {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		WriteJSON(w, apiErr.GetStatus(), ErrResponse{Error: apiErr.Error(), Code: apiErr.GetCode()})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, ErrResponse{
		Error: err.Error(),
		Code:  "INTERNAL_ERROR",
	})
}

// ValidationError is a convenience wrapper for 400 field-level errors.
func ValidationError(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusBadRequest, ErrResponse{
		Error: msg,
		Code:  "VALIDATION_ERROR",
	})
}
