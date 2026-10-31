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
)

type Service struct {
	repo      *Repository
	audit     *audit.Recorder
	plans     PlanLoader
	customers CustomerLoader
}

func NewService(pool *pgxpool.Pool, recorder *audit.Recorder, plans PlanLoader, customers CustomerLoader) *Service {
	repo := NewRepository(pool)
	if customers == nil {
		customers = repo // default: check against the database directly
	}
	return &Service{repo: repo, audit: recorder, plans: plans, customers: customers}
}

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

	if ok, err := s.customers.Exists(ctx, customerID); err != nil {
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
	_ = before
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
	}
	if current.ExpiryDate != nil && current.StartDate.After(*current.ExpiryDate) {
		details["expiry_date"] = "must be on or after start_date"
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	if err != nil {
		return nil, err
	}
	return s.Get(ctx, subscriptionID)
}

// runTransition is the shared skeleton for every lifecycle hop: validate the
// current state, run the mutation inside one transaction, reload the row,
// carry the lines over (the in-tx Get does not load them) and audit it.
// Keeping this in one place means the state machine reads as a list of
// (validate, mutate) pairs instead of six near-identical blocks.
func (s *Service) runTransition(
	ctx context.Context, actor *auth.Actor, meta audit.Meta,
	subscriptionID uuid.UUID, action string,
	validate func(*Subscription) error,
	mutate func(ctx context.Context, tx pgx.Tx, sub *Subscription) error,
) (*Subscription, error) {
	current, err := s.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if err := validate(current); err != nil {
		return nil, err
	}

	var stored *Subscription
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := mutate(ctx, tx, current); err != nil {
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
			Action: action, EntityType: "subscription",
			EntityID: subscriptionID, Before: current, After: updated, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (s *Service) Confirm(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	return s.runTransition(ctx, actor, meta, subscriptionID, "subscription.confirm",
		func(sub *Subscription) error {
			if sub.Status != StatusDraft && sub.Status != StatusQuotation {
				return httpx.Conflict("only DRAFT or QUOTATION subscriptions can be confirmed")
			}
			if sub.QuoteExpiresAt != nil && time.Now().UTC().After(*sub.QuoteExpiresAt) {
				return httpx.Conflict("this quotation has expired")
			}
			return nil
		},
		func(ctx context.Context, tx pgx.Tx, sub *Subscription) error {
			if err := s.repo.SetStatus(ctx, tx, sub.ID, StatusConfirmed); err != nil {
				return err
			}
			return s.repo.SetNextBillingDate(ctx, tx, sub.ID, sub.StartDate)
		},
	)
}

func (s *Service) Activate(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	return s.runTransition(ctx, actor, meta, subscriptionID, "subscription.activate",
		func(sub *Subscription) error {
			if sub.Status != StatusConfirmed {
				return httpx.Conflict("only CONFIRMED subscriptions can be activated")
			}
			return nil
		},
		func(ctx context.Context, tx pgx.Tx, sub *Subscription) error {
			now := time.Now().UTC()
			if err := s.repo.MarkActivated(ctx, tx, sub.ID, now); err != nil {
				return err
			}
			var nextBilling time.Time
			if sub.NextBillingDate != nil {
				nextBilling = dateOnly(*sub.NextBillingDate)
			} else {
				nextBilling = dateOnly(sub.StartDate)
			}
			return s.repo.SetNextBillingDate(ctx, tx, sub.ID, nextBilling)
		},
	)
}

func (s *Service) Pause(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	return s.runTransition(ctx, actor, meta, subscriptionID, "subscription.pause",
		func(sub *Subscription) error {
			if sub.Status != StatusActive {
				return httpx.Conflict("only ACTIVE subscriptions can be paused")
			}
			flags, err := s.planFlags(ctx, sub.PlanID)
			if err != nil {
				return err
			}
			if !flags.AllowPause {
				return httpx.Conflict("the plan on this subscription does not allow pausing")
			}
			return nil
		},
		func(ctx context.Context, tx pgx.Tx, sub *Subscription) error {
			return s.repo.MarkPaused(ctx, tx, sub.ID, time.Now().UTC())
		},
	)
}

func (s *Service) Resume(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	return s.runTransition(ctx, actor, meta, subscriptionID, "subscription.resume",
		func(sub *Subscription) error {
			if sub.Status != StatusPaused {
				return httpx.Conflict("only PAUSED subscriptions can be resumed")
			}
			return nil
		},
		func(ctx context.Context, tx pgx.Tx, sub *Subscription) error {
			return s.repo.MarkResumed(ctx, tx, sub.ID, today())
		},
	)
}

func (s *Service) Close(ctx context.Context, actor *auth.Actor, meta audit.Meta, subscriptionID uuid.UUID) (*Subscription, error) {
	return s.runTransition(ctx, actor, meta, subscriptionID, "subscription.close",
		func(sub *Subscription) error {
			if sub.Status != StatusActive && sub.Status != StatusPaused && sub.Status != StatusConfirmed {
				return httpx.Conflict("only ACTIVE, PAUSED or CONFIRMED subscriptions can be closed")
			}
			flags, err := s.planFlags(ctx, sub.PlanID)
			if err != nil {
				return err
			}
			if sub.ExpiryDate != nil && today().Before(*sub.ExpiryDate) && !flags.AllowEarlyClose {
				return httpx.Conflict("the plan on this subscription does not allow early closes")
			}
			return nil
		},
		func(ctx context.Context, tx pgx.Tx, sub *Subscription) error {
			return s.repo.MarkClosed(ctx, tx, sub.ID, time.Now().UTC())
		},
	)
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
	quantity := in.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if in.UnitPriceCents < 0 {
		details["unit_price_cents"] = "must not be negative"
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

// editableStatus gates PATCH /subscriptions/:id. Only DRAFT and QUOTATION
// subscriptions are editable; every other state is frozen so the audit
// trail stays consistent with the lifecycle documented in docs/API.md.
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

// scanForBilling scans a row from ListForBilling: subscription columns followed
// by plan columns. It populates sub.Plan so the billing worker avoids a
// separate PlanFlagsByID round-trip.
func scanForBilling(row pgx.Row) (*Subscription, error) {
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
		// plan columns
		planID     pgtype.UUID
		productID  pgtype.UUID
		taxID      pgtype.UUID
		planName   pgtype.Text
		interval   pgtype.Text
		priceCents int64
		minQty     int
		autoClose  bool
		autoRenew  bool
		allowPause bool
		allowEarly bool
		isActive   bool
	)
	err := row.Scan(&s.ID, &s.Number, &s.CustomerID, &s.PlanID, &s.Status, &startDate,
		&expiryDate, &s.PaymentTermsDays, &s.Quantity, &discountID, &nextBilling,
		&s.QuoteExpiresAt, &notes, &createdBy, &activatedAt, &pausedAt, &closedAt,
		&s.CreatedAt, &s.UpdatedAt,
		&planID, &productID, &taxID, &planName, &interval, &priceCents, &minQty,
		&autoClose, &autoRenew, &allowPause, &allowEarly, &isActive)
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
	// Populate plan flags when the join matched.
	if planID.Valid {
		pid := uuid.UUID(planID.Bytes)
		spec := &PlanSpec{
			ID:              pid,
			ProductID:       uuid.UUID(productID.Bytes),
			Name:            planName.String,
			Interval:        interval.String,
			PriceCents:      priceCents,
			MinimumQuantity: minQty,
			AutoClose:       autoClose,
			AutoRenew:       autoRenew,
			AllowPause:      allowPause,
			AllowEarlyClose: allowEarly,
			IsActive:        isActive,
		}
		if taxID.Valid {
			v := uuid.UUID(taxID.Bytes)
			spec.TaxID = &v
		}
		s.Plan = spec
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
