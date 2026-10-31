package invoices

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// draftLine carries the inclusive/exclusive flag across the finalize step.

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
		taxTotal += tax
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
