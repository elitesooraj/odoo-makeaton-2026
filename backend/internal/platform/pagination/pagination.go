// Package pagination parses page/per_page query parameters and builds the
// meta block returned with list responses.
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPerPage = 25
	MaxPerPage     = 100
)

type Page struct {
	Number  int
	PerPage int
}

// Parse reads ?page= and ?per_page=, clamping both to safe values so a client
// cannot request an unbounded result set.
func Parse(c *gin.Context) Page {
	number := positiveInt(c.Query("page"), 1)
	perPage := positiveInt(c.Query("per_page"), DefaultPerPage)
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	return Page{Number: number, PerPage: perPage}
}

func (p Page) Offset() int {
	return (p.Number - 1) * p.PerPage
}

// Meta is merged into the response envelope by httpx.List.
func Meta(p Page, total int) map[string]any {
	totalPages := 0
	if total > 0 {
		totalPages = (total + p.PerPage - 1) / p.PerPage
	}
	return map[string]any{
		"page":        p.Number,
		"per_page":    p.PerPage,
		"total":       total,
		"total_pages": totalPages,
	}
}

func positiveInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
