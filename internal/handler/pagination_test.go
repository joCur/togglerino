package handler

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	limit, offset := parsePagination(req)
	if limit != 50 {
		t.Errorf("default limit: got %d, want 50", limit)
	}
	if offset != 0 {
		t.Errorf("default offset: got %d, want 0", offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?limit=25&offset=10", nil)
	limit, offset := parsePagination(req)
	if limit != 25 {
		t.Errorf("custom limit: got %d, want 25", limit)
	}
	if offset != 10 {
		t.Errorf("custom offset: got %d, want 10", offset)
	}
}

func TestParsePagination_ClampsLimitOver100(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?limit=200", nil)
	limit, _ := parsePagination(req)
	if limit != 100 {
		t.Errorf("clamped limit: got %d, want 100", limit)
	}
}

func TestParsePagination_RejectsNegativeLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?limit=-5", nil)
	limit, _ := parsePagination(req)
	if limit != 50 {
		t.Errorf("negative limit should use default: got %d, want 50", limit)
	}
}

func TestParsePagination_RejectsNegativeOffset(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?offset=-1", nil)
	_, offset := parsePagination(req)
	if offset != 0 {
		t.Errorf("negative offset should use default: got %d, want 0", offset)
	}
}
