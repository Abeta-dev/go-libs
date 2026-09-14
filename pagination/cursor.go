// SPDX-License-Identifier: MIT

package pagination

import (
	"encoding/base64"
	"net/http"
)

// CursorParams holds parsed cursor-based pagination parameters.
type CursorParams struct {
	Cursor string
	Limit  int
}

// ParseCursor extracts cursor and limit query parameters from an HTTP request.
// If the limit is missing or invalid, it defaults to DefaultLimit, clamped between 1 and MaxLimit.
func ParseCursor(r *http.Request) CursorParams {
	return CursorParams{
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  parseIntClamp(r.URL.Query().Get("limit"), DefaultLimit, 1, MaxLimit),
	}
}

// CursorResponse wraps items with standard cursor-based pagination metadata.
type CursorResponse struct {
	Items      any    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// NewCursorResponse constructs a cursor-paginated response.
func NewCursorResponse(items any, nextCursor string, hasMore bool, limit int) CursorResponse {
	return CursorResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Limit:      limit,
	}
}

// TypedCursorResponse wraps a type-safe slice with cursor-based pagination metadata.
type TypedCursorResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// NewTypedCursorResponse constructs a type-safe cursor paginated response.
func NewTypedCursorResponse[T any](items []T, nextCursor string, hasMore bool, limit int) TypedCursorResponse[T] {
	return TypedCursorResponse[T]{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Limit:      limit,
	}
}

// EncodeCursor encodes a raw string identifier into an opaque, URL-safe base64 cursor token.
func EncodeCursor(val string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(val))
}

// DecodeCursor decodes an opaque URL-safe base64 cursor token back to its raw string identifier.
func DecodeCursor(cursor string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
