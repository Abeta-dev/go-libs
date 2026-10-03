// SPDX-License-Identifier: MIT

package pagination

// PageMeta provides standardized pagination metadata for paginated API responses.
type PageMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// PaginateSlice safely bounds and slices any generic slice [T any],
// eliminating duplicate slice slicing and fixing the silent overflow bug
// where requesting out-of-range pages returned the entire unfiltered dataset.
func PaginateSlice[T any](items []T, page, limit int) ([]T, PageMeta) {
	total := len(items)
	if limit <= 0 {
		return items, PageMeta{
			Page:       1,
			Limit:      total,
			Total:      total,
			TotalPages: 1,
			HasMore:    false,
		}
	}
	if page <= 0 {
		page = 1
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	start := (page - 1) * limit
	if start >= total {
		return []T{}, PageMeta{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
			HasMore:    false,
		}
	}

	end := start + limit
	if end > total {
		end = total
	}

	return items[start:end], PageMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasMore:    end < total,
	}
}
