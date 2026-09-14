// SPDX-License-Identifier: MIT

package pagination_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/umesh0492/go-libs/pagination"
)

func TestParse_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	p := pagination.Parse(r)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.Limit)
}

func TestParse_ExplicitValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/?page=3&limit=50", nil)
	p := pagination.Parse(r)
	assert.Equal(t, 3, p.Page)
	assert.Equal(t, 50, p.Limit)
}

func TestParse_ClampsMaxLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=999", nil)
	p := pagination.Parse(r)
	assert.Equal(t, pagination.MaxLimit, p.Limit)
}

func TestParse_InvalidPageFallsToDefault(t *testing.T) {
	r := httptest.NewRequest("GET", "/?page=abc", nil)
	p := pagination.Parse(r)
	assert.Equal(t, 1, p.Page)
}

func TestOffset(t *testing.T) {
	p := pagination.Params{Page: 3, Limit: 20}
	assert.Equal(t, 40, p.Offset())
}

func TestTotalPages(t *testing.T) {
	assert.Equal(t, 5, pagination.TotalPages(100, 20))
	assert.Equal(t, 6, pagination.TotalPages(101, 20))
	assert.Equal(t, 1, pagination.TotalPages(1, 20))
	assert.Equal(t, 0, pagination.TotalPages(0, 20))
	assert.Equal(t, 0, pagination.TotalPages(-50, 20))
	assert.Equal(t, 0, pagination.TotalPages(100, 0))
	assert.Equal(t, 0, pagination.TotalPages(100, -10))
}

func TestParse_NegativePage(t *testing.T) {
	r := httptest.NewRequest("GET", "/?page=-5&limit=-20", nil)
	p := pagination.Parse(r)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.Limit)
}

func TestNewResponse(t *testing.T) {
	p := pagination.Params{Page: 2, Limit: 10}
	resp := pagination.NewResponse([]string{"a", "b"}, 25, p)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 10, resp.Limit)
	assert.Equal(t, 3, resp.Pages)
}

func TestNewTypedResponse(t *testing.T) {
	type Order struct {
		ID int
	}
	items := []Order{{ID: 101}, {ID: 102}}
	p := pagination.Params{Page: 1, Limit: 10}
	resp := pagination.NewTypedResponse(items, 15, p)
	assert.Equal(t, 15, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 10, resp.Limit)
	assert.Equal(t, 2, resp.Pages)
	assert.Equal(t, 2, len(resp.Items))
	assert.Equal(t, 101, resp.Items[0].ID)
}

func TestCursorPagination(t *testing.T) {
	// 1. Parse defaults
	reqDefault := httptest.NewRequest("GET", "/api/items", nil)
	p1 := pagination.ParseCursor(reqDefault)
	assert.Equal(t, "", p1.Cursor)
	assert.Equal(t, pagination.DefaultLimit, p1.Limit)

	// 2. Parse with query params & clamping
	reqCustom := httptest.NewRequest("GET", "/api/items?cursor=eyJpZCI6MTIzfQ&limit=50", nil)
	p2 := pagination.ParseCursor(reqCustom)
	assert.Equal(t, "eyJpZCI6MTIzfQ", p2.Cursor)
	assert.Equal(t, 50, p2.Limit)

	// 3. NewCursorResponse
	resp := pagination.NewCursorResponse([]string{"a", "b"}, "next-tok", true, 2)
	assert.Equal(t, "next-tok", resp.NextCursor)
	assert.True(t, resp.HasMore)
	assert.Equal(t, 2, resp.Limit)

	// 4. NewTypedCursorResponse
	typedResp := pagination.NewTypedCursorResponse([]int{1, 2, 3}, "next-3", false, 10)
	assert.Equal(t, "next-3", typedResp.NextCursor)
	assert.False(t, typedResp.HasMore)
	assert.Equal(t, 10, typedResp.Limit)
	assert.Equal(t, 3, len(typedResp.Items))

	// 5. EncodeCursor & DecodeCursor round-trip
	orig := "user_2026_09_07_xyz"
	encoded := pagination.EncodeCursor(orig)
	assert.NotEmpty(t, encoded)
	decoded, err := pagination.DecodeCursor(encoded)
	assert.NoError(t, err)
	assert.Equal(t, orig, decoded)

	// 6. Decode invalid base64 cursor
	_, err = pagination.DecodeCursor("!@#$%^&*()")
	assert.Error(t, err)
}
