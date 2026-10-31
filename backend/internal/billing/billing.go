// Package billing implements recurring invoicing for ACTIVE subscriptions.
//
// Thin by design for the hackathon: one run finds every ACTIVE subscription
// whose next_billing_date is due, snapshots an invoice for the period, advances
// next_billing_date by the plan interval and records a billing_runs row. The
// unique index on invoices(subscription_id, period_start) plus the billing_runs
// key make a run idempotent; per-subscription work is one transaction.
package billing

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/money"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type RunResult struct {
	SubscriptionID string    `json:"subscription_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	InvoiceID      *string   `json:"invoice_id,omitempty"`
	Status         string    `json:"status"` // CREATED, SKIPPED, CLOSED, FAILED
	Detail         string    `json:"detail,omitempty"`
}

type RunSummary struct {
	AsOf     time.Time   `json:"as_of"`
	Billed   int         `json:"billed"`
	Skipped  int         `json:"skipped"`
	Closed   int         `json:"closed"`
	Failed   int         `json:"failed"`
	Invoices []uuid.UUID `json:"invoices,omitempty"`
	Results  []RunResult `json:"results"`
}

type Result struct {
	Billed  int
	Skipped int
	Closed  int
	Failed  int
}

type BillingRuns struct{ pool *pgxpool.Pool }

func NewBillingRuns(pool *pgxpool.Pool) *BillingRuns { return &BillingRuns{pool: pool} }

func (r *BillingRuns) Record(ctx context.Context, q db.Querier, res RunResult, invoiceID *uuid.UUID, detail string) (fresh bool, err error) {
	tag, err := q.Exec(ctx, `INSERT INTO billing_runs
		(id, subscription_id, period_start, period_end, invoice_id, status, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (subscription_id, period_start) DO NOTHING`,
		id.New(), res.SubscriptionID, res.PeriodStart, res.PeriodEnd,
		invoiceID, statusOf(res), detail)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func statusOf(res RunResult) string {
	switch res.Status {
	case "SKIPPED", "CLOSED", "FAILED":
		return res.Status
	default:
		return "CREATED"
	}
}

// Service owns a whole run.
type Service struct {
	pool     *pgxpool.Pool
	subs     *subscriptions.Service
	invoices *invoices.Service
	runs     *BillingRuns
	audit    *audit.Recorder

	mu sync.Mutex // one run at a time
}

func NewService(pool *pgxpool.Pool, subs *subscriptions.Service, inv *invoices.Service, recorder *audit.Recorder) *Service {
	return &Service{
		pool:     pool,
		subs:     subs,
		invoices: inv,
		runs:     NewBillingRuns(pool),
		audit:    recorder,
	}
}

func (s *Service) Run(ctx context.Context, actor *auth.Actor, meta audit.Meta) (*RunSummary, error) {
	if !s.mu.TryLock() {
		return nil, httpx.Conflict("a billing run is already in progress")
	}
	defer s.mu.Unlock()

	due := time.Now().UTC()
	page := pagination.Page{Number: 1, PerPage: 100}
	summary := &RunSummary{AsOf: due, Results: make([]RunResult, 0)}

	for {
		subs, _, err := s.subs.Repo().ListForBilling(ctx, due, page)
		if err != nil {
			return nil, err
		}
		if len(subs) == 0 {
			break
		}
		for i := range subs {
			summary.Results = append(summary.Results, s.billOne(ctx, actor, meta, &subs[i], due))
		}
		if len(subs) < page.PerPage {
			break
		}
		page.Number++
	}
	for _, r := range summary.Results {
		switch r.Status {
		case "CREATED":
			summary.Billed++
		case "SKIPPED":
			summary.Skipped++
		case "CLOSED":
			summary.Closed++
		case "FAILED":
			summary.Failed++
		}
	}
	return summary, nil
}

func (s *Service) billOne(ctx context.Context, actor *auth.Actor, meta audit.Meta, sub *subscriptions.Subscription, now time.Time) RunResult {
	base := RunResult{SubscriptionID: sub.ID.String()}

	// Plan flags come from the JOIN in ListForBilling; keep them across the
	// re-fetch below, which only exists to load the subscription's lines.
	planFlags := sub.Plan
	if planFlags == nil {
		loaded, err := s.subs.Repo().PlanFlagsByID(ctx, s.pool, sub.PlanID)
		if err != nil {
			return fail(base, err)
		}
		planFlags = loaded
	}
	if planFlags == nil {
		return fail(base, errors.New("plan missing"))
	}
	flags := planFlags

	// The billing listing does not load lines; re-fetch with lines so the
	// invoice snapshot has items to copy.
	fullSub, err := s.subs.Get(ctx, sub.ID)
	if err != nil {
		return fail(base, err)
	}
	fullSub.Plan = planFlags
	sub = fullSub

	if sub.NextBillingDate == nil {
		return fail(base, errors.New("next_billing_date is not set"))
	}

	periodStart := dateOnly(*sub.NextBillingDate)
	periodEnd := dateOnly(money.AddInterval(periodStart, flags.Interval))

	// Expiry-aware renewal handling.
	subClosed := false
	if sub.ExpiryDate != nil {
		expiry := dateOnly(*sub.ExpiryDate)
		if periodStart.After(expiry) && !flags.AutoRenew {
			// Term ended and the plan does not auto-renew: close quietly.
			base.Status = "CLOSED"
			if err := s.close(ctx, sub.ID); err != nil {
				return fail(base, err)
			}
			return base
		}
		if !flags.AutoRenew && periodEnd.After(expiry) {
			// Final period spills past expiry: bill it once, then close.
			subClosed = true
		}
	}

	var invoiceID *uuid.UUID
	var result RunResult
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		inv, err := s.invoices.CreateForBilling(ctx, tx, actorID(actor), meta, sub, periodStart, periodEnd)
		if err != nil {
			return err
		}
		invoiceID = &inv.ID

		var next time.Time
		if subClosed {
			// Last invoice: no further billing date; the subscription closes.
			if _, err := tx.Exec(ctx, `UPDATE subscriptions SET next_billing_date = NULL,
				status = 'CLOSED', closed_at = $2 WHERE id = $1`, sub.ID, now); err != nil {
				return err
			}
			result = RunResult{
				SubscriptionID: sub.ID.String(),
				PeriodStart:    periodStart,
				PeriodEnd:      periodEnd,
				InvoiceID:      ptr(inv.ID.String()),
				Status:         "CLOSED",
				Detail:         "billed final period and closed",
			}
		} else {
			next = periodEnd
			if _, err := tx.Exec(ctx, `UPDATE subscriptions SET next_billing_date = $2 WHERE id = $1`,
				sub.ID, next); err != nil {
				return err
			}
			result = RunResult{
				SubscriptionID: sub.ID.String(),
				PeriodStart:    periodStart,
				PeriodEnd:      periodEnd,
				InvoiceID:      ptr(inv.ID.String()),
				Status:         "CREATED",
			}
		}
		// Audit trail: record the run. ON CONFLICT DO NOTHING keeps this
		// idempotent if a stale row exists from an earlier version.
		_, _ = s.runs.Record(ctx, tx, result, invoiceID, result.Detail)
		return nil
	})
	if err != nil {
		// The invoices(subscription_id, period_start) unique index refuses a
		// duplicate: report the period as already invoiced, not failed.
		if strings.Contains(err.Error(), "invoices_subscription_period_uq") {
			base.PeriodStart = periodStart
			base.PeriodEnd = periodEnd
			base.Status = "SKIPPED"
			base.Detail = "a confirmed invoice already exists for this period"
			return base
		}
		return fail(base, err)
	}
	return result
}

func (s *Service) close(ctx context.Context, subscriptionID uuid.UUID) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE subscriptions SET status = 'CLOSED', closed_at = $2,
			next_billing_date = NULL WHERE id = $1`, subscriptionID, time.Now().UTC())
		return err
	})
}

func fail(base RunResult, err error) RunResult {
	base.Status = "FAILED"
	base.Detail = err.Error()
	return base
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func ptr(s string) *string { return &s }

func actorID(actor *auth.Actor) *uuid.UUID {
	if actor == nil {
		return nil
	}
	return &actor.UserID
}

// Handlers ---------------------------------------------------------------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Run(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	summary, err := h.svc.Run(c.Request.Context(), actor, meta)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	// surface the invoice numbers for quick inspection
	ids := make([]uuid.UUID, 0)
	for _, r := range summary.Results {
		if r.InvoiceID != nil {
			if id, err := uuid.Parse(*r.InvoiceID); err == nil {
				ids = append(ids, id)
			}
		}
	}
	summary.Invoices = ids
	httpx.OK(c, summary)
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
}
