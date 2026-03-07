package handler

import (
	"net/http"
	"strconv"
)

// PaginatedResponse wraps a paginated result with metadata.
type PaginatedResponse struct {
	Data       any `json:"data"`
	TotalCount int `json:"total_count"`
	Limit      int `json:"limit"`
	Offset     int `json:"offset"`
}

// parsePagination extracts limit and offset from query parameters.
// Defaults: limit=50, offset=0. Limit is clamped to [1, 100].
// Negative values for limit or offset are rejected and replaced with defaults.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed < 0 {
				// reject negative, keep default
			} else {
				limit = parsed
			}
		}
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed < 0 {
				// reject negative, keep default
			} else {
				offset = parsed
			}
		}
	}

	// Clamp limit to [1, 100]
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	return limit, offset
}
