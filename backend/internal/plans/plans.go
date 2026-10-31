// Package plans owns subscription plans. A plan belongs to one product and
// carries the interval, price and the flags that control subscription behaviour:
// auto_close, auto_renew, allow_pause and allow_early_close.
package plans

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
	"github.com/brightloop/subscription-platform/backend/internal/products"
)

var ErrNotFound = errors.New("plan not found")

type Plan struct {
	ID              uuid.UUID  `json:"id"`
	ProductID       uuid.UUID  `json:"product_id"`
	TaxID           *uuid.UUID `json:"tax_id"`
	Name            string     `json:"name"`
	Interval        string     `json:"interval"`
	PriceCents      int64      `json:"price_cents"`
	MinimumQuantity int        `json:"minimum_quantity"`
	AvailableFrom   *time.Time `json:"available_from"`
	AvailableTo     *time.Time `json:"available_to"`
	AutoClose       bool       `json:"auto_close"`
	AutoRenew       bool       `json:"auto_renew"`
	AllowPause      bool       `json:"allow_pause"`
	AllowEarlyClose bool       `json:"allow_early_close"`
	IsActive        bool       `json:"is_active"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, product_id, tax_id, name, interval, price_cents, minimum_quantity,
	available_from, available_to, auto_close, auto_renew, allow_pause, allow_early_close,
	is_active, created_at, updated_at`

func (r *Repository) Insert(ctx context.Context, q db.Querier, p *Plan) error {
	_, err := q.Exec(ctx, `INSERT INTO plans
		(id, product_id, tax_id, name, interval, price_cents, minimum_quantity,
		 available_from, available_to, auto_close, auto_renew, allow_pause, allow_early_close)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		p.ID, p.ProductID, id.UUIDArg(p.TaxID), p.Name, p.Interval, p.PriceCents, p.MinimumQuantity,
		id.DateArg(p.AvailableFrom), id.DateArg(p.AvailableTo),
		p.AutoClose, p.AutoRenew, p.AllowPause, p.AllowEarlyClose)
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, planID uuid.UUID) (*Plan, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM plans WHERE id = $1`, planID))
}

func (r *Repository) List(ctx context.Context, search string, productID *uuid.UUID, activeOnly bool, page pagination.Page) ([]Plan, int, error) {
	where := `WHERE ($1 = '' OR lower(name) LIKE '%' || lower($1) || '%')
		AND ($2::uuid IS NULL OR product_id = $2)
		AND (NOT $3 OR is_active)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM plans `+where, search, productID, activeOnly).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM plans `+where+`
		ORDER BY lower(name), id LIMIT $4 OFFSET $5`, search, productID, activeOnly, page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Plan, 0)
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, q db.Querier, p *Plan) error {
	tag, err := q.Exec(ctx, `UPDATE plans SET product_id=$2, tax_id=$3, name=$4, interval=$5,
		price_cents=$6, minimum_quantity=$7, available_from=$8, available_to=$9,
		auto_close=$10, auto_renew=$11, allow_pause=$12, allow_early_close=$13, is_active=$14
		WHERE id=$1`,
		p.ID, p.ProductID, id.UUIDArg(p.TaxID), p.Name, p.Interval, p.PriceCents, p.MinimumQuantity,
		id.DateArg(p.AvailableFrom), id.DateArg(p.AvailableTo),
		p.AutoClose, p.AutoRenew, p.AllowPause, p.AllowEarlyClose, p.IsActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type Service struct {
	repo     *Repository
	products *products.Service
}

func NewService(repo *Repository, productsSvc *products.Service) *Service {
	return &Service{repo: repo, products: productsSvc}
}

type Input struct {
	ProductID       string  `json:"product_id"`
	TaxID           *string `json:"tax_id"`
	Name            string  `json:"name"`
	Interval        string  `json:"interval"`
	PriceCents      int64   `json:"price_cents"`
	MinimumQuantity int     `json:"minimum_quantity"`
	AvailableFrom   *string `json:"available_from"`
	AvailableTo     *string `json:"available_to"`
	AutoClose       bool    `json:"auto_close"`
	AutoRenew       bool    `json:"auto_renew"`
	AllowPause      bool    `json:"allow_pause"`
	AllowEarlyClose bool    `json:"allow_early_close"`
}

type UpdateInput struct {
	ProductID       *string `json:"product_id"`
	TaxID           *string `json:"tax_id"`
	Name            *string `json:"name"`
	Interval        *string `json:"interval"`
	PriceCents      *int64  `json:"price_cents"`
	MinimumQuantity *int    `json:"minimum_quantity"`
	AvailableFrom   *string `json:"available_from"`
	AvailableTo     *string `json:"available_to"`
	AutoClose       *bool   `json:"auto_close"`
	AutoRenew       *bool   `json:"auto_renew"`
	AllowPause      *bool   `json:"allow_pause"`
	AllowEarlyClose *bool   `json:"allow_early_close"`
	IsActive        *bool   `json:"is_active"`
}

func (s *Service) Create(ctx context.Context, in Input) (*Plan, error) {
	p, err := s.build(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Insert(ctx, s.repo.pool, p); err != nil {
		return nil, err
	}
	return s.Get(ctx, p.ID)
}

func (s *Service) build(ctx context.Context, in Input) (*Plan, error) {
	details := map[string]string{}
	productID, err := uuid.Parse(strings.TrimSpace(in.ProductID))
	if err != nil {
		details["product_id"] = "a valid product id is required"
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		details["name"] = "plan name is required"
	}
	interval := strings.ToUpper(strings.TrimSpace(in.Interval))
	if !validInterval(interval) {
		details["interval"] = "interval must be DAILY, WEEKLY, MONTHLY or YEARLY"
	}
	if in.PriceCents < 0 {
		details["price_cents"] = "must not be negative"
	}
	minQty := in.MinimumQuantity
	if minQty == 0 {
		minQty = 1
	}
	if minQty < 1 {
		details["minimum_quantity"] = "must be at least 1"
	}
	var taxID *uuid.UUID
	if in.TaxID != nil && strings.TrimSpace(*in.TaxID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.TaxID))
		if err != nil {
			details["tax_id"] = "invalid tax id"
		} else {
			taxID = &parsed
		}
	}
	from, err := parseDate(in.AvailableFrom)
	if err != nil {
		details["available_from"] = "use YYYY-MM-DD"
	}
	to, err := parseDate(in.AvailableTo)
	if err != nil {
		details["available_to"] = "use YYYY-MM-DD"
	}
	if from != nil && to != nil && from.After(*to) {
		details["available_to"] = "must be on or after available_from"
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	if _, err := s.products.Get(ctx, productID); err != nil {
		return nil, err
	}
	return &Plan{
		ID: id.New(), ProductID: productID, TaxID: taxID, Name: name, Interval: interval,
		PriceCents: in.PriceCents, MinimumQuantity: minQty, AvailableFrom: from, AvailableTo: to,
		AutoClose: in.AutoClose, AutoRenew: in.AutoRenew, AllowPause: in.AllowPause,
		AllowEarlyClose: in.AllowEarlyClose, IsActive: true,
	}, nil
}

func (s *Service) Get(ctx context.Context, planID uuid.UUID) (*Plan, error) {
	p, err := s.repo.Get(ctx, s.repo.pool, planID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NotFound("plan not found")
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, search string, productID *uuid.UUID, activeOnly bool, page pagination.Page) ([]Plan, int, error) {
	return s.repo.List(ctx, strings.TrimSpace(search), productID, activeOnly, page)
}

func (s *Service) Update(ctx context.Context, planID uuid.UUID, in UpdateInput) (*Plan, error) {
	p, err := s.Get(ctx, planID)
	if err != nil {
		return nil, err
	}
	details := map[string]string{}
	if in.ProductID != nil {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.ProductID))
		if err != nil {
			details["product_id"] = "invalid product id"
		} else {
			p.ProductID = parsed
		}
	}
	if in.TaxID != nil {
		if strings.TrimSpace(*in.TaxID) == "" {
			p.TaxID = nil
		} else if parsed, err := uuid.Parse(strings.TrimSpace(*in.TaxID)); err != nil {
			details["tax_id"] = "invalid tax id"
		} else {
			p.TaxID = &parsed
		}
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			details["name"] = "plan name is required"
		}
		p.Name = name
	}
	if in.Interval != nil {
		interval := strings.ToUpper(strings.TrimSpace(*in.Interval))
		if !validInterval(interval) {
			details["interval"] = "interval must be DAILY, WEEKLY, MONTHLY or YEARLY"
		}
		p.Interval = interval
	}
	if in.PriceCents != nil {
		if *in.PriceCents < 0 {
			details["price_cents"] = "must not be negative"
		}
		p.PriceCents = *in.PriceCents
	}
	if in.MinimumQuantity != nil {
		if *in.MinimumQuantity < 1 {
			details["minimum_quantity"] = "must be at least 1"
		}
		p.MinimumQuantity = *in.MinimumQuantity
	}
	if in.AvailableFrom != nil {
		if parsed, err := parseDate(in.AvailableFrom); err != nil {
			details["available_from"] = "use YYYY-MM-DD"
		} else {
			p.AvailableFrom = parsed
		}
	}
	if in.AvailableTo != nil {
		if parsed, err := parseDate(in.AvailableTo); err != nil {
			details["available_to"] = "use YYYY-MM-DD"
		} else {
			p.AvailableTo = parsed
		}
	}
	if in.AutoClose != nil {
		p.AutoClose = *in.AutoClose
	}
	if in.AutoRenew != nil {
		p.AutoRenew = *in.AutoRenew
	}
	if in.AllowPause != nil {
		p.AllowPause = *in.AllowPause
	}
	if in.AllowEarlyClose != nil {
		p.AllowEarlyClose = *in.AllowEarlyClose
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
	return s.Get(ctx, planID)
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	var in Input
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
	var productID *uuid.UUID
	if raw := c.Query("product_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(c, httpx.BadRequest("invalid product_id"))
			return
		}
		productID = &parsed
	}
	items, total, err := h.svc.List(c.Request.Context(), c.Query("search"), productID, c.Query("active") == "true", page)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	planID, ok := parseID(c)
	if !ok {
		return
	}
	p, err := h.svc.Get(c.Request.Context(), planID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func (h *Handler) Update(c *gin.Context) {
	planID, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	p, err := h.svc.Update(c.Request.Context(), planID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func (h *Handler) Deactivate(c *gin.Context) {
	planID, ok := parseID(c)
	if !ok {
		return
	}
	inactive := false
	p, err := h.svc.Update(c.Request.Context(), planID, UpdateInput{IsActive: &inactive})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	planID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid plan id"))
		return uuid.Nil, false
	}
	return planID, true
}

func validInterval(interval string) bool {
	switch interval {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
		return true
	}
	return false
}

func parseDate(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scan(row pgx.Row) (*Plan, error) {
	var (
		p        Plan
		taxID    pgtype.UUID
		from, to pgtype.Date
	)
	err := row.Scan(&p.ID, &p.ProductID, &taxID, &p.Name, &p.Interval, &p.PriceCents,
		&p.MinimumQuantity, &from, &to, &p.AutoClose, &p.AutoRenew, &p.AllowPause,
		&p.AllowEarlyClose, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if taxID.Valid {
		v := uuid.UUID(taxID.Bytes)
		p.TaxID = &v
	}
	if from.Valid {
		p.AvailableFrom = &from.Time
	}
	if to.Valid {
		p.AvailableTo = &to.Time
	}
	return &p, nil
}

func httpError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("plan not found")
	}
	return err
}
