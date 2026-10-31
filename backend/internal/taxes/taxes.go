// Package taxes owns tax rules. A rule is data applied at line level when an
// invoice is issued. Rates are stored in basis points so 7.5% is 750 and no
// float ever enters the money path. is_inclusive means prices already contain
// the tax: invoices still record the tax amount, but it is carved out of the
// line total instead of added on top.
package taxes

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

var ErrNotFound = errors.New("tax rate not found")

const MaxRateBP = 100000

type Tax struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	RateBP      int       `json:"rate_bp"`
	IsInclusive bool      `json:"is_inclusive"`
	CountryCode string    `json:"country_code,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, name, rate_bp, is_inclusive, country_code, is_active, created_at, updated_at`

func (r *Repository) Insert(ctx context.Context, q db.Querier, t *Tax) error {
	_, err := q.Exec(ctx, `INSERT INTO taxes (id, name, rate_bp, is_inclusive, country_code)
		VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.Name, t.RateBP, t.IsInclusive, emptyToNil(t.CountryCode))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, taxID uuid.UUID) (*Tax, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM taxes WHERE id = $1`, taxID))
}

func (r *Repository) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Tax, int, error) {
	where := `WHERE ($1 = '' OR lower(name) LIKE '%' || lower($1) || '%')
		AND (NOT $2 OR is_active)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM taxes `+where, search, activeOnly).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM taxes `+where+`
		ORDER BY lower(name), id LIMIT $3 OFFSET $4`, search, activeOnly, page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Tax, 0)
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, q db.Querier, t *Tax) error {
	tag, err := q.Exec(ctx, `UPDATE taxes SET name = $2, rate_bp = $3,
		is_inclusive = $4, country_code = $5, is_active = $6 WHERE id = $1`,
		t.ID, t.Name, t.RateBP, t.IsInclusive, emptyToNil(t.CountryCode), t.IsActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

type Input struct {
	Name        string `json:"name"`
	RateBP      int    `json:"rate_bp"`
	IsInclusive bool   `json:"is_inclusive"`
	CountryCode string `json:"country_code"`
}

type UpdateInput struct {
	Name        *string `json:"name"`
	RateBP      *int    `json:"rate_bp"`
	IsInclusive *bool   `json:"is_inclusive"`
	CountryCode *string `json:"country_code"`
	IsActive    *bool   `json:"is_active"`
}

func (s *Service) Create(ctx context.Context, in Input) (*Tax, error) {
	t, details := build("", in)
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	t.ID = id.New()
	t.IsActive = true
	if err := s.repo.Insert(ctx, s.repo.pool, t); err != nil {
		return nil, err
	}
	return s.Get(ctx, t.ID)
}

func (s *Service) Get(ctx context.Context, taxID uuid.UUID) (*Tax, error) {
	t, err := s.repo.Get(ctx, s.repo.pool, taxID)
	if err != nil {
		return nil, httpError(err)
	}
	return t, nil
}

func (s *Service) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Tax, int, error) {
	return s.repo.List(ctx, strings.TrimSpace(search), activeOnly, page)
}

func (s *Service) Update(ctx context.Context, taxID uuid.UUID, in UpdateInput) (*Tax, error) {
	t, err := s.Get(ctx, taxID)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		t.Name = strings.TrimSpace(*in.Name)
	}
	if in.RateBP != nil {
		t.RateBP = *in.RateBP
	}
	if in.IsInclusive != nil {
		t.IsInclusive = *in.IsInclusive
	}
	if in.CountryCode != nil {
		t.CountryCode = strings.ToUpper(strings.TrimSpace(*in.CountryCode))
	}
	if in.IsActive != nil {
		t.IsActive = *in.IsActive
	}
	if t.Name == "" || t.RateBP < 0 || t.RateBP > MaxRateBP {
		return nil, httpx.Validation(map[string]string{
			"name":    "tax name is required",
			"rate_bp": "rate must be between 0 and 100000 basis points",
		})
	}
	if err := s.repo.Update(ctx, s.repo.pool, t); err != nil {
		return nil, httpError(err)
	}
	return s.Get(ctx, taxID)
}

func (s *Service) Deactivate(ctx context.Context, taxID uuid.UUID) (*Tax, error) {
	inactive := false
	return s.Update(ctx, taxID, UpdateInput{IsActive: &inactive})
}

// build validates an Input. Existing fields are carried from the current tax on
// update; the first parameter is only set for validation messages.
func build(_ string, in Input) (*Tax, map[string]string) {
	details := map[string]string{}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		details["name"] = "tax name is required"
	}
	if in.RateBP < 0 || in.RateBP > MaxRateBP {
		details["rate_bp"] = "rate must be between 0 and 100000 basis points"
	}
	code := strings.ToUpper(strings.TrimSpace(in.CountryCode))
	if code != "" && len(code) != 2 {
		details["country_code"] = "use a two letter ISO country code"
	}
	return &Tax{
		Name: name, RateBP: in.RateBP, IsInclusive: in.IsInclusive,
		CountryCode: code,
	}, details
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	var in Input
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, t)
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	items, total, err := h.svc.List(c.Request.Context(), c.Query("search"), c.Query("active") == "true", page)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	taxID, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.Get(c.Request.Context(), taxID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *Handler) Update(c *gin.Context) {
	taxID, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	t, err := h.svc.Update(c.Request.Context(), taxID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func (h *Handler) Deactivate(c *gin.Context) {
	taxID, ok := parseID(c)
	if !ok {
		return
	}
	t, err := h.svc.Deactivate(c.Request.Context(), taxID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, t)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	taxID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid tax id"))
		return uuid.Nil, false
	}
	return taxID, true
}

func scan(row pgx.Row) (*Tax, error) {
	var t Tax
	code := pgtype.Text{}
	err := row.Scan(&t.ID, &t.Name, &t.RateBP, &t.IsInclusive, &code, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if code.Valid {
		t.CountryCode = code.String
	}
	return &t, nil
}

func httpError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("tax rate not found")
	}
	return err
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
