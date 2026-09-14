// SPDX-License-Identifier: MIT

package pagination_test

import (
	"fmt"
	"net/http/httptest"

	"github.com/umesh0492/go-libs/pagination"
)

func ExampleParse() {
	req := httptest.NewRequest("GET", "/api/items?page=3&limit=25", nil)
	params := pagination.Parse(req)

	fmt.Printf("page: %d, limit: %d, offset: %d\n", params.Page, params.Limit, params.Offset())

	// Output:
	// page: 3, limit: 25, offset: 50
}

func ExampleNewTypedResponse() {
	params := pagination.Params{Page: 1, Limit: 10}
	items := []string{"apple", "banana", "cherry"}

	resp := pagination.NewTypedResponse(items, 25, params)
	fmt.Printf("total: %d, pages: %d, items_count: %d\n", resp.Total, resp.Pages, len(resp.Items))

	// Output:
	// total: 25, pages: 3, items_count: 3
}

func ExampleParseCursor() {
	rawCursor := "order_9999"
	encodedCursor := pagination.EncodeCursor(rawCursor)

	req := httptest.NewRequest("GET", "/api/orders?cursor="+encodedCursor+"&limit=15", nil)
	params := pagination.ParseCursor(req)

	decoded, _ := pagination.DecodeCursor(params.Cursor)
	fmt.Printf("cursor: %s, limit: %d\n", decoded, params.Limit)

	// Output:
	// cursor: order_9999, limit: 15
}
