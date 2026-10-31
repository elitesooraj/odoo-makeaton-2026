package pagination

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseDefaultsAndClamps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		query       string
		wantPage    int
		wantPerPage int
	}{
		{"", 1, DefaultPerPage},
		{"?page=3&per_page=10", 3, 10},
		{"?page=0&per_page=0", 1, DefaultPerPage},
		{"?page=-4&per_page=abc", 1, DefaultPerPage},
		{"?per_page=5000", 1, MaxPerPage},
	}

	for _, tt := range tests {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/customers"+tt.query, nil)
		page := Parse(c)
		if page.Number != tt.wantPage || page.PerPage != tt.wantPerPage {
			t.Errorf("%q: got page=%d per_page=%d, want %d/%d",
				tt.query, page.Number, page.PerPage, tt.wantPage, tt.wantPerPage)
		}
	}
}

func TestOffset(t *testing.T) {
	page := Page{Number: 3, PerPage: 25}
	if got := page.Offset(); got != 50 {
		t.Fatalf("offset = %d, want 50", got)
	}
}

func TestMeta(t *testing.T) {
	page := Page{Number: 2, PerPage: 25}
	meta := Meta(page, 51)
	if meta["total"] != 51 || meta["total_pages"] != 3 {
		t.Fatalf("unexpected meta: %v", meta)
	}
	if empty := Meta(page, 0); empty["total_pages"] != 0 {
		t.Fatalf("empty result should have zero pages: %v", empty)
	}
}
