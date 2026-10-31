// Package subscriptions owns subscriptions, subscription lines and the state
// machine: DRAFT -> QUOTATION -> CONFIRMED -> ACTIVE -> (PAUSED) -> CLOSED.
//
// A quotation is a subscription in QUOTATION status with quote_expires_at set.
// Financial history is never deleted: a subscription is closed, not removed.
// The billing worker (billing module) reads next_billing_date on ACTIVE
// subscriptions and produces invoices; the unique index on
// invoices(subscription_id, period_start) keeps that worker exactly-once.
package subscriptions

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/number"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

const (
	StatusDraft     = "DRAFT"
	StatusQuotation = "QUOTATION"
	StatusConfirmed = "CONFIRMED"
	StatusActive    = "ACTIVE"
	StatusPaused    = "PAUSED"
	StatusClosed    = "CLOSED"
)

var ErrNotFound = errors.New("subscription not found")

// PlanFlags carries the plan properties the state machine needs: line seeding,
// billing cadence and which transitions the plan allows.
type PlanFlags struct {
	ID              uuid.UUID
	ProductID       uuid.UUID
	TaxID           *uuid.UUID
	Name            string
	Interval        string
	PriceCents      int64
	MinimumQuantity int
	AutoClose       bool
	AutoRenew       bool
	AllowPause      bool
	AllowEarlyClose bool
	IsActive        bool
}

// PlanLoader is satisfied by plans.Service.
type PlanLoader interface {
	Get(ctx context.Context, planID uuid.UUID) (*PlanSpec, error)
}

// PlanSpec is the neutral plan view this package consumes, avoiding an import
// cycle with the plans package.
type PlanSpec struct {
	ID              uuid.UUID  `json:"id"`
	ProductID       uuid.UUID  `json:"product_id"`
	TaxID           *uuid.UUID `json:"tax_id"`
	Name            string     `json:"name"`
	Interval        string     `json:"interval"`
	PriceCents      int64      `json:"price_cents"`
	MinimumQuantity int        `json:"minimum_quantity"`
	AutoClose       bool       `json:"auto_close"`
	AutoRenew       bool       `json:"auto_renew"`
	AllowPause      bool       `json:"allow_pause"`
	AllowEarlyClose bool       `json:"allow_early_close"`
	IsActive        bool       `json:"is_active"`
}

type Subscription struct {
	ID               uuid.UUID  `json:"id"`
	Number           string     `json:"number"`
	CustomerID       uuid.UUID  `json:"customer_id"`
	PlanID           uuid.UUID  `json:"plan_id"`
	Status           string     `json:"status"`
	StartDate        time.Time  `json:"start_date"`
	ExpiryDate       *time.Time `json:"expiry_date"`
	PaymentTermsDays int        `json:"payment_terms_days"`
	Quantity         int        `json:"quantity"`
	DiscountID       *uuid.UUID `json:"discount_id"`
	NextBillingDate  *time.Time `json:"next_billing_date"`
	QuoteExpiresAt   *time.Time `json:"quote_expires_at"`
	Notes            *string    `json:"notes"`
	CreatedBy        *uuid.UUID `json:"created_by"`
	ActivatedAt      *time.Time `json:"activated_at"`
	PausedAt         *time.Time `json:"paused_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	Lines            []Line     `json:"lines,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Line struct {
	ID             uuid.UUID  `json:"id"`
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	ProductID      uuid.UUID  `json:"product_id"`
	VariantID      *uuid.UUID `json:"variant_id"`
	Description    *string    `json:"description"`
	Quantity       int        `json:"quantity"`
	UnitPriceCents int64      `json:"unit_price_cents"`
	TaxID          *uuid.UUID `json:"tax_id"`
	Position       int        `json:"position"`
}

type Filter struct {
	CustomerID *uuid.UUID
	Status     string
	Search     string
	Page       pagination.Page
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, number, customer_id, plan_id, status, start_date, expiry_date,
	payment_terms_days, quantity, discount_id, next_billing_date, quote_expires_at,
	notes, created_by, activated_at, paused_at, closed_at, created_at, updated_at`

const lineColumns = `id, subscription_id, product_id, variant_id, description,
	quantity, unit_price_cents, tax_id, position`

func (r *Repository) Insert(ctx context.Context, q db.Querier, s *Subscription) error {
	_, err := q.Exec(ctx, `INSERT INTO subscriptions
		(id, number, customer_id, plan_id, status, start_date, expiry_date,
		 payment_terms_days, quantity, discount_id, next_billing_date, quote_expires_at,
		 notes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		s.ID, s.Number, s.CustomerID, s.PlanID, s.Status, id.DateArg(&s.StartDate),
		id.DateArg(s.ExpiryDate), s.PaymentTermsDays, s.Quantity, id.UUIDArg(s.DiscountID),
		id.DateArg(s.NextBillingDate), s.QuoteExpiresAt, id.StringArg(s.Notes),
		id.UUIDArg(s.CreatedBy))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, subscriptionID uuid.UUID) (*Subscription, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM subscriptions WHERE id = $1`, subscriptionID))
}

func (r *Repository) List(ctx context.Context, f Filter) ([]Subscription, int, error) {
	where := `WHERE ($1::uuid IS NULL OR customer_id = $1)
		AND ($2 = '' OR status = $2)
		AND ($3 = '' OR number LIKE '%' || $3 || '%')`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM subscriptions `+where,
		id.UUIDArg(f.CustomerID), f.Status, strings.TrimSpace(f.Search)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM subscriptions `+where+`
		ORDER BY created_at DESC, id LIMIT $4 OFFSET $5`,
		id.UUIDArg(f.CustomerID), f.Status, strings.TrimSpace(f.Search),
		f.Page.PerPage, f.Page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Subscription, 0)
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

// ListForBilling returns ACTIVE subscriptions with next_billing_date due on or
// before `due`. Used by the billing worker.
func (r *Repository) ListForBilling(ctx context.Context, due time.Time, page pagination.Page) ([]Subscription, int, error) {
	const where = `WHERE status = 'ACTIVE' AND next_billing_date <= $1`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM subscriptions `+where, id.DateArg(&due)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM subscriptions `+where+`
		ORDER BY next_billing_date, id LIMIT $2 OFFSET $3`,
		id.DateArg(&due), page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Subscription, 0)
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

func (r *Repository) UpdateFields(ctx context.Context, q db.Querier, s *Subscription) error {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET start_date=$2, expiry_date=$3,
		payment_terms_days=$4, quantity=$5, discount_id=$6, next_billing_date=$7,
		quote_expires_at=$8, notes=$9
		WHERE id=$1`,
		s.ID, id.DateArg(&s.StartDate), id.DateArg(s.ExpiryDate), s.PaymentTermsDays,
		s.Quantity, id.UUIDArg(s.DiscountID), id.DateArg(s.NextBillingDate),
		s.QuoteExpiresAt, id.StringArg(s.Notes))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) SetStatus(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, status string) error {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status = $2 WHERE id = $1`, subscriptionID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkActivated(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, at time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', activated_at=$2 WHERE id=$1`, subscriptionID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkPaused(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET status='PAUSED', paused_at=$2 WHERE id=$1`, subscriptionID, at)
	return err
}

func (r *Repository) MarkResumed(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, nextBilling time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET status='ACTIVE', paused_at=NULL,
		next_billing_date=$2 WHERE id=$1`, subscriptionID, id.DateArg(&nextBilling))
	return err
}

func (r *Repository) MarkClosed(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, at time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET status='CLOSED', closed_at=$2,
		next_billing_date=NULL WHERE id=$1`, subscriptionID, at)
	return err
}

func (r *Repository) SetNextBillingDate(ctx context.Context, q db.Querier, subscriptionID uuid.UUID, d time.Time) error {
	_, err := q.Exec(ctx, `UPDATE subscriptions SET next_billing_date=$2 WHERE id=$1`, subscriptionID, id.DateArg(&d))
	return err
}

func (r *Repository) InsertLine(ctx context.Context, q db.Querier, l *Line) error {
	_, err := q.Exec(ctx, `INSERT INTO subscription_lines
		(id, subscription_id, product_id, variant_id, description, quantity, unit_price_cents, tax_id, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		l.ID, l.SubscriptionID, l.ProductID, id.UUIDArg(l.VariantID), id.StringArg(l.Description),
		l.Quantity, l.UnitPriceCents, id.UUIDArg(l.TaxID), l.Position)
	return err
}

func (r *Repository) Lines(ctx context.Context, q db.Querier, subscriptionID uuid.UUID) ([]Line, error) {
	rows, err := q.Query(ctx, `SELECT `+lineColumns+` FROM subscription_lines
		WHERE subscription_id = $1 ORDER BY position, created_at`, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Line, 0)
	for rows.Next() {
		l, err := scanLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

func (r *Repository) GetLine(ctx context.Context, q db.Querier, subscriptionID, lineID uuid.UUID) (*Line, error) {
	return scanLine(q.QueryRow(ctx, `SELECT `+lineColumns+` FROM subscription_lines
		WHERE id = $1 AND subscription_id = $2`, lineID, subscriptionID))
}

func (r *Repository) UpdateLine(ctx context.Context, q db.Querier, l *Line) error {
	tag, err := q.Exec(ctx, `UPDATE subscription_lines SET product_id=$2, variant_id=$3,
		description=$4, quantity=$5, unit_price_cents=$6, tax_id=$7, position=$8
		WHERE id=$1 AND subscription_id=$9`,
		l.ID, l.ProductID, id.UUIDArg(l.VariantID), id.StringArg(l.Description),
		l.Quantity, l.UnitPriceCents, id.UUIDArg(l.TaxID), l.Position, l.SubscriptionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteLine(ctx context.Context, q db.Querier, subscriptionID, lineID uuid.UUID) error {
	tag, err := q.Exec(ctx, `DELETE FROM subscription_lines WHERE id = $1 AND subscription_id = $2`,
		lineID, subscriptionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CustomerExists(ctx context.Context, q db.Querier, customerID uuid.UUID) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM customers WHERE id = $1)`, customerID).Scan(&ok)
	return ok, err
}

// PlanFlagsByID loads the plan columns the state machine and the billing
// worker need, straight from the database.
func (r *Repository) PlanFlagsByID(ctx context.Context, q db.Querier, planID uuid.UUID) (*PlanSpec, error) {
	row := q.QueryRow(ctx, `SELECT id, product_id, tax_id, name, interval, price_cents,
		minimum_quantity, auto_close, auto_renew, allow_pause, allow_early_close, is_active
		FROM plans WHERE id = $1`, planID)
	var (
		f     PlanSpec
		taxID pgtype.UUID
	)
	if err := row.Scan(&f.ID, &f.ProductID, &taxID, &f.Name, &f.Interval, &f.PriceCents,
		&f.MinimumQuantity, &f.AutoClose, &f.AutoRenew, &f.AllowPause, &f.AllowEarlyClose, &f.IsActive); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if taxID.Valid {
		v := uuid.UUID(taxID.Bytes)
		f.TaxID = &v
	}
	return &f, nil
}

type Service struct {
	repo  *Repository
	audit *audit.Recorder
	plans PlanLoader
}

func NewService(pool *pgxpool.Pool, recorder *audit.Recorder, plans PlanLoader) *Service {
	return &Service{repo: NewRepository(pool), audit: recorder, plans: plans}
}

type LineInput struct {
	ProductID      string  `json:"product_id"`
	VariantID      *string `json:"variant_id"`
	Description    string  `json:"description"`
	Quantity       int     `json:"quantity"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	TaxID          *string `json:"tax_id"`
}

type CreateInput struct {
	CustomerID       string      `json:"customer_id"`
	PlanID           string      `json:"plan_id"`
	Status           string      `json:"status"`
	StartDate        string      `json:"start_date"`
	ExpiryDate       *string     `json:"expiry_date"`
	PaymentTermsDays int         `json:"payment_terms_days"`
	Quantity         int         `json:"quantity"`
	DiscountID       *string     `json:"discount_id"`
	QuoteExpiresAt   *string     `json:"quote_expires_at"`
	Notes            string      `json:"notes"`
	Lines            []LineInput `json:"lines"`
}

type UpdateInput struct {
	StartDate        string  `json:"start_date"`
	ExpiryDate       *string `json:"expiry_date"`
	PaymentTermsDays *int    `json:"payment_terms_days"`
	Quantity         *int    `json:"quantity"`
	DiscountID       *string `json:"discount_id"`
	NextBillingDate  *string `json:"next_billing_date"`
	QuoteExpiresAt   *string `json:"quote_expires_at"`
	Notes            string  `json:"notes"`
}

// Handlers ---------------------------------------------------------------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	var in CreateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	sub, err := h.svc.Create(c.Request.Context(), actor, meta, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, sub)
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	f := Filter{
		Status: strings.ToUpper(strings.TrimSpace(c.Query("status"))),
		Search: strings.TrimSpace(c.Query("search")),
		Page:   page,
	}
	if raw := c.Query("customer_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			httpx.Fail(c, httpx.BadRequest("invalid customer_id"))
			return
		}
		f.CustomerID = &parsed
	}
	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	subID, ok := parseID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Get(c.Request.Context(), subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Update(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	sub, err := h.svc.Update(c.Request.Context(), actor, meta, subID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Confirm(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Confirm(c.Request.Context(), actor, meta, subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Activate(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Activate(c.Request.Context(), actor, meta, subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Pause(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Pause(c.Request.Context(), actor, meta, subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Resume(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Resume(c.Request.Context(), actor, meta, subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Close(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	sub, err := h.svc.Close(c.Request.Context(), actor, meta, subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) AddLine(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	var in LineInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	sub, err := h.svc.AddLine(c.Request.Context(), actor, meta, subID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) UpdateLine(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	lineID, ok := parseLineID(c)
	if !ok {
		return
	}
	var in LineInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	sub, err := h.svc.UpdateLine(c.Request.Context(), actor, meta, subID, lineID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) DeleteLine(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	subID, ok := parseID(c)
	if !ok {
		return
	}
	lineID, ok := parseLineID(c)
	if !ok {
		return
	}
	sub, err := h.svc.DeleteLine(c.Request.Context(), actor, meta, subID, lineID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

// End handlers -----------------------------------------------------------

// Repo exposes the repository for the billing worker and the portal.
func (s *Service) Repo() *Repository { return s.repo }

func (s *Service) Create(ctx context.Context, actor *auth.Actor, meta audit.Meta, in CreateInput) (*Subscription, error) {
	details := map[string]string{}
	customerID, err := uuid.Parse(strings.TrimSpace(in.CustomerID))
	if err != nil {
		details["customer_id"] = "a valid customer id is required"
	}
	planID, err := uuid.Parse(strings.TrimSpace(in.PlanID))
	if err != nil {
		details["plan_id"] = "a valid plan id is required"
	}

	status := strings.ToUpper(strings.TrimSpace(in.Status))
	if status == "" {
		status = StatusDraft
	}
	if status != StatusDraft && status != StatusQuotation {
		details["status"] = "new subscriptions start as DRAFT or QUOTATION"
	}

	if len(in.Notes) > 2000 {
		details["notes"] = "must be 2000 characters or less"
	}
	startDate := today()
	if strings.TrimSpace(in.StartDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(in.StartDate))
		if err != nil {
			details["start_date"] = "use YYYY-MM-DD"
		} else {
			startDate = parsed
		}
	}

	quantity := in.Quantity
	if quantity == 0 {
		quantity = 1
	}
	terms := in.PaymentTermsDays
	if terms == 0 {
		terms = 30
	}

	var discountID *uuid.UUID
	if in.DiscountID != nil && strings.TrimSpace(*in.DiscountID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.DiscountID))
		if err != nil {
			details["discount_id"] = "invalid discount id"
		} else {
			discountID = &parsed
		}
	}

	var expiry, quoteExpires *time.Time
	if v, err := parseDatePtr(in.ExpiryDate); err != nil {
		details["expiry_date"] = "use YYYY-MM-DD"
	} else {
		expiry = v
	}
	if v, err := parseTimestampPtr(in.QuoteExpiresAt); err != nil {
		details["quote_expires_at"] = "use an RFC3339 timestamp or YYYY-MM-DD"
	} else {
		quoteExpires = v
	}
	if status == StatusQuotation && quoteExpires == nil {
		details["quote_expires_at"] = "a QUOTATION subscription needs a quote expiry (quote_expires_at)"
	}
	if expiry != nil && startDate.After(*expiry) {
		details["expiry_date"] = "must be on or after start_date"
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	if ok, err := s.repo.CustomerExists(ctx, s.repo.pool, customerID); err != nil {
		return nil, err
	} else if !ok {
		return nil, httpx.Validation(map[string]string{"customer_id": "customer not found"})
	}

	flags, err := s.planFlags(ctx, planID)
	if err != nil {
		return nil, err
	}
	if !flags.IsActive {
		return nil, httpx.Validation(map[string]string{"plan_id": "plan is not active"})
	}
	if quantity < flags.MinimumQuantity {
		return nil, httpx.Validation(map[string]string{
			"quantity": "below the plan minimum quantity of " + strconv.Itoa(flags.MinimumQuantity),
		})
	}

	// Explicit lines when given; otherwise one line seeded from the plan so an
	// empty subscription still invoices correctly.
	lines := make([]Line, 0, len(in.Lines))
	if len(in.Lines) == 0 {
		desc := "Subscription: " + flags.Name
		lines = append(lines, Line{
			SubscriptionID: uuid.Nil, ProductID: flags.ProductID, TaxID: flags.TaxID,
			Description: &desc, Quantity: quantity, UnitPriceCents: flags.PriceCents,
		})
	} else {
		for _, raw := range in.Lines {
			line, lineDetails, ok := buildLine(raw)
			if !ok {
				return nil, httpx.Validation(lineDetails)
			}
			lines = append(lines, line)
		}
	}

	sub := &Subscription{
		CustomerID: customerID, PlanID: planID, Status: status,
		StartDate: dateOnly(startDate), ExpiryDate: datePtr(expiry),
		PaymentTermsDays: terms, Quantity: quantity, DiscountID: discountID,
		QuoteExpiresAt: quoteExpires, Notes: ptrNotes(in.Notes),
		CreatedBy: uid(actor),
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		number, err := number.Next(ctx, tx, "subscription_number_seq", "SUB")
		if err != nil {
			return err
		}
		sub.ID = id.New()
		sub.Number = number
		if err := s.repo.Insert(ctx, tx, sub); err != nil {
			return err
		}
		for i := range lines {
			lines[i].ID = id.New()
			lines[i].SubscriptionID = sub.ID
			lines[i].Position = i
			if err := s.repo.InsertLine(ctx, tx, &lines[i]); err != nil {
				return err
			}
		}
		stored, err := s.repo.Get(ctx, tx, sub.ID)
		if err != nil {
			return err
		}
		subLines, err := s.repo.Lines(ctx, tx, sub.ID)
		if err != nil {
			return err
		}
		stored.Lines = subLines
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.create", EntityType: "subscription",
			EntityID: sub.ID, After: stored, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, sub.ID)
}

func (s *Service) Get(ctx context.Context, subscriptionID uuid.UUID) (*Subscription, error) {
	sub, err := s.repo.Get(ctx, s.repo.pool, subscriptionID)
	if err != nil {
		return nil, httpError(err)
	}
	lines, err := s.repo.Lines(ctx, s.repo.pool, subscriptionID)
	if err != nil {
		return nil, err
	}
	sub.Lines = lines
	return sub, nil
}

func (s *Service) List(ctx context.Context, f Filter) ([]Subscription, int, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) Update(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID, in UpdateInput) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if editableErr := editableStatus(current.Status); editableErr != nil {
		return nil, editableErr
	}

	before := *current

	details := map[string]string{}
	if in.StartDate != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(in.StartDate))
		if err != nil {
			details["start_date"] = "use YYYY-MM-DD"
		} else {
			current.StartDate = dateOnly(parsed)
		}
	}
	if in.ExpiryDate != nil {
		if strings.TrimSpace(*in.ExpiryDate) == "" {
			current.ExpiryDate = nil
		} else if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*in.ExpiryDate)); err != nil {
			details["expiry_date"] = "use YYYY-MM-DD"
		} else {
			current.ExpiryDate = datePtr(&parsed)
		}
	}
	if in.PaymentTermsDays != nil {
		if *in.PaymentTermsDays < 0 {
			details["payment_terms_days"] = "must not be negative"
		}
		current.PaymentTermsDays = *in.PaymentTermsDays
	}
	if in.Quantity != nil {
		if *in.Quantity < 1 {
			details["quantity"] = "must be at least 1"
		}
		if *in.Quantity > 1_000_000 {
			details["quantity"] = "must not exceed 1,000,000"
		}
		current.Quantity = *in.Quantity
	}
	if in.DiscountID != nil {
		if strings.TrimSpace(*in.DiscountID) == "" {
			current.DiscountID = nil
		} else if parsed, err := uuid.Parse(strings.TrimSpace(*in.DiscountID)); err != nil {
			details["discount_id"] = "invalid discount id"
		} else {
			current.DiscountID = &parsed
		}
	}
	if in.NextBillingDate != nil {
		if strings.TrimSpace(*in.NextBillingDate) == "" {
			current.NextBillingDate = nil
		} else if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*in.NextBillingDate)); err != nil {
			details["next_billing_date"] = "use YYYY-MM-DD"
		} else {
			current.NextBillingDate = &parsed
		}
	}
	if in.QuoteExpiresAt != nil {
		if strings.TrimSpace(*in.QuoteExpiresAt) == "" {
			current.QuoteExpiresAt = nil
		} else if parsed, err := parseTimestampPtr(in.QuoteExpiresAt); err != nil {
			details["quote_expires_at"] = "use an RFC3339 timestamp or YYYY-MM-DD"
		} else {
			current.QuoteExpiresAt = parsed
		}
	}
	if in.Notes != "" {
		current.Notes = &in.Notes
	} else {
		current.Notes = nil
	}
	if current.Notes != nil && len(*current.Notes) > 2000 {
		details["notes"] = "must be 2000 characters or less"
	}
	if current.ExpiryDate != nil && current.StartDate.After(*current.ExpiryDate) {
		details["expiry_date"] = "must be on or after start_date"
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.UpdateFields(ctx, tx, current); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.update", EntityType: "subscription",
			EntityID: subscriptionID, Before: before, After: *current, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, subscriptionID)
}

func (s *Service) Confirm(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusDraft && current.Status != StatusQuotation {
		return nil, httpx.Conflict("only DRAFT or QUOTATION subscriptions can be confirmed")
	}
	if current.QuoteExpiresAt != nil && time.Now().UTC().After(*current.QuoteExpiresAt) {
		return nil, httpx.Conflict("this quotation has expired")
	}

	var stored *Subscription
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.SetStatus(ctx, tx, subscriptionID, StatusConfirmed); err != nil {
			return err
		}
		if err := s.repo.SetNextBillingDate(ctx, tx, subscriptionID, current.StartDate); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		updated.Lines = current.Lines
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.confirm", EntityType: "subscription",
			EntityID: subscriptionID, Before: current, After: updated, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Activate(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusConfirmed {
		return nil, httpx.Conflict("only CONFIRMED subscriptions can be activated")
	}

	now := time.Now().UTC()
	var nextBilling *time.Time
	if current.NextBillingDate != nil {
		v := dateOnly(*current.NextBillingDate)
		nextBilling = &v
	} else {
		v := dateOnly(current.StartDate)
		nextBilling = &v
	}

	var stored *Subscription
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.MarkActivated(ctx, tx, subscriptionID, now); err != nil {
			return err
		}
		if err := s.repo.SetNextBillingDate(ctx, tx, subscriptionID, *nextBilling); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		updated.Lines = current.Lines
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.activate", EntityType: "subscription",
			EntityID: subscriptionID, Before: current, After: updated, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Pause(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusActive {
		return nil, httpx.Conflict("only ACTIVE subscriptions can be paused")
	}
	flags, err := s.planFlags(ctx, current.PlanID)
	if err != nil {
		return nil, err
	}
	if !flags.AllowPause {
		return nil, httpx.Conflict("the plan on this subscription does not allow pausing")
	}
	now := time.Now().UTC()

	var stored *Subscription
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.MarkPaused(ctx, tx, subscriptionID, now); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		updated.Lines = current.Lines
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.pause", EntityType: "subscription",
			EntityID: subscriptionID, Before: current, After: updated, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Resume(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusPaused {
		return nil, httpx.Conflict("only PAUSED subscriptions can be resumed")
	}

	var stored *Subscription
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.MarkResumed(ctx, tx, subscriptionID, today()); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		updated.Lines = current.Lines
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.resume", EntityType: "subscription",
			EntityID: subscriptionID, Before: current, After: updated, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Close(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusActive && current.Status != StatusPaused && current.Status != StatusConfirmed {
		return nil, httpx.Conflict("only ACTIVE, PAUSED or CONFIRMED subscriptions can be closed")
	}
	flags, err := s.planFlags(ctx, current.PlanID)
	if err != nil {
		return nil, err
	}
	if current.ExpiryDate != nil && today().Before(*(current.ExpiryDate)) && !flags.AllowEarlyClose {
		return nil, httpx.Conflict("the plan on this subscription does not allow early closes")
	}
	now := time.Now().UTC()

	var stored *Subscription
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.MarkClosed(ctx, tx, subscriptionID, now); err != nil {
			return err
		}
		updated, err := s.repo.Get(ctx, tx, subscriptionID)
		if err != nil {
			return err
		}
		updated.Lines = current.Lines
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.close", EntityType: "subscription",
			EntityID: subscriptionID, Before: current, After: updated, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) AddLine(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID, in LineInput) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusDraft && current.Status != StatusQuotation {
		return nil, httpx.Conflict("subscription lines can only change while the subscription is DRAFT or QUOTATION")
	}
	line, details, ok := buildLine(in)
	if !ok {
		return nil, httpx.Validation(details)
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		line.ID = id.New()
		line.SubscriptionID = subscriptionID
		line.Position = len(current.Lines)
		if err := s.repo.InsertLine(ctx, tx, &line); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.line.add", EntityType: "subscription",
			EntityID: subscriptionID, After: line, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, subscriptionID)
}

func (s *Service) UpdateLine(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID, lineID uuid.UUID, in LineInput) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusDraft && current.Status != StatusQuotation {
		return nil, httpx.Conflict("subscription lines can only change while the subscription is DRAFT or QUOTATION")
	}
	line, details, ok := buildLine(in)
	if !ok {
		return nil, httpx.Validation(details)
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetLine(ctx, tx, subscriptionID, lineID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return httpx.NotFound("subscription line not found")
			}
			return err
		}
		line.ID = existing.ID
		line.SubscriptionID = subscriptionID
		line.Position = existing.Position
		if err := s.repo.UpdateLine(ctx, tx, &line); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.line.update", EntityType: "subscription",
			EntityID: subscriptionID, Before: existing, After: line, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, subscriptionID)
}

func (s *Service) DeleteLine(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID, lineID uuid.UUID) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusDraft && current.Status != StatusQuotation {
		return nil, httpx.Conflict("subscription lines can only change while the subscription is DRAFT or QUOTATION")
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		existing, err := s.repo.GetLine(ctx, tx, subscriptionID, lineID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return httpx.NotFound("subscription line not found")
			}
			return err
		}
		if err := s.repo.DeleteLine(ctx, tx, subscriptionID, lineID); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "subscription.line.remove", EntityType: "subscription",
			EntityID: subscriptionID, Before: existing, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, subscriptionID)
}

// planFlags resolves the current flags of a plan through the loader.
func (s *Service) planFlags(ctx context.Context, planID uuid.UUID) (*PlanSpec, error) {
	p, err := s.plans.Get(ctx, planID)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func buildLine(in LineInput) (Line, map[string]string, bool) {
	details := map[string]string{}
	productID, err := uuid.Parse(strings.TrimSpace(in.ProductID))
	if err != nil {
		details["product_id"] = "a valid product id is required"
	}
	if in.Quantity > 1_000_000 {
		details["quantity"] = "must not exceed 1,000,000"
	}
	quantity := in.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if in.UnitPriceCents < 0 {
		details["unit_price_cents"] = "must not be negative"
	}
	if in.UnitPriceCents > 1_000_000_000_000 {
		details["unit_price_cents"] = "must not exceed 1,000,000,000,000 (10 billion)"
	}
	var variantID, taxID *uuid.UUID
	if in.VariantID != nil && strings.TrimSpace(*in.VariantID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.VariantID))
		if err != nil {
			details["variant_id"] = "invalid variant id"
		} else {
			variantID = &parsed
		}
	}
	if in.TaxID != nil && strings.TrimSpace(*in.TaxID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.TaxID))
		if err != nil {
			details["tax_id"] = "invalid tax id"
		} else {
			taxID = &parsed
		}
	}
	if len(details) > 0 {
		return Line{}, details, false
	}
	line := Line{
		ProductID: productID, VariantID: variantID, Quantity: quantity,
		UnitPriceCents: in.UnitPriceCents, TaxID: taxID,
	}
	if in.Description != "" {
		line.Description = &in.Description
	}
	return line, nil, true
}

// editableStatus allows edits only in DRAFT and QUOTATION.
func editableStatus(status string) error {
	if status != StatusDraft && status != StatusQuotation {
		return httpx.Conflict("only DRAFT or QUOTATION subscriptions can be edited")
	}
	return nil
}

func parseDatePtr(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseTimestampPtr(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw)); err == nil {
		return &t, nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(*raw))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// helper section so database dates compare cleanly.
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// datePtr truncates an optional time; nil stays nil.
func datePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := dateOnly(*t)
	return &v
}

func today() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	subID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid subscription id"))
		return uuid.Nil, false
	}
	return subID, true
}

func parseLineID(c *gin.Context) (uuid.UUID, bool) {
	lineID, err := uuid.Parse(c.Param("line_id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid line id"))
		return uuid.Nil, false
	}
	return lineID, true
}

func scan(row pgx.Row) (*Subscription, error) {
	var (
		s           Subscription
		startDate   pgtype.Date
		expiryDate  pgtype.Date
		discountID  pgtype.UUID
		nextBilling pgtype.Date
		createdBy   pgtype.UUID
		activatedAt pgtype.Timestamptz
		pausedAt    pgtype.Timestamptz
		closedAt    pgtype.Timestamptz
		notes       pgtype.Text
	)
	err := row.Scan(&s.ID, &s.Number, &s.CustomerID, &s.PlanID, &s.Status, &startDate,
		&expiryDate, &s.PaymentTermsDays, &s.Quantity, &discountID, &nextBilling,
		&s.QuoteExpiresAt, &notes, &createdBy, &activatedAt, &pausedAt, &closedAt,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if startDate.Valid {
		s.StartDate = startDate.Time
	}
	if expiryDate.Valid {
		t := expiryDate.Time
		s.ExpiryDate = &t
	}
	if discountID.Valid {
		v := uuid.UUID(discountID.Bytes)
		s.DiscountID = &v
	}
	if nextBilling.Valid {
		t := nextBilling.Time
		s.NextBillingDate = &t
	}
	if createdBy.Valid {
		v := uuid.UUID(createdBy.Bytes)
		s.CreatedBy = &v
	}
	if activatedAt.Valid {
		t := activatedAt.Time
		s.ActivatedAt = &t
	}
	if pausedAt.Valid {
		t := pausedAt.Time
		s.PausedAt = &t
	}
	if closedAt.Valid {
		t := closedAt.Time
		s.ClosedAt = &t
	}
	if notes.Valid {
		v := notes.String
		s.Notes = &v
	}
	return &s, nil
}

func scanLine(row pgx.Row) (*Line, error) {
	var (
		l           Line
		variantID   pgtype.UUID
		description pgtype.Text
		taxID       pgtype.UUID
	)
	err := row.Scan(&l.ID, &l.SubscriptionID, &l.ProductID, &variantID, &description,
		&l.Quantity, &l.UnitPriceCents, &taxID, &l.Position)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if variantID.Valid {
		v := uuid.UUID(variantID.Bytes)
		l.VariantID = &v
	}
	if description.Valid {
		v := description.String
		l.Description = &v
	}
	if taxID.Valid {
		v := uuid.UUID(taxID.Bytes)
		l.TaxID = &v
	}
	return &l, nil
}

func httpError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return httpx.NotFound("subscription not found")
	}
	return err
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
}

func ptrNotes(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func uid(actor *auth.Actor) *uuid.UUID {
	if actor == nil {
		return nil
	}
	return &actor.UserID
}

func role(actor *auth.Actor) string {
	if actor == nil {
		return ""
	}
	return actor.Role
}
