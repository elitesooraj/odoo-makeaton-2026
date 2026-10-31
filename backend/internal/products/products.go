// Package products owns the product catalog and its named variants.
package products

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

var ErrNotFound = errors.New("product not found")

type Product struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	SalesPriceCents int64     `json:"sales_price_cents"`
	CostPriceCents  int64     `json:"cost_price_cents"`
	Notes           *string   `json:"notes"`
	IsActive        bool      `json:"is_active"`
	Variants        []Variant `json:"variants,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Variant struct {
	ID              uuid.UUID `json:"id"`
	ProductID       uuid.UUID `json:"product_id"`
	Name            string    `json:"name"`
	PriceDeltaCents int64     `json:"price_delta_cents"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const productColumns = `id, name, type, sales_price_cents, cost_price_cents, notes, is_active, created_at, updated_at`
const variantColumns = `id, product_id, name, price_delta_cents, is_active, created_at, updated_at`

func (r *Repository) Create(ctx context.Context, q db.Querier, p *Product) error {
	_, err := q.Exec(ctx, `INSERT INTO products
		(id, name, type, sales_price_cents, cost_price_cents, notes)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.Name, p.Type, p.SalesPriceCents, p.CostPriceCents, textArg(p.Notes))
	return err
}

func (r *Repository) CreateVariant(ctx context.Context, q db.Querier, v *Variant) error {
	_, err := q.Exec(ctx, `INSERT INTO product_variants
		(id, product_id, name, price_delta_cents)
		VALUES ($1, $2, $3, $4)`,
		v.ID, v.ProductID, v.Name, v.PriceDeltaCents)
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, productID uuid.UUID) (*Product, error) {
	return scanProduct(q.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id = $1`, productID))
}

func (r *Repository) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Product, int, error) {
	const where = `WHERE ($1 = '' OR lower(name) LIKE '%' || lower($1) || '%')
		AND (NOT $2 OR is_active)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM products `+where, search, activeOnly).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+productColumns+` FROM products `+where+`
		ORDER BY lower(name), id LIMIT $3 OFFSET $4`, search, activeOnly, page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

func (r *Repository) Variants(ctx context.Context, q db.Querier, productID uuid.UUID) ([]Variant, error) {
	rows, err := q.Query(ctx, `SELECT `+variantColumns+` FROM product_variants
		WHERE product_id = $1 ORDER BY name`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Variant, 0)
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

func (r *Repository) Update(ctx context.Context, q db.Querier, p *Product) error {
	tag, err := q.Exec(ctx, `UPDATE products SET name = $2, type = $3,
		sales_price_cents = $4, cost_price_cents = $5, notes = $6, is_active = $7
		WHERE id = $1`,
		p.ID, p.Name, p.Type, p.SalesPriceCents, p.CostPriceCents, textArg(p.Notes), p.IsActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetVariant(ctx context.Context, q db.Querier, productID, variantID uuid.UUID) (*Variant, error) {
	return scanVariant(q.QueryRow(ctx, `SELECT `+variantColumns+` FROM product_variants
		WHERE id = $1 AND product_id = $2`, variantID, productID))
}

func (r *Repository) UpdateVariant(ctx context.Context, q db.Querier, v *Variant) error {
	tag, err := q.Exec(ctx, `UPDATE product_variants SET name = $3, price_delta_cents = $4, is_active = $5
		WHERE id = $1 AND product_id = $2`,
		v.ID, v.ProductID, v.Name, v.PriceDeltaCents, v.IsActive)
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

type VariantInput struct {
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"price_delta_cents"`
}

type CreateInput struct {
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	SalesPriceCents int64          `json:"sales_price_cents"`
	CostPriceCents  int64          `json:"cost_price_cents"`
	Notes           string         `json:"notes"`
	Variants        []VariantInput `json:"variants"`
}

type UpdateInput struct {
	Name            *string `json:"name"`
	Type            *string `json:"type"`
	SalesPriceCents *int64  `json:"sales_price_cents"`
	CostPriceCents  *int64  `json:"cost_price_cents"`
	Notes           *string `json:"notes"`
	IsActive        *bool   `json:"is_active"`
}

type VariantUpdateInput struct {
	Name            *string `json:"name"`
	PriceDeltaCents *int64  `json:"price_delta_cents"`
	IsActive        *bool   `json:"is_active"`
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Product, error) {
	details := map[string]string{}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		details["name"] = "product name is required"
	}
	kind := strings.ToUpper(strings.TrimSpace(in.Type))
	if kind == "" {
		kind = "SERVICE"
	}
	if kind != "SERVICE" && kind != "GOODS" {
		details["type"] = "type must be SERVICE or GOODS"
	}
	if in.SalesPriceCents < 0 {
		details["sales_price_cents"] = "must not be negative"
	}
	if in.CostPriceCents < 0 {
		details["cost_price_cents"] = "must not be negative"
	}
	seen := map[string]struct{}{}
	for _, v := range in.Variants {
		if strings.TrimSpace(v.Name) == "" {
			details["variants"] = "every variant needs a name"
		}
		if _, dup := seen[strings.ToLower(v.Name)]; dup {
			details["variants"] = "variant names must be unique"
		}
		seen[strings.ToLower(v.Name)] = struct{}{}
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	p := &Product{
		ID:              id.New(),
		Name:            name,
		Type:            kind,
		SalesPriceCents: in.SalesPriceCents,
		CostPriceCents:  in.CostPriceCents,
		Notes:           optional(strings.TrimSpace(in.Notes)),
		IsActive:        true,
	}

	err := db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.Create(ctx, tx, p); err != nil {
			return err
		}
		for _, v := range in.Variants {
			if err := s.repo.CreateVariant(ctx, tx, &Variant{
				ID: id.New(), ProductID: p.ID,
				Name: strings.TrimSpace(v.Name), PriceDeltaCents: v.PriceDeltaCents, IsActive: true,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, p.ID)
}

func (s *Service) Get(ctx context.Context, productID uuid.UUID) (*Product, error) {
	p, err := s.repo.Get(ctx, s.repo.pool, productID)
	if err != nil {
		return nil, httpError(err)
	}
	variants, err := s.repo.Variants(ctx, s.repo.pool, productID)
	if err != nil {
		return nil, err
	}
	p.Variants = variants
	return p, nil
}

func (s *Service) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Product, int, error) {
	return s.repo.List(ctx, strings.TrimSpace(search), activeOnly, page)
}

func (s *Service) Update(ctx context.Context, productID uuid.UUID, in UpdateInput) (*Product, error) {
	p, err := s.repo.Get(ctx, s.repo.pool, productID)
	if err != nil {
		return nil, httpError(err)
	}
	details := map[string]string{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			details["name"] = "product name is required"
		}
		p.Name = name
	}
	if in.Type != nil {
		kind := strings.ToUpper(strings.TrimSpace(*in.Type))
		if kind != "SERVICE" && kind != "GOODS" {
			details["type"] = "type must be SERVICE or GOODS"
		}
		p.Type = kind
	}
	if in.SalesPriceCents != nil {
		if *in.SalesPriceCents < 0 {
			details["sales_price_cents"] = "must not be negative"
		}
		p.SalesPriceCents = *in.SalesPriceCents
	}
	if in.CostPriceCents != nil {
		if *in.CostPriceCents < 0 {
			details["cost_price_cents"] = "must not be negative"
		}
		p.CostPriceCents = *in.CostPriceCents
	}
	if in.Notes != nil {
		p.Notes = optional(strings.TrimSpace(*in.Notes))
	}
	if in.IsActive != nil {
		p.IsActive = *in.IsActive
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	if err := s.repo.Update(ctx, s.repo.pool, p); err != nil {
		return nil, httpError(err)
	}
	return s.Get(ctx, productID)
}

func (s *Service) Deactivate(ctx context.Context, productID uuid.UUID) (*Product, error) {
	p, err := s.Get(ctx, productID)
	if err != nil {
		return nil, err
	}
	p.IsActive = false
	if err := s.repo.Update(ctx, s.repo.pool, p); err != nil {
		return nil, httpError(err)
	}
	return s.Get(ctx, productID)
}

func (s *Service) AddVariant(ctx context.Context, productID uuid.UUID, in VariantInput) (*Variant, error) {
	if _, err := s.repo.Get(ctx, s.repo.pool, productID); err != nil {
		return nil, httpError(err)
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, httpx.Validation(map[string]string{"name": "variant name is required"})
	}
	v := &Variant{ID: id.New(), ProductID: productID, Name: name, PriceDeltaCents: in.PriceDeltaCents, IsActive: true}
	if err := s.repo.CreateVariant(ctx, s.repo.pool, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *Service) UpdateVariant(ctx context.Context, productID, variantID uuid.UUID, in VariantUpdateInput) (*Variant, error) {
	v, err := s.repo.GetVariant(ctx, s.repo.pool, productID, variantID)
	if err != nil {
		return nil, httpError(err)
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, httpx.Validation(map[string]string{"name": "variant name is required"})
		}
		v.Name = name
	}
	if in.PriceDeltaCents != nil {
		v.PriceDeltaCents = *in.PriceDeltaCents
	}
	if in.IsActive != nil {
		v.IsActive = *in.IsActive
	}
	if err := s.repo.UpdateVariant(ctx, s.repo.pool, v); err != nil {
		return nil, httpError(err)
	}
	return v, nil
}

func (s *Service) DeactivateVariant(ctx context.Context, productID, variantID uuid.UUID) (*Variant, error) {
	v, err := s.repo.GetVariant(ctx, s.repo.pool, productID, variantID)
	if err != nil {
		return nil, httpError(err)
	}
	v.IsActive = false
	if err := s.repo.UpdateVariant(ctx, s.repo.pool, v); err != nil {
		return nil, httpError(err)
	}
	return v, nil
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	var in CreateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	p, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, p)
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	activeOnly := c.Query("active") == "true"
	items, total, err := h.svc.List(c.Request.Context(), c.Query("search"), activeOnly, page)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	productID, ok := parseID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.Get(c.Request.Context(), productID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func (h *Handler) Update(c *gin.Context) {
	productID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	p, err := h.svc.Update(c.Request.Context(), productID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func (h *Handler) Deactivate(c *gin.Context) {
	productID, ok := parseID(c, "id")
	if !ok {
		return
	}
	p, err := h.svc.Deactivate(c.Request.Context(), productID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func (h *Handler) AddVariant(c *gin.Context) {
	productID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in VariantInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	v, err := h.svc.AddVariant(c.Request.Context(), productID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, v)
}

func (h *Handler) UpdateVariant(c *gin.Context) {
	productID, ok := parseID(c, "id")
	if !ok {
		return
	}
	variantID, ok := parseID(c, "variant_id")
	if !ok {
		return
	}
	var in VariantUpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	v, err := h.svc.UpdateVariant(c.Request.Context(), productID, variantID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, v)
}

func (h *Handler) DeactivateVariant(c *gin.Context) {
	productID, ok := parseID(c, "id")
	if !ok {
		return
	}
	variantID, ok := parseID(c, "variant_id")
	if !ok {
		return
	}
	v, err := h.svc.DeactivateVariant(c.Request.Context(), productID, variantID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, v)
}

func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	value, err := uuid.Parse(c.Param(name))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid "+name))
		return uuid.Nil, false
	}
	return value, true
}

func scanProduct(row pgx.Row) (*Product, error) {
	var p Product
	var notes pgtype.Text
	err := row.Scan(&p.ID, &p.Name, &p.Type, &p.SalesPriceCents, &p.CostPriceCents,
		&notes, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if notes.Valid {
		p.Notes = &notes.String
	}
	return &p, nil
}

func scanVariant(row pgx.Row) (*Variant, error) {
	var v Variant
	err := row.Scan(&v.ID, &v.ProductID, &v.Name, &v.PriceDeltaCents, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

func httpError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("product or variant not found")
	}
	return err
}

func textArg(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
