// Package reports exposes read-only aggregates for the back office.
//
// Thin by design: every endpoint is one SQL query with no business logic.
// MRR approximates a monthly figure from the plan's interval (daily plans are
// multiplied by 30, weekly by 52/12), which is documented in the API reference.
package reports

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Summary is the top-of-page dashboard payload.
type Summary struct {
	ActiveSubscriptions     int   `json:"active_subscriptions"`
	PausedSubscriptions     int   `json:"paused_subscriptions"`
	QuotationsOpen          int   `json:"open_quotations"`
	MonthlyRecurringCents   int64 `json:"mrr_cents"`
	OutstandingCents        int64 `json:"outstanding_cents"`
	OverdueCount            int   `json:"overdue_invoices"`
	OverdueCents            int64 `json:"overdue_cents"`
	CollectedThisMonthCents int64 `json:"collected_this_month_cents"`
}

func (s *Service) Summary(ctx context.Context) (*Summary, error) {
	out := &Summary{}
	err := s.pool.QueryRow(ctx, `SELECT
		COALESCE(COUNT(*) FILTER (WHERE status = 'ACTIVE'), 0),
		COALESCE(COUNT(*) FILTER (WHERE status = 'PAUSED'), 0),
		COALESCE(COUNT(*) FILTER (WHERE status = 'QUOTATION'), 0)
		FROM subscriptions`).Scan(&out.ActiveSubscriptions, &out.PausedSubscriptions, &out.QuotationsOpen)
	if err != nil {
		return nil, err
	}
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(
		price_cents * s.quantity * CASE p.interval
			WHEN 'YEARLY' THEN 1.0/12
			WHEN 'WEEKLY' THEN 52.0/12
			WHEN 'DAILY'  THEN 30
			ELSE 1 END), 0)::bigint
		FROM subscriptions s JOIN plans p ON p.id = s.plan_id
		WHERE s.status = 'ACTIVE'`).Scan(&out.MonthlyRecurringCents)
	if err != nil {
		return nil, err
	}
	err = s.pool.QueryRow(ctx, `SELECT
		COALESCE(SUM(balance_cents) FILTER (WHERE status = 'CONFIRMED'), 0),
		COALESCE(COUNT(*) FILTER (WHERE status = 'CONFIRMED' AND balance_cents > 0
			AND due_date < current_date), 0),
		COALESCE(SUM(balance_cents) FILTER (WHERE status = 'CONFIRMED' AND balance_cents > 0
			AND due_date < current_date), 0)
		FROM invoices`).Scan(&out.OutstandingCents, &out.OverdueCount, &out.OverdueCents)
	if err != nil {
		return nil, err
	}
	err = s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents), 0)
		FROM payments WHERE paid_at >= date_trunc('month', current_date)`).Scan(&out.CollectedThisMonthCents)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type RevenuePoint struct {
	Month         string `json:"month"`
	PaidCents     int64  `json:"paid_cents"`
	InvoicedCents int64  `json:"invoiced_cents"`
}

// Revenue returns a paid-vs-invoiced series for the last N months.
func (s *Service) Revenue(ctx context.Context, months int) ([]RevenuePoint, error) {
	if months < 1 || months > 24 {
		months = 12
	}
	rows, err := s.pool.Query(ctx, `
		WITH series AS (
			SELECT generate_series(
				date_trunc('month', current_date) - make_interval(months => $1 - 1),
				date_trunc('month', current_date),
				'1 month')::date AS month)
		SELECT to_char(ss.month, 'YYYY-MM'),
			COALESCE(p.paid, 0)::bigint,
			COALESCE(i.invoiced, 0)::bigint
		FROM series ss
		LEFT JOIN (
			SELECT date_trunc('month', paid_at)::date AS m, SUM(amount_cents) AS paid
			FROM payments GROUP BY 1) p ON p.m = ss.month
		LEFT JOIN (
			SELECT date_trunc('month', issue_date)::date AS m, SUM(total_cents) AS invoiced
			FROM invoices WHERE status IN ('CONFIRMED','PAID') GROUP BY 1) i ON i.m = ss.month
		ORDER BY ss.month`, months)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RevenuePoint, 0)
	for rows.Next() {
		var pt RevenuePoint
		if err := rows.Scan(&pt.Month, &pt.PaidCents, &pt.InvoicedCents); err != nil {
			return nil, err
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

func (s *Service) SubscriptionsByStatus(ctx context.Context) ([]StatusCount, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM subscriptions
		GROUP BY status ORDER BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]StatusCount, 0)
	for rows.Next() {
		var sc StatusCount
		if err := rows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

type OverdueInvoice struct {
	ID           uuid.UUID `json:"id"`
	Number       string    `json:"number"`
	CustomerID   uuid.UUID `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	DueDate      time.Time `json:"due_date"`
	DaysLate     int       `json:"days_late"`
	BalanceCents int64     `json:"balance_cents"`
	TotalCents   int64     `json:"total_cents"`
}

func (s *Service) OverdueInvoices(ctx context.Context) ([]OverdueInvoice, error) {
	rows, err := s.pool.Query(ctx, `SELECT i.id, i.number, i.customer_id, c.name,
		i.due_date, (current_date - i.due_date), i.balance_cents, i.total_cents
		FROM invoices i JOIN customers c ON c.id = i.customer_id
		WHERE i.status = 'CONFIRMED' AND i.balance_cents > 0 AND i.due_date < current_date
		ORDER BY i.due_date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]OverdueInvoice, 0)
	for rows.Next() {
		var o OverdueInvoice
		if err := rows.Scan(&o.ID, &o.Number, &o.CustomerID, &o.CustomerName, &o.DueDate,
			&o.DaysLate, &o.BalanceCents, &o.TotalCents); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Handlers ---------------------------------------------------------------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Summary(c *gin.Context) {
	out, err := h.svc.Summary(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, out)
}

func (h *Handler) Revenue(c *gin.Context) {
	months := positiveInt(c.Query("months"), 12)
	points, err := h.svc.Revenue(c.Request.Context(), months)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, points)
}

func (h *Handler) SubscriptionsByStatus(c *gin.Context) {
	out, err := h.svc.SubscriptionsByStatus(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, out)
}

func (h *Handler) Overdue(c *gin.Context) {
	out, err := h.svc.OverdueInvoices(c.Request.Context())
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, out)
}

func positiveInt(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
