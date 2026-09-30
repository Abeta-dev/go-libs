// SPDX-License-Identifier: MIT

package pagination_test

import (
	"testing"

	"github.com/umesh0492/go-libs/pagination"
)

func TestPaginateSlice(t *testing.T) {
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// First page of 3
	p1, meta1 := pagination.PaginateSlice(data, 1, 3)
	if len(p1) != 3 || p1[0] != 1 || p1[2] != 3 {
		t.Fatalf("unexpected p1: %v", p1)
	}
	if meta1.Total != 10 || meta1.TotalPages != 4 || !meta1.HasMore {
		t.Fatalf("unexpected meta1: %+v", meta1)
	}

	// Last page
	p4, meta4 := pagination.PaginateSlice(data, 4, 3)
	if len(p4) != 1 || p4[0] != 10 {
		t.Fatalf("unexpected p4: %v", p4)
	}
	if meta4.HasMore {
		t.Fatal("expected HasMore to be false on last page")
	}

	// Out of bounds page
	p5, meta5 := pagination.PaginateSlice(data, 5, 3)
	if len(p5) != 0 {
		t.Fatalf("expected empty slice for out of bounds page, got: %v", p5)
	}
	if meta5.HasMore {
		t.Fatal("expected HasMore false for out of bounds page")
	}

	// Empty slice
	pEmpty, metaEmpty := pagination.PaginateSlice([]string{}, 1, 10)
	if len(pEmpty) != 0 || metaEmpty.Total != 0 || metaEmpty.HasMore {
		t.Fatalf("unexpected empty pagination: %v, %+v", pEmpty, metaEmpty)
	}

	// Negative / zero limit
	pAll, metaAll := pagination.PaginateSlice(data, 1, 0)
	if len(pAll) != 10 || metaAll.Total != 10 {
		t.Fatalf("expected all items when limit <= 0, got: %v", pAll)
	}
}
