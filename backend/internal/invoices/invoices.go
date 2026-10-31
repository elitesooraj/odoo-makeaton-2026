// Package invoices owns invoice documents and their financial math.
//
// Rules the rest of the codebase relies on:
//   - Invoices snapshot every value they need: line description, unit price and
//     tax rate. Rendering an old invoice never joins live catalog prices.
//   - gross(line)  = unit_price_cents * quantity
//   - line_total   = gross - allocated share of the discount rule
//   - tax per line = exclusive: money.Percent(line_total, rate_bp), added on top
//     inclusive: money.IncludedTax(line_total, rate_bp), carved
//     out of the payable amount, never added on top
//   - subtotal     = sum of gross
//   - discount     = sum of allocated discount
//   - total        = payable = sum(line_total) + sum(exclusive taxes)
//   - balance      = total minus recorded payments, never below zero
//   - overdue      = derived at read time (due_date < today, balance > 0,
//     status CONFIRMED); it is not a stored status
package invoices

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

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/discounts"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/money"
	"github.com/brightloop/subscription-platform/backend/internal/platform/number"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
)

const (
	StatusDraft     = "DRAFT"
	StatusConfirmed = "CONFIRMED"
	StatusPaid      = "PAID"
	StatusCancelled = "CANCELLED"
)

var ErrNotFound = errors.New("invoice not found")

type Invoice struct {
	ID             uuid.UUID  `json:"id"`
	Number         string     `json:"number"`
	CustomerID     uuid.UUID  `json:"customer_id"`
	SubscriptionID *uuid.UUID `json:"subscription_id"`
	Status         string     `json:"status"`
	IssueDate      time.Time  `json:"issue_date"`
	DueDate        time.Time  `json:"due_date"`
	PeriodStart    *time.Time `json:"period_start"`
	PeriodEnd      *time.Time `json:"period_end"`
	SubtotalCents  int64      `json:"subtotal_cents"`
	DiscountCents  int64      `json:"discount_cents"`
	TaxCents       int64      `json:"tax_cents"`
	TotalCents     int64      `json:"total_cents"`
	BalanceCents   int64      `json:"balance_cents"`
	Notes          *string    `json:"notes"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	SentAt         *time.Time `json:"sent_at"`
	PaidAt         *time.Time `json:"paid_at"`
	CancelledAt    *time.Time `json:"cancelled_at"`
	CreatedBy      *uuid.UUID `json:"created_by"`
	Lines          []Line     `json:"lines,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Line struct {
	ID             uuid.UUID  `json:"id"`
	InvoiceID      uuid.UUID  `json:"invoice_id"`
	ProductID      *uuid.UUID `json:"product_id"`
	VariantID      *uuid.UUID `json:"variant_id"`
	Description    string     `json:"description"`
	Quantity       int        `json:"quantity"`
	UnitPriceCents int64      `json:"unit_price_cents"`
	TaxRateBP      int        `json:"tax_rate_bp"`
	TaxCents       int64      `json:"tax_cents"`
	DiscountCents  int64      `json:"discount_cents"`
	LineTotalCents int64      `json:"line_total_cents"`
	Position       int        `json:"position"`
}

type Filter struct {
	CustomerID     *uuid.UUID
	SubscriptionID *uuid.UUID
	Status         string
	Search         string
	Overdue        bool
	Page           pagination.Page
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, number, customer_id, subscription_id, status, issue_date, due_date,
	period_start, period_end, subtotal_cents, discount_cents, tax_cents, total_cents,
	balance_cents, notes, confirmed_at, sent_at, paid_at, cancelled_at, created_by,
	created_at, updated_at`

const lineColumns = `id, invoice_id, product_id, variant_id, description, quantity,
	unit_price_cents, tax_rate_bp, tax_cents, discount_cents, line_total_cents, position`

func (r *Repository) Insert(ctx context.Context, q db.Querier, i *Invoice) error {
	_, err := q.Exec(ctx, `INSERT INTO invoices
		(id, number, customer_id, subscription_id, status, issue_date, due_date,
		 period_start, period_end, subtotal_cents, discount_cents, tax_cents,
		 total_cents, balance_cents, notes, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		i.ID, i.Number, i.CustomerID, id.UUIDArg(i.SubscriptionID), i.Status,
		id.DateArg(&i.IssueDate), id.DateArg(&i.DueDate), id.DateArg(i.PeriodStart),
		id.DateArg(i.PeriodEnd), i.SubtotalCents, i.DiscountCents, i.TaxCents,
		i.TotalCents, i.BalanceCents, id.StringArg(i.Notes), id.UUIDArg(i.CreatedBy))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, invoiceID uuid.UUID) (*Invoice, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM invoices WHERE id = $1`, invoiceID))
}

func (r *Repository) List(ctx context.Context, f Filter) ([]Invoice, int, error) {
	where := `WHERE ($1::uuid IS NULL OR customer_id = $1)
		AND ($2::uuid IS NULL OR subscription_id = $2)
		AND ($3 = '' OR status = $3)
		AND ($4 = '' OR number LIKE '%' || $4 || '%')
		AND (NOT $5 OR (
			status = 'CONFIRMED' AND balance_cents > 0 AND due_date < current_date))`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM invoices `+where,
		id.UUIDArg(f.CustomerID), id.UUIDArg(f.SubscriptionID), f.Status,
		strings.TrimSpace(f.Search), f.Overdue).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM invoices `+where+`
		ORDER BY issue_date DESC, created_at DESC, id LIMIT $6 OFFSET $7`,
		id.UUIDArg(f.CustomerID), id.UUIDArg(f.SubscriptionID), f.Status,
		strings.TrimSpace(f.Search), f.Overdue, f.Page.PerPage, f.Page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Invoice, 0)
	for rows.Next() {
		i, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *i)
	}
	return out, total, rows.Err()
}

func (r *Repository) Lines(ctx context.Context, q db.Querier, invoiceID uuid.UUID) ([]Line, error) {
	rows, err := q.Query(ctx, `SELECT `+lineColumns+` FROM invoice_lines
		WHERE invoice_id = $1 ORDER BY position, created_at`, invoiceID)
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

func (r *Repository) InsertLine(ctx context.Context, q db.Querier, l *Line) error {
	_, err := q.Exec(ctx, `INSERT INTO invoice_lines
		(id, invoice_id, product_id, variant_id, description, quantity,
		 unit_price_cents, tax_rate_bp, tax_cents, discount_cents, line_total_cents, position)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		l.ID, l.InvoiceID, id.UUIDArg(l.ProductID), id.UUIDArg(l.VariantID), l.Description,
		l.Quantity, l.UnitPriceCents, l.TaxRateBP, l.TaxCents, l.DiscountCents,
		l.LineTotalCents, l.Position)
	return err
}

func (r *Repository) UpdateLineAmounts(ctx context.Context, q db.Querier, l *Line) error {
	_, err := q.Exec(ctx, `UPDATE invoice_lines SET tax_cents = $2, discount_cents = $3,
		line_total_cents = $4 WHERE id = $1`, l.ID, l.TaxCents, l.DiscountCents, l.LineTotalCents)
	return err
}

// ReplaceLines deletes and rewrites the lines of a DRAFT invoice.
func (r *Repository) ReplaceLines(ctx context.Context, q db.Querier, invoiceID uuid.UUID) error {
	_, err := q.Exec(ctx, `DELETE FROM invoice_lines WHERE invoice_id = $1`, invoiceID)
	return err
}

func (r *Repository) UpdateDraft(ctx context.Context, q db.Querier, i *Invoice) error {
	tag, err := q.Exec(ctx, `UPDATE invoices SET due_date = $2, notes = $3,
		period_start = $4, period_end = $5
		WHERE id = $1 AND status = 'DRAFT'`,
		i.ID, id.DateArg(&i.DueDate), id.StringArg(i.Notes),
		id.DateArg(i.PeriodStart), id.DateArg(i.PeriodEnd))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("invoice is not a draft")
	}
	return nil
}

func (r *Repository) UpdateTotals(ctx context.Context, q db.Querier, invoiceID uuid.UUID, subtotal, discount, tax, total, balance int64) error {
	_, err := q.Exec(ctx, `UPDATE invoices SET subtotal_cents = $2, discount_cents = $3,
		tax_cents = $4, total_cents = $5, balance_cents = $6
		WHERE id = $1 AND status = 'DRAFT'`,
		invoiceID, subtotal, discount, tax, total, balance)
	return err
}

func (r *Repository) SetStatus(ctx context.Context, q db.Querier, invoiceID uuid.UUID, status string, at time.Time) error {
	var query string
	switch status {
	case StatusConfirmed:
		query = `UPDATE invoices SET status='CONFIRMED', confirmed_at=$2, balance_cents=total_cents
			WHERE id=$1 AND status='DRAFT' AND total_cents > 0`
	case StatusPaid:
		query = `UPDATE invoices SET status='PAID', paid_at=$2 WHERE id=$1 AND status='CONFIRMED'`
	case StatusCancelled:
		query = `UPDATE invoices SET status='CANCELLED', cancelled_at=$2 WHERE id=$1 AND status='DRAFT'`
	default:
		return errors.New("unsupported status transition")
	}
	tag, err := q.Exec(ctx, query, invoiceID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: status}
	}
	return nil
}

// ErrTransition marks an illegal lifecycle move; httpError maps it to 409.
type ErrTransition struct{ to string }

func (e ErrTransition) Error() string { return "transition to " + e.to + " is not allowed" }

func (r *Repository) CancelConfirmed(ctx context.Context, q db.Querier, invoiceID uuid.UUID, at time.Time) error {
	// Only an unpaid confirmed invoice may be cancelled.
	tag, err := q.Exec(ctx, `UPDATE invoices SET status='CANCELLED', cancelled_at=$2
		WHERE id=$1 AND status='CONFIRMED' AND balance_cents = total_cents`, invoiceID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: StatusCancelled}
	}
	return nil
}

func (r *Repository) MarkSent(ctx context.Context, q db.Querier, invoiceID uuid.UUID, at time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE invoices SET sent_at = $2 WHERE id = $1 AND status = 'CONFIRMED'`,
		invoiceID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: "sent"}
	}
	return nil
}

func (r *Repository) ApplyPayment(ctx context.Context, q db.Querier, invoiceID uuid.UUID, amount int64, at time.Time) error {
	tag, err := q.Exec(ctx, `UPDATE invoices SET balance_cents = balance_cents - $2,
		status = CASE WHEN balance_cents - $2 = 0 AND status = 'CONFIRMED' THEN 'PAID'
			WHEN balance_cents - $2 > 0 AND status = 'PAID' THEN 'CONFIRMED' ELSE status END,
		paid_at = CASE WHEN balance_cents - $2 = 0 AND status = 'CONFIRMED' THEN $3
			WHEN balance_cents - $2 > 0 AND status = 'PAID' THEN NULL ELSE paid_at END
		WHERE id = $1 AND status IN ('CONFIRMED','PAID') AND balance_cents >= $2`,
		invoiceID, amount, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTransition{to: "paid"}
	}
	return nil
}

// TaxDetail is the tax snapshot the service needs when building a line.
type TaxDetail struct {
	RateBP      int
	IsInclusive bool
}

// TaxLoader resolves tax rules by ID; implemented via the taxes package.
type TaxLoader interface {
	Get(ctx context.Context, taxID uuid.UUID) (*TaxDetail, error)
}

type Service struct {
	repo      *Repository
	audit     *audit.Recorder
	subs      *subscriptions.Service
	discounts *discounts.Service
	taxes     TaxLoader
}

// Repo exposes the shared repository for payments and the billing worker.
func (s *Service) Repo() *Repository { return s.repo }

func NewService(pool *pgxpool.Pool, recorder *audit.Recorder, subs *subscriptions.Service,
	disc *discounts.Service, taxes TaxLoader) *Service {
	return &Service{
		repo:      NewRepository(pool),
		audit:     recorder,
		subs:      subs,
		discounts: disc,
		taxes:     taxes,
	}
}

type LineInput struct {
	ProductID      *string `json:"product_id"`
	VariantID      *string `json:"variant_id"`
	Description    string  `json:"description"`
	Quantity       int     `json:"quantity"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	TaxID          *string `json:"tax_id"`
}

type CreateInput struct {
	CustomerID     string      `json:"customer_id"`
	SubscriptionID *string     `json:"subscription_id"`
	IssueDate      *string     `json:"issue_date"`
	DueDate        *string     `json:"due_date"`
	PeriodStart    *string     `json:"period_start"`
	PeriodEnd      *string     `json:"period_end"`
	DiscountID     *string     `json:"discount_id"`
	Notes          string      `json:"notes"`
	Lines          []LineInput `json:"lines"`
}

type UpdateInput struct {
	DueDate     *string      `json:"due_date"`
	Notes       string       `json:"notes"`
	PeriodStart *string      `json:"period_start"`
	PeriodEnd   *string      `json:"period_end"`
	Lines       *[]LineInput `json:"lines"`
}

// draftLine carries the inclusive/exclusive flag across the finalize step.
type draftLine struct {
	Line
	taxInclusive bool
}

// Create makes a DRAFT invoice. Provide explicit lines, or a subscription_id to
// snapshot the subscription's lines — never both.
func (s *Service) Create(ctx context.Context, actor *auth.Actor, meta audit.Meta, in CreateInput) (*Invoice, error) {
	details := map[string]string{}
	customerID, err := uuid.Parse(strings.TrimSpace(in.CustomerID))
	if err != nil {
		details["customer_id"] = "a valid customer id is required"
	}
	var subscriptionID *uuid.UUID
	if in.SubscriptionID != nil && strings.TrimSpace(*in.SubscriptionID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*in.SubscriptionID))
		if err != nil {
			details["subscription_id"] = "invalid subscription id"
		} else {
			subscriptionID = &parsed
		}
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

	issueDate := today()
	if in.IssueDate != nil && strings.TrimSpace(*in.IssueDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*in.IssueDate))
		if err != nil {
			details["issue_date"] = "use YYYY-MM-DD"
		} else {
			issueDate = dateOnly(parsed)
		}
	}

	var periodStart, periodEnd *time.Time
	if v, ok := parseDate(in.PeriodStart, details, "period_start"); ok {
		periodStart = v
	}
	if v, ok := parseDate(in.PeriodEnd, details, "period_end"); ok {
		periodEnd = v
	}
	if periodStart != nil && periodEnd != nil && !periodStart.Before(*periodEnd) {
		details["period_end"] = "must be after period_start"
	}

	if subscriptionID != nil && len(in.Lines) > 0 {
		details["lines"] = "cannot combine subscription_id with explicit lines"
	}
	if subscriptionID == nil && len(in.Lines) == 0 {
		details["lines"] = "provide lines or a subscription_id"
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	terms := 30
	if subscriptionID != nil {
		sub, err := s.subs.Get(ctx, *subscriptionID)
		if err != nil {
			return nil, err
		}
		if sub.CustomerID != customerID {
			return nil, httpx.Validation(map[string]string{
				"customer_id": "customer does not own this subscription",
			})
		}
		terms = sub.PaymentTermsDays
		if periodStart == nil {
			start := sub.StartDate
			periodStart = &start
		}
		if periodEnd == nil {
			end := money.AddInterval(*periodStart, "MONTHLY")
			periodEnd = &end
		}
	}

	dueDate := issueDate.AddDate(0, 0, terms)
	if in.DueDate != nil && strings.TrimSpace(*in.DueDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*in.DueDate))
		if err != nil {
			return nil, httpx.Validation(map[string]string{"due_date": "use YYYY-MM-DD"})
		}
		dueDate = parsed
	}

	inv := &Invoice{
		CustomerID:     customerID,
		SubscriptionID: subscriptionID,
		Status:         StatusDraft,
		IssueDate:      *issueDate,
		DueDate:        *dateOnly(dueDate),
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		Notes:          ptrNotes(in.Notes),
		CreatedBy:      uid(actor),
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		invNumber, err := number.Next(ctx, tx, "invoice_number_seq", "INV")
		if err != nil {
			return err
		}
		inv.ID = id.New()
		inv.Number = invNumber
		if err := s.repo.Insert(ctx, tx, inv); err != nil {
			return err
		}

		var drafts []draftLine
		if subscriptionID != nil {
			sub, err := s.subs.Get(ctx, *subscriptionID)
			if err != nil {
				return err
			}
			drafts, err = s.draftLinesFromSubscription(ctx, tx, inv.ID, sub)
			if err != nil {
				return err
			}
		} else {
			drafts, err = s.draftLinesFromInput(ctx, tx, inv.ID, in.Lines)
			if err != nil {
				return err
			}
		}

		if err := s.finalize(ctx, tx, inv, drafts, discountID); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "invoice.create", EntityType: "invoice",
			EntityID: inv.ID, After: inv, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, inv.ID)
}

// CreateForBilling is the billing-worker entry point. It runs inside the
// caller's transaction, snapshots the subscription lines, applies the discount
// rule, sets totals and confirms the invoice.
func (s *Service) CreateForBilling(ctx context.Context, q db.Querier, actorID *uuid.UUID,
	meta audit.Meta, sub *subscriptions.Subscription, periodStart, periodEnd time.Time) (*Invoice, error) {

	inv := &Invoice{
		ID:             id.New(),
		CustomerID:     sub.CustomerID,
		SubscriptionID: &sub.ID,
		Status:         StatusDraft,
		IssueDate:      *today(),
		DueDate:        *dateOnly(time.Now().UTC().AddDate(0, 0, sub.PaymentTermsDays)),
		PeriodStart:    dateOnly(periodStart),
		PeriodEnd:      dateOnly(periodEnd),
		Notes:          ptrNotes("Billing for period " + periodStart.Format("2006-01-02")),
		CreatedBy:      actorID,
	}
	invNumber, err := number.Next(ctx, q, "invoice_number_seq", "INV")
	if err != nil {
		return nil, err
	}
	inv.Number = invNumber
	if err := s.repo.Insert(ctx, q, inv); err != nil {
		return nil, err
	}

	drafts, err := s.draftLinesFromSubscription(ctx, q, inv.ID, sub)
	if err != nil {
		return nil, err
	}
	if err := s.finalize(ctx, q, inv, drafts, sub.DiscountID); err != nil {
		return nil, err
	}
	if err := s.repo.SetStatus(ctx, q, inv.ID, StatusConfirmed, time.Now().UTC()); err != nil {
		if errors.As(err, new(ErrTransition)) || errors.Is(err, ErrNotFound) {
			return nil, httpx.Conflict("billing produced an invalid invoice")
		}
		return nil, err
	}
	inv.Status = StatusConfirmed
	inv.Number = invNumber

	if err := s.audit.Record(ctx, q, audit.Event{
		ActorID: actorID, ActorRole: "SYSTEM",
		Action: "invoice.confirm", EntityType: "invoice",
		EntityID: inv.ID, After: inv, Meta: meta,
	}); err != nil {
		return nil, err
	}
	return inv, nil
}

// draftLinesFromSubscription snapshots the subscription lines plus tax rates.
func (s *Service) draftLinesFromSubscription(ctx context.Context, q db.Querier, invoiceID uuid.UUID, sub *subscriptions.Subscription) ([]draftLine, error) {
	if len(sub.Lines) == 0 {
		return nil, httpx.Conflict("subscription has no lines to invoice")
	}
	drafts := make([]draftLine, 0, len(sub.Lines))
	for i, sl := range sub.Lines {
		detail := TaxDetail{}
		if sl.TaxID != nil {
			t, err := s.taxes.Get(ctx, *sl.TaxID)
			if err != nil {
				return nil, err
			}
			detail = *t
		}
		desc := "Subscription item"
		if sl.Description != nil {
			desc = *sl.Description
		}
		drafts = append(drafts, draftLine{
			Line: Line{
				ID: id.New(), InvoiceID: invoiceID, ProductID: &sl.ProductID,
				VariantID: sl.VariantID, Description: desc, Quantity: sl.Quantity,
				UnitPriceCents: sl.UnitPriceCents, TaxRateBP: detail.RateBP, Position: i,
			},
			taxInclusive: detail.IsInclusive,
		})
	}
	for i := range drafts {
		if err := s.repo.InsertLine(ctx, q, &drafts[i].Line); err != nil {
			return nil, err
		}
	}
	return drafts, nil
}

// draftLinesFromInput resolves explicit lines with tax snapshots.
func (s *Service) draftLinesFromInput(ctx context.Context, q db.Querier, invoiceID uuid.UUID, inputs []LineInput) ([]draftLine, error) {
	drafts := make([]draftLine, 0, len(inputs))
	for i, in := range inputs {
		details := map[string]string{}
		if strings.TrimSpace(in.Description) == "" {
			details["lines.description"] = "every invoice line needs a description"
		}
		if in.Quantity < 1 {
			details["lines.quantity"] = "must be at least 1"
		}
		if in.UnitPriceCents < 0 {
			details["lines.unit_price_cents"] = "must not be negative"
		}
		var productID, variantID *uuid.UUID
		if in.ProductID != nil && strings.TrimSpace(*in.ProductID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(*in.ProductID))
			if err != nil {
				details["lines.product_id"] = "invalid product id"
			} else {
				productID = &parsed
			}
		}
		if in.VariantID != nil && strings.TrimSpace(*in.VariantID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(*in.VariantID))
			if err != nil {
				details["lines.variant_id"] = "invalid variant id"
			} else {
				variantID = &parsed
			}
		}
		t := TaxDetail{}
		if in.TaxID != nil && strings.TrimSpace(*in.TaxID) != "" {
			parsed, err := uuid.Parse(strings.TrimSpace(*in.TaxID))
			if err != nil {
				details["lines.tax_id"] = "invalid tax id"
			} else {
				loaded, err := s.taxes.Get(ctx, parsed)
				if err != nil {
					details["lines.tax_id"] = "tax rate not found"
				} else {
					t = *loaded
				}
			}
		}
		if len(details) > 0 {
			return nil, httpx.Validation(details)
		}
		drafts = append(drafts, draftLine{
			Line: Line{
				ID: id.New(), InvoiceID: invoiceID, ProductID: productID,
				VariantID: variantID, Description: strings.TrimSpace(in.Description),
				Quantity: in.Quantity, UnitPriceCents: in.UnitPriceCents,
				TaxRateBP: t.RateBP, Position: i,
			},
			taxInclusive: t.IsInclusive,
		})
	}
	for i := range drafts {
		if err := s.repo.InsertLine(ctx, q, &drafts[i].Line); err != nil {
			return nil, err
		}
	}
	return drafts, nil
}

// finalize validates the discount rule, allocates it over the lines, computes
// taxes and totals, persists the snapshots inside the invoice and the ledger.
func (s *Service) finalize(ctx context.Context, q db.Querier, inv *Invoice, drafts []draftLine, discountID *uuid.UUID) error {
	subtotal := int64(0)
	totalQuantity := 0
	for i := range drafts {
		gross := int64(drafts[i].Quantity) * drafts[i].UnitPriceCents
		inv2 := gross // keep the value in a local for clarity in allocation
		_ = inv2
		subtotal += gross
		totalQuantity += drafts[i].Quantity
	}

	var rule *discounts.Discount
	if discountID != nil {
		productIDs := make([]uuid.UUID, 0, len(drafts))
		for _, d := range drafts {
			if d.ProductID != nil {
				productIDs = append(productIDs, *d.ProductID)
			}
		}
		d, err := s.discounts.CheckForSubscription(ctx, *discountID, time.Now(), subtotal, totalQuantity, productIDs)
		if err != nil {
			return err
		}
		rule = d
	}

	// 1) allocate discount on gross
	discountTotal := int64(0)
	if rule != nil && rule.Kind == "FIXED" {
		shares := money.Allocate(rule.ValueCents, grossWeights(drafts))
		for i := range drafts {
			drafts[i].DiscountCents = shares[i]
		}
		discountTotal = rule.ValueCents
	} else if rule != nil {
		for i := range drafts {
			drafts[i].DiscountCents = money.Percent(gross(drafts[i]), rule.ValueBP)
		}
		for i := range drafts {
			discountTotal += drafts[i].DiscountCents
		}
	}

	// 2) compute line_total = gross - discount and per-line tax
	taxTotal := int64(0)
	exclusiveTax := int64(0)
	for i := range drafts {
		g := gross(drafts[i])
		lineTotal := g - drafts[i].DiscountCents
		if lineTotal < 0 {
			lineTotal = 0
		}
		var tax int64
		if drafts[i].TaxRateBP > 0 {
			if drafts[i].taxInclusive {
				tax = money.IncludedTax(lineTotal, drafts[i].TaxRateBP)
			} else {
				tax = money.Percent(lineTotal, drafts[i].TaxRateBP)
				exclusiveTax += tax
			}
		}
		drafts[i].LineTotalCents = lineTotal
		drafts[i].TaxCents = tax
	}

	// 3) publish totals
	inv.SubtotalCents = subtotal
	inv.DiscountCents = discountTotal
	inv.TaxCents = taxTotal
	inv.TotalCents = sumLineTotals(drafts) + exclusiveTax
	inv.BalanceCents = inv.TotalCents

	for i := range drafts {
		if err := s.repo.UpdateLineAmounts(ctx, q, &drafts[i].Line); err != nil {
			return err
		}
	}
	if err := s.repo.UpdateTotals(ctx, q, inv.ID, inv.SubtotalCents, inv.DiscountCents,
		inv.TaxCents, inv.TotalCents, inv.BalanceCents); err != nil {
		return err
	}
	inv.Lines = lineValues(drafts)

	if rule != nil {
		if err := s.discounts.IncrementUsed(ctx, q, rule.ID); err != nil {
			return err
		}
		if err := RecordApplication(ctx, q, rule.ID, inv, discountTotal); err != nil {
			return err
		}
	}
	return nil
}

func gross(d draftLine) int64 { return int64(d.Quantity) * d.UnitPriceCents }

func grossWeights(drafts []draftLine) []int64 {
	out := make([]int64, len(drafts))
	for i, d := range drafts {
		out[i] = gross(d)
	}
	return out
}

func sumLineTotals(drafts []draftLine) int64 {
	var total int64
	for i := range drafts {
		total += drafts[i].LineTotalCents
	}
	return total
}

func lineValues(drafts []draftLine) []Line {
	out := make([]Line, len(drafts))
	for i, d := range drafts {
		out[i] = d.Line
	}
	return out
}

// Update patches a DRAFT invoice. Replacing lines recomputes totals; the
// discount rule is not re-applied when lines change by hand.
func (s *Service) Update(ctx context.Context, actor *auth.Actor, meta audit.Meta, invoiceID uuid.UUID, in UpdateInput) (*Invoice, error) {
	current, err := s.Get(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusDraft {
		return nil, httpx.Conflict("only DRAFT invoices can be edited")
	}
	before := *current

	if in.Lines != nil {
		details := map[string]string{}
		if in.DueDate != nil {
			if v, ok := parseDate(in.DueDate, details, "due_date"); ok && v != nil {
				current.DueDate = *v
			}
		}
		if len(details) > 0 {
			return nil, httpx.Validation(details)
		}
		if in.Notes != "" {
			current.Notes = ptrNotes(in.Notes)
		} else {
			current.Notes = nil
		}
		if err := db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
			if err := s.repo.ReplaceLines(ctx, tx, invoiceID); err != nil {
				return err
			}
			newDrafts, err := s.draftLinesFromInput(ctx, tx, invoiceID, *in.Lines)
			if err != nil {
				return err
			}
			if err := s.finalize(ctx, tx, current, newDrafts, nil); err != nil {
				return err
			}
			return s.audit.Record(ctx, tx, audit.Event{
				ActorID: uid(actor), ActorRole: role(actor),
				Action: "invoice.update", EntityType: "invoice",
				EntityID: invoiceID, Before: before, After: current, Meta: meta,
			})
		}); err != nil {
			return nil, err
		}
		return s.Get(ctx, invoiceID)
	}

	details := map[string]string{}
	if in.DueDate != nil {
		if v, ok := parseDate(in.DueDate, details, "due_date"); ok && v != nil {
			current.DueDate = *v
		}
	}
	if in.PeriodStart != nil {
		if v, ok := parseDate(in.PeriodStart, details, "period_start"); ok {
			current.PeriodStart = v
		}
	}
	if in.PeriodEnd != nil {
		if v, ok := parseDate(in.PeriodEnd, details, "period_end"); ok {
			current.PeriodEnd = v
		}
	}
	if current.PeriodStart != nil && current.PeriodEnd != nil && !current.PeriodStart.Before(*current.PeriodEnd) {
		details["period_end"] = "must be after period_start"
	}
	if in.Notes != "" {
		current.Notes = ptrNotes(in.Notes)
	} else {
		current.Notes = nil
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.UpdateDraft(ctx, tx, current); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "invoice.update", EntityType: "invoice",
			EntityID: invoiceID, Before: before, After: current, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, invoiceID)
}

func (s *Service) Get(ctx context.Context, invoiceID uuid.UUID) (*Invoice, error) {
	inv, err := s.repo.Get(ctx, s.repo.pool, invoiceID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NotFound("invoice not found")
		}
		return nil, err
	}
	lines, err := s.repo.Lines(ctx, s.repo.pool, invoiceID)
	if err != nil {
		return nil, err
	}
	inv.Lines = lines
	return inv, nil
}

func (s *Service) List(ctx context.Context, f Filter) ([]Invoice, int, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) OverdueInvoices(ctx context.Context, page pagination.Page) ([]Invoice, int, error) {
	return s.repo.List(ctx, Filter{Overdue: true, Page: page})
}

func (s *Service) Confirm(ctx context.Context, actor *auth.Actor, meta audit.Meta, invoiceID uuid.UUID) (*Invoice, error) {
	current, err := s.Get(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusDraft {
		return nil, httpx.Conflict("only DRAFT invoices can be confirmed")
	}
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.SetStatus(ctx, tx, invoiceID, StatusConfirmed, time.Now().UTC()); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "invoice.confirm", EntityType: "invoice",
			EntityID: invoiceID, After: current, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, invoiceID)
}

func (s *Service) Cancel(ctx context.Context, actor *auth.Actor, meta audit.Meta, invoiceID uuid.UUID) (*Invoice, error) {
	current, err := s.Get(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var cancelErr error
		switch current.Status {
		case StatusDraft:
			cancelErr = s.repo.SetStatus(ctx, tx, invoiceID, StatusCancelled, time.Now().UTC())
		case StatusConfirmed:
			cancelErr = s.repo.CancelConfirmed(ctx, tx, invoiceID, time.Now().UTC())
		default:
			cancelErr = ErrTransition{to: StatusCancelled}
		}
		if cancelErr != nil {
			return cancelErr
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "invoice.cancel", EntityType: "invoice",
			EntityID: invoiceID, Before: current, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, invoiceID)
}

func (s *Service) Send(ctx context.Context, actor *auth.Actor, meta audit.Meta, invoiceID uuid.UUID) (*Invoice, error) {
	current, err := s.Get(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if current.Status != StatusConfirmed {
		return nil, httpx.Conflict("only a CONFIRMED invoice can be sent")
	}
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.MarkSent(ctx, tx, invoiceID, time.Now().UTC()); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "invoice.send", EntityType: "invoice",
			EntityID: invoiceID, Before: current, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, invoiceID)
}

// RecordApplication writes a redemption row for the invoice.
func RecordApplication(ctx context.Context, q db.Querier, discountID uuid.UUID, inv *Invoice, amount int64) error {
	_, err := q.Exec(ctx, `INSERT INTO discount_applications
		(id, discount_id, invoice_id, subscription_id, amount_cents)
		VALUES ($1, $2, $3, $4, $5)`,
		id.New(), discountID, inv.ID, id.UUIDArg(inv.SubscriptionID), amount)
	return err
}

func parseDate(raw *string, details map[string]string, field string) (*time.Time, bool) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, false
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*raw))
	if err != nil {
		details[field] = "use YYYY-MM-DD"
		return nil, false
	}
	v := dateOnly(parsed)
	return v, true
}

func ptrNotes(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func dateOnly(t time.Time) *time.Time {
	v := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &v
}

func today() *time.Time {
	return dateOnly(time.Now().UTC())
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

func httpError(err error) error {
	var transition ErrTransition
	if errors.As(err, &transition) {
		return httpx.Conflict(transition.Error())
	}
	return err
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
	inv, err := h.svc.Create(c.Request.Context(), actor, meta, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, inv)
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	f := Filter{
		Status:  strings.ToUpper(strings.TrimSpace(c.Query("status"))),
		Search:  strings.TrimSpace(c.Query("search")),
		Overdue: c.Query("overdue") == "true",
		Page:    page,
	}
	if raw := c.Query("customer_id"); raw != "" {
		if parsed, err := uuid.Parse(raw); err == nil {
			f.CustomerID = &parsed
		} else {
			httpx.Fail(c, httpx.BadRequest("invalid customer_id"))
			return
		}
	}
	if raw := c.Query("subscription_id"); raw != "" {
		if parsed, err := uuid.Parse(raw); err == nil {
			f.SubscriptionID = &parsed
		} else {
			httpx.Fail(c, httpx.BadRequest("invalid subscription_id"))
			return
		}
	}
	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	invID, ok := parseID(c)
	if !ok {
		return
	}
	inv, err := h.svc.Get(c.Request.Context(), invID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, inv)
}

func (h *Handler) Update(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	invID, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	inv, err := h.svc.Update(c.Request.Context(), actor, meta, invID, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, inv)
}

func (h *Handler) Confirm(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	invID, ok := parseID(c)
	if !ok {
		return
	}
	inv, err := h.svc.Confirm(c.Request.Context(), actor, meta, invID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, inv)
}

func (h *Handler) Cancel(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	invID, ok := parseID(c)
	if !ok {
		return
	}
	inv, err := h.svc.Cancel(c.Request.Context(), actor, meta, invID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, inv)
}

func (h *Handler) Send(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	invID, ok := parseID(c)
	if !ok {
		return
	}
	inv, err := h.svc.Send(c.Request.Context(), actor, meta, invID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, inv)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	invID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid invoice id"))
		return uuid.Nil, false
	}
	return invID, true
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
}

// End handlers -----------------------------------------------------------

func scan(row pgx.Row) (*Invoice, error) {
	var (
		i            Invoice
		issueDate    pgtype.Date
		dueDate      pgtype.Date
		subscription pgtype.UUID
		periodStart  pgtype.Date
		periodEnd    pgtype.Date
		notes        pgtype.Text
		confirmedAt  pgtype.Timestamptz
		sentAt       pgtype.Timestamptz
		paidAt       pgtype.Timestamptz
		cancelledAt  pgtype.Timestamptz
		createdBy    pgtype.UUID
	)
	err := row.Scan(&i.ID, &i.Number, &i.CustomerID, &subscription, &i.Status,
		&issueDate, &dueDate, &periodStart, &periodEnd, &i.SubtotalCents,
		&i.DiscountCents, &i.TaxCents, &i.TotalCents, &i.BalanceCents, &notes,
		&confirmedAt, &sentAt, &paidAt, &cancelledAt, &createdBy,
		&i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if subscription.Valid {
		v := uuid.UUID(subscription.Bytes)
		i.SubscriptionID = &v
	}
	if issueDate.Valid {
		i.IssueDate = issueDate.Time
	}
	if dueDate.Valid {
		i.DueDate = dueDate.Time
	}
	if periodStart.Valid {
		t := periodStart.Time
		i.PeriodStart = &t
	}
	if periodEnd.Valid {
		t := periodEnd.Time
		i.PeriodEnd = &t
	}
	if notes.Valid {
		v := notes.String
		i.Notes = &v
	}
	timestamp := func(ts pgtype.Timestamptz) *time.Time {
		if !ts.Valid {
			return nil
		}
		v := ts.Time
		return &v
	}
	i.ConfirmedAt = timestamp(confirmedAt)
	i.SentAt = timestamp(sentAt)
	i.PaidAt = timestamp(paidAt)
	i.CancelledAt = timestamp(cancelledAt)
	if createdBy.Valid {
		v := uuid.UUID(createdBy.Bytes)
		i.CreatedBy = &v
	}
	return &i, nil
}

func scanLine(row pgx.Row) (*Line, error) {
	var (
		l         Line
		productID pgtype.UUID
		variantID pgtype.UUID
	)
	err := row.Scan(&l.ID, &l.InvoiceID, &productID, &variantID, &l.Description,
		&l.Quantity, &l.UnitPriceCents, &l.TaxRateBP, &l.TaxCents, &l.DiscountCents,
		&l.LineTotalCents, &l.Position)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if productID.Valid {
		v := uuid.UUID(productID.Bytes)
		l.ProductID = &v
	}
	if variantID.Valid {
		v := uuid.UUID(variantID.Bytes)
		l.VariantID = &v
	}
	return &l, nil
}
