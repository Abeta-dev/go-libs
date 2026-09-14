// SPDX-License-Identifier: MIT

package pagination_test

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/umesh0492/go-libs/pagination"
)

func FuzzParse(f *testing.F) {
	// Seed corpus with common, edge-case, and adversarial inputs
	f.Add("1", "20")
	f.Add("0", "0")
	f.Add("-1", "-50")
	f.Add("1000000", "999999")
	f.Add("abc", "xyz")
	f.Add("", "")
	f.Add("9223372036854775807", "9223372036854775807")
	f.Add("-9223372036854775808", "-9223372036854775808")
	f.Add("   12   ", "   50   ")
	f.Add("0x10", "1e5")
	f.Add("%00%ff", "%20")

	f.Fuzz(func(t *testing.T, pageStr, limitStr string) {
		reqURL := fmt.Sprintf("/?page=%s&limit=%s", url.QueryEscape(pageStr), url.QueryEscape(limitStr))
		r := httptest.NewRequest("GET", reqURL, nil)

		p := pagination.Parse(r)

		if p.Page < 1 || p.Page > 10_000 {
			t.Fatalf("Page out of bounds [1, 10000]: got %d for pageStr=%q", p.Page, pageStr)
		}
		if p.Limit < 1 || p.Limit > pagination.MaxLimit {
			t.Fatalf("Limit out of bounds [1, %d]: got %d for limitStr=%q", pagination.MaxLimit, p.Limit, limitStr)
		}

		offset := p.Offset()
		if offset < 0 {
			t.Fatalf("Offset must be non-negative: got %d", offset)
		}
	})
}

func FuzzTotalPages(f *testing.F) {
	f.Add(100, 20)
	f.Add(0, 20)
	f.Add(-100, 20)
	f.Add(100, 0)
	f.Add(100, -10)
	f.Add(0, 0)
	f.Add(-1, -1)
	f.Add(1, 1)
	f.Add(101, 20)
	f.Add(1_000_000, 10)

	f.Fuzz(func(t *testing.T, total, limit int) {
		pages := pagination.TotalPages(total, limit)
		if limit <= 0 || total <= 0 {
			if pages != 0 {
				t.Fatalf("expected 0 pages for total=%d limit=%d, got %d", total, limit, pages)
			}
			return
		}
		if pages < 1 {
			t.Fatalf("expected >= 1 pages for positive total=%d limit=%d, got %d", total, limit, pages)
		}
	})
}
