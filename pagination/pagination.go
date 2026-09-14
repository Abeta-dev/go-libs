// SPDX-License-Identifier: MIT

// Package pagination provides reusable helpers for offset-based pagination.
// It has no external dependencies and can be used as a standalone library module.
package pagination

import (
	"fmt"
	"net/http"
)

// Default pagination constants and bounds.
const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// Params holds parsed pagination values.
type Params struct {
	Page  int
	Limit int
}

// Parse extracts page and limit query parameters from the HTTP request.
// Values are clamped to sane defaults.
func Parse(r *http.Request) Params {
	return Params{
		Page:  parseIntClamp(r.URL.Query().Get("page"), DefaultPage, 1, 10_000),
		Limit: parseIntClamp(r.URL.Query().Get("limit"), DefaultLimit, 1, MaxLimit),
	}
}

// Offset returns the SQL OFFSET for the given page and limit.
func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// TotalPages calculates the number of pages for a given total count.
func TotalPages(total, limit int) int {
	if limit <= 0 || total <= 0 {
		return 0
	}
	pages := total / limit
	if total%limit != 0 {
		pages++
	}
	return pages
}

// Response wraps items with standard pagination metadata.
type Response struct {
	Items interface{} `json:"items"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
	Pages int         `json:"pages"`
}

// NewResponse constructs a paginated response.
func NewResponse(items interface{}, total int, p Params) Response {
	return Response{
		Items: items,
		Total: total,
		Page:  p.Page,
		Limit: p.Limit,
		Pages: TotalPages(total, p.Limit),
	}
}

// TypedResponse wraps a typed slice of items with standard pagination metadata.
type TypedResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Pages int `json:"pages"`
}

// NewTypedResponse constructs a type-safe paginated response for a typed slice.
func NewTypedResponse[T any](items []T, total int, p Params) TypedResponse[T] {
	return TypedResponse[T]{
		Items: items,
		Total: total,
		Page:  p.Page,
		Limit: p.Limit,
		Pages: TotalPages(total, p.Limit),
	}
}

// parseIntClamp parses the string s as an int, returning def on failure,
// clamped to [minVal, maxVal].
func parseIntClamp(s string, def, minVal, maxVal int) int {
	if s == "" {
		return def
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil || v < minVal {
		return def
	}
	if v > maxVal {
		return maxVal
	}
	return v
}
