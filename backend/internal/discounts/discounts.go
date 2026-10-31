// Package discounts owns discount rules and their redemption ledger.
//
// A rule is FIXED (value_cents) or PERCENT (value_bp), scoped to the whole
// subscription or restricted to one product, guarded by minimum purchase,
// minimum quantity and an activity window, and optionally limited by max_uses.
// Rule matching and redemption counting happen at invoice time; the invoices
// module calls CheckForSubscription and RecordApplication.
package discounts

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

var ErrNotFound = errors.New("discount not found")

// MaxPercentBP is the cap for percentage rules: 100.00%.
const MaxPercentBP = 10000

type Discount struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	Kind             string     `json:"kind"`
	ValueCents       int64      `json:"value_cents"`
	ValueBP          int        `json:"value_bp"`
	Scope            string     `json:"scope"`
	ProductID        *uuid.UUID `json:"product_id"`
	MinPurchaseCents int64      `json:"min_purchase_cents"`
	MinQuantity      int        `json:"min_quantity"`
	StartsAt         *time.Time `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at"`
	MaxUses          *int       `json:"max_uses"`
	UsedCount        int        `json:"used_count"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, name, kind, value_cents, value_bp, scope, product_id,
	min_purchase_cents, min_quantity, starts_at, ends_at, max_uses, used_count,
	is_active, created_at, updated_at`

func (r *Repository) Insert(ctx context.Context, q db.Querier, d *Discount) error {
	_, err := q.Exec(ctx, `INSERT INTO discounts
		(id, name, kind, value_cents, value_bp, scope, product_id,
		 min_purchase_cents, min_quantity, starts_at, ends_at, max_uses)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		d.ID, d.Name, d.Kind, d.ValueCents, d.ValueBP, d.Scope, id.UUIDArg(d.ProductID),
		d.MinPurchaseCents, d.MinQuantity, id.DateArg(d.StartsAt), id.DateArg(d.EndsAt),
		intPtrArg(d.MaxUses))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, discountID uuid.UUID) (*Discount, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM discounts WHERE id = $1`, discountID))
}

func (r *Repository) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Discount, int, error) {
	where := `WHERE ($1 = '' OR lower(name) LIKE '%' || lower($1) || '%')
		AND (NOT $2 OR is_active)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM discounts `+where, search, activeOnly).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM discounts `+where+`
		ORDER BY lower(name), id LIMIT $3 OFFSET $4`, search, activeOnly, page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Discount, 0)
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *d)
	}
	return out, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, q db.Querier, d *Discount) error {
	tag, err := q.Exec(ctx, `UPDATE discounts SET name=$2, kind=$3, value_cents=$4, value_bp=$5,
		scope=$6, product_id=$7, min_purchase_cents=$8, min_quantity=$9, starts_at=$10,
		ends_at=$11, max_uses=$12, is_active=$13
		WHERE id=$1`,
		d.ID, d.Name, d.Kind, d.ValueCents, d.ValueBP, d.Scope, id.UUIDArg(d.ProductID),
		d.MinPurchaseCents, d.MinQuantity, id.DateArg(d.StartsAt), id.DateArg(d.EndsAt),
		intPtrArg(d.MaxUses), d.IsActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// IncrementUsed bumps the redemption counter in the invoice transaction.
func (r *Repository) IncrementUsed(ctx context.Context, q db.Querier, discountID uuid.UUID) error {
	tag, err := q.Exec(ctx, `UPDATE discounts SET used_count = used_count + 1
		WHERE id = $1 AND (max_uses IS NULL OR used_count < max_uses)`, discountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("max uses reached")
	}
	return nil
}

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

type Input struct {
	Name             string  `json:"name"`
	Kind             string  `json:"kind"`
	ValueCents       int64   `json:"value_cents"`
	ValueBP          int     `json:"value_bp"`
	Scope            string  `json:"scope"`
	ProductID        *string `json:"product_id"`
	MinPurchaseCents int64   `json:"min_purchase_cents"`
	MinQuantity      int     `json:"min_quantity"`
	StartsAt         *string `json:"starts_at"`
	EndsAt           *string `json:"ends_at"`
	MaxUses          *int    `json:"max_uses"`
}

type UpdateInput struct {
	Name             *string `json:"name"`
	Kind             *string `json:"kind"`
	ValueCents       *int64  `json:"value_cents"`
	ValueBP          *int    `json:"value_bp"`
	Scope            *string `json:"scope"`
	ProductID        *string `json:"product_id"`
	MinPurchaseCents *int64  `json:"min_purchase_cents"`
	MinQuantity      *int    `json:"min_quantity"`
	StartsAt         *string `json:"starts_at"`
	EndsAt           *string `json:"ends_at"`
	MaxUses          *int    `json:"max_uses"`
	IsActive         *bool   `json:"is_active"`
}

func (s *Service) Create(ctx context.Context, in Input) (*Discount, error) {
	d, details := validate("", in, nil)
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	d.ID = id.New()
	d.IsActive = true
	if err := s.repo.Insert(ctx, s.repo.pool, d); err != nil {
		return nil, err
	}
	return s.Get(ctx, d.ID)
}

func (s *Service) Get(ctx context.Context, discountID uuid.UUID) (*Discount, error) {
	d, err := s.repo.Get(ctx, s.repo.pool, discountID)
	if err != nil {
		return nil, httpError(err)
	}
	return d, nil
}

func (s *Service) List(ctx context.Context, search string, activeOnly bool, page pagination.Page) ([]Discount, int, error) {
	return s.repo.List(ctx, strings.TrimSpace(search), activeOnly, page)
}

func (s *Service) Update(ctx context.Context, discountID uuid.UUID, in UpdateInput) (*Discount, error) {
	current, err := s.Get(ctx, discountID)
	if err != nil {
		return nil, err
	}
	// Reuse the create validator by folding patch fields back into an Input.
	base := Input{
		Name:             current.Name,
		Kind:             current.Kind,
		ValueCents:       current.ValueCents,
		ValueBP:          current.ValueBP,
		Scope:            current.Scope,
		MinPurchaseCents: current.MinPurchaseCents,
		MinQuantity:      current.MinQuantity,
	}
	if in.Name != nil {
		base.Name = *in.Name
	}
	if in.Kind != nil {
		base.Kind = *in.Kind
	}
	if in.ValueCents != nil {
		base.ValueCents = *in.ValueCents
	}
	if in.ValueBP != nil {
		base.ValueBP = *in.ValueBP
	}
	if in.Scope != nil {
		base.Scope = *in.Scope
	}
	if in.ProductID != nil {
		trimmed := strings.TrimSpace(*in.ProductID)
		if trimmed == "" {
			base.ProductID = nil
		} else {
			base.ProductID = in.ProductID
		}
	} else if current.ProductID != nil {
		pid := current.ProductID.String()
		base.ProductID = &pid
	}
	if in.MinPurchaseCents != nil {
		base.MinPurchaseCents = *in.MinPurchaseCents
	}
	if in.MinQuantity != nil {
		base.MinQuantity = *in.MinQuantity
	}
	if in.StartsAt != nil {
		base.StartsAt = in.StartsAt
	}
	if in.EndsAt != nil {
		base.EndsAt = in.EndsAt
	}
	if in.MaxUses != nil {
		base.MaxUses = in.MaxUses
	}

	updated, details := validate(discountID.String(), base, in.IsActive)
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	updated.ID = current.ID
	updated.IsActive = current.IsActive
	updated.UsedCount = current.UsedCount
	if in.IsActive != nil {
		updated.IsActive = *in.IsActive
	}
	if in.StartsAt == nil {
		updated.StartsAt = current.StartsAt
	}
	if in.EndsAt == nil {
		updated.EndsAt = current.EndsAt
	}
	if in.MaxUses == nil {
		updated.MaxUses = current.MaxUses
	}
	if err := s.repo.Update(ctx, s.repo.pool, updated); err != nil {
		return nil, httpError(err)
	}
	return s.Get(ctx, discountID)
}

func (s *Service) Deactivate(ctx context.Context, discountID uuid.UUID) (*Discount, error) {
	inactive := false
	return s.Update(ctx, discountID, UpdateInput{IsActive: &inactive})
}

// CheckForSubscription validates a rule against a prospective purchase and
// returns a client-facing validation error when it cannot apply. It is called
// by the invoices module when attaching a discount to an invoice.
func (s *Service) CheckForSubscription(ctx context.Context, discountID uuid.UUID, now time.Time, subtotalCents int64, totalQuantity int, productIDs []uuid.UUID) (*Discount, error) {
	d, err := s.Get(ctx, discountID)
	if err != nil {
		return nil, err
	}
	details := map[string]string{}
	if !d.IsActive {
		details["discount_id"] = "discount is not active"
	}
	if d.StartsAt != nil && now.Before(*d.StartsAt) {
		details["discount_id"] = "discount has not started yet"
	}
	if d.EndsAt != nil && now.After(*d.EndsAt) {
		details["discount_id"] = "discount has expired"
	}
	if d.MaxUses != nil && d.UsedCount >= *d.MaxUses {
		details["discount_id"] = "discount usage limit reached"
	}
	if subtotalCents < d.MinPurchaseCents {
		details["min_purchase_cents"] = "invoice total is below the discount minimum"
	}
	if totalQuantity < d.MinQuantity {
		details["min_quantity"] = "quantity is below the discount minimum"
	}
	if d.Scope == "PRODUCT" {
		if d.ProductID != nil {
			matched := false
			for _, pid := range productIDs {
				if pid == *d.ProductID {
					matched = true
					break
				}
			}
			if !matched {
				details["discount_id"] = "discount is limited to a product not on this invoice"
			}
		}
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}
	return d, nil
}

// IncrementUsed is exposed for the invoices module's transaction.
func (s *Service) IncrementUsed(ctx context.Context, q db.Querier, discountID uuid.UUID) error {
	return s.repo.IncrementUsed(ctx, q, discountID)
}

// validate assembles a Discount from an Input. On update, isActive is patched
// by the caller afterwards.
func validate(_ string, in Input, _ *bool) (*Discount, map[string]string) {
	details := map[string]string{}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		details["name"] = "discount name is required"
	}
	kind := strings.ToUpper(strings.TrimSpace(in.Kind))
	if kind == "" {
		kind = "PERCENT"
	}
	if kind != "FIXED" && kind != "PERCENT" {
		details["kind"] = "kind must be FIXED or PERCENT"
	}
	scope := strings.ToUpper(strings.TrimSpace(in.Scope))
	if scope == "" {
		scope = "SUBSCRIPTION"
	}
	if scope != "SUBSCRIPTION" && scope != "PRODUCT" {
		details["scope"] = "scope must be SUBSCRIPTION or PRODUCT"
	}

	var productID *uuid.UUID
	if in.ProductID != nil && strings.TrimSpace(*in.ProductID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.ProductID))
		if err != nil {
			details["product_id"] = "invalid product id"
		} else {
			productID = &parsed
		}
	}
	if scope == "PRODUCT" && productID == nil {
		details["product_id"] = "a product scope requires a product"
	}

	valueCents, valueBP := in.ValueCents, in.ValueBP
	switch kind {
	case "FIXED":
		if valueCents <= 0 {
			details["value_cents"] = "a fixed discount needs a positive value_cents"
		}
		valueBP = 0
	case "PERCENT":
		if valueBP <= 0 || valueBP > MaxPercentBP {
			details["value_bp"] = "value_bp must be between 1 and 10000"
		}
		valueCents = 0
	}

	startsAt, err := parseTimestamp(in.StartsAt)
	if err != nil {
		details["starts_at"] = "use an RFC3339 timestamp"
	}
	endsAt, err := parseTimestamp(in.EndsAt)
	if err != nil {
		details["ends_at"] = "use an RFC3339 timestamp"
	}
	if startsAt != nil && endsAt != nil && !startsAt.Before(*endsAt) {
		details["ends_at"] = "must be after starts_at"
	}
	if in.MaxUses != nil && *in.MaxUses <= 0 {
		details["max_uses"] = "must be a positive number"
	}
	if in.MinPurchaseCents < 0 {
		details["min_purchase_cents"] = "must not be negative"
	}
	if in.MinQuantity < 0 {
		details["min_quantity"] = "must not be negative"
	}
	if len(details) > 0 {
		return nil, details
	}
	return &Discount{
		Name: name, Kind: kind, ValueCents: valueCents, ValueBP: valueBP,
		Scope: scope, ProductID: productID, MinPurchaseCents: in.MinPurchaseCents,
		MinQuantity: in.MinQuantity, StartsAt: startsAt, EndsAt: endsAt,
		MaxUses: in.MaxUses,
	}, nil
}

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	var in Input
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, d)
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
	discountID, ok := parseID(c)
	if !ok {
		return
	}
	d, err := h.svc.Get(c.Request.Context(), discountID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, d)
}

func (h *Handler) Update(c *gin.Context) {
	discountID, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	d, err := h.svc.Update(c.Request.Context(), discountID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, d)
}

func (h *Handler) Deactivate(c *gin.Context) {
	discountID, ok := parseID(c)
	if !ok {
		return
	}
	d, err := h.svc.Deactivate(c.Request.Context(), discountID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, d)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	discountID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid discount id"))
		return uuid.Nil, false
	}
	return discountID, true
}

func parseTimestamp(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scan(row pgx.Row) (*Discount, error) {
	var (
		d         Discount
		productID pgtype.UUID
		startsAt  pgtype.Timestamptz
		endsAt    pgtype.Timestamptz
		maxUses   pgtype.Int4
	)
	err := row.Scan(&d.ID, &d.Name, &d.Kind, &d.ValueCents, &d.ValueBP, &d.Scope,
		&productID, &d.MinPurchaseCents, &d.MinQuantity, &startsAt, &endsAt,
		&maxUses, &d.UsedCount, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if productID.Valid {
		v := uuid.UUID(productID.Bytes)
		d.ProductID = &v
	}
	if startsAt.Valid {
		t := startsAt.Time
		d.StartsAt = &t
	}
	if endsAt.Valid {
		t := endsAt.Time
		d.EndsAt = &t
	}
	if maxUses.Valid {
		v := int(maxUses.Int32)
		d.MaxUses = &v
	}
	return &d, nil
}

func httpError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("discount not found")
	}
	return err
}

func intPtrArg(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}
