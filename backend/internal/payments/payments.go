// Package payments owns payment records against invoices.
//
// A payment stores method, amount and paid_at, and is always linked to exactly
// one invoice. Recording a payment and updating the invoice balance happen in
// the same transaction; the invoices module owns the balance update
// (Repository.ApplyPayment) so the balance and status rules live in one place.
// Overpayments are rejected, never allowed to drive a balance negative.
package payments

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
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

var ErrNotFound = errors.New("payment not found")

const (
	MethodCash        = "CASH"
	MethodBank        = "BANK_TRANSFER"
	MethodCard        = "CARD"
	MethodMobileMoney = "MOBILE_MONEY"
	MethodOther       = "OTHER"
)

var validMethods = map[string]struct{}{
	MethodCash: {}, MethodBank: {}, MethodCard: {}, MethodMobileMoney: {}, MethodOther: {},
}

type Payment struct {
	ID          uuid.UUID  `json:"id"`
	InvoiceID   uuid.UUID  `json:"invoice_id"`
	Method      string     `json:"method"`
	AmountCents int64      `json:"amount_cents"`
	PaidAt      time.Time  `json:"paid_at"`
	Note        *string    `json:"note"`
	RecordedBy  *uuid.UUID `json:"recorded_by"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const columns = `id, invoice_id, method, amount_cents, paid_at, note, recorded_by, created_at`

func (r *Repository) Insert(ctx context.Context, q db.Querier, p *Payment) error {
	_, err := q.Exec(ctx, `INSERT INTO payments
		(id, invoice_id, method, amount_cents, paid_at, note, recorded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.InvoiceID, p.Method, p.AmountCents, p.PaidAt, id.StringArg(p.Note), id.UUIDArg(p.RecordedBy))
	return err
}

func (r *Repository) Get(ctx context.Context, q db.Querier, paymentID uuid.UUID) (*Payment, error) {
	return scan(q.QueryRow(ctx, `SELECT `+columns+` FROM payments WHERE id = $1`, paymentID))
}

func (r *Repository) List(ctx context.Context, invoiceID *uuid.UUID, page pagination.Page) ([]Payment, int, error) {
	where := `WHERE ($1::uuid IS NULL OR invoice_id = $1)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM payments `+where, id.UUIDArg(invoiceID)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM payments `+where+`
		ORDER BY paid_at DESC, created_at DESC LIMIT $2 OFFSET $3`,
		id.UUIDArg(invoiceID), page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Payment, 0)
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// ListByCustomer exposes every payment made by one customer, for the portal.
func (r *Repository) ListByCustomer(ctx context.Context, customerID uuid.UUID, page pagination.Page) ([]Payment, int, error) {
	const where = `WHERE invoice_id IN (SELECT id FROM invoices WHERE customer_id = $1)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM payments `+where, customerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+columns+` FROM payments `+where+`
		ORDER BY paid_at DESC, created_at DESC LIMIT $2 OFFSET $3`,
		customerID, page.PerPage, page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]Payment, 0)
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	return out, total, rows.Err()
}

// Service applies payments to invoices.
type Service struct {
	repo     *Repository
	invoices *invoices.Repository
	audit    *audit.Recorder
}

func NewService(repo *Repository, invoicesRepo *invoices.Repository, recorder *audit.Recorder) *Service {
	return &Service{repo: repo, invoices: invoicesRepo, audit: recorder}
}

type RecordInput struct {
	InvoiceID   string  `json:"invoice_id"`
	Method      string  `json:"method"`
	AmountCents int64   `json:"amount_cents"`
	PaidAt      *string `json:"paid_at"`
	Note        string  `json:"note"`
}

// Record inserts a payment and closes the loop on the invoice balance in one
// transaction, returning the updated invoice.
func (s *Service) Record(ctx context.Context, actor *auth.Actor, meta audit.Meta, in RecordInput) (*invoices.Invoice, error) {
	details := map[string]string{}
	invoiceID, err := uuid.Parse(strings.TrimSpace(in.InvoiceID))
	if err != nil {
		details["invoice_id"] = "a valid invoice id is required"
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if _, ok := validMethods[method]; !ok {
		details["method"] = "method must be CASH, BANK_TRANSFER, CARD, MOBILE_MONEY or OTHER"
	}
	if in.AmountCents <= 0 {
		details["amount_cents"] = "a payment must be a positive amount"
	}
	paidAt := time.Now().UTC()
	if in.PaidAt != nil && strings.TrimSpace(*in.PaidAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*in.PaidAt))
		if err != nil {
			details["paid_at"] = "use an RFC3339 timestamp"
		} else {
			paidAt = parsed
		}
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	var after *invoices.Invoice
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		inv, err := s.invoices.Get(ctx, tx, invoiceID)
		if err != nil {
			if errors.Is(err, invoices.ErrNotFound) {
				return httpx.Validation(map[string]string{"invoice_id": "invoice not found"})
			}
			return err
		}
		if inv.Status != invoices.StatusConfirmed && inv.Status != invoices.StatusPaid {
			return httpx.Conflict("only a CONFIRMED invoice accepts payments")
		}
		if in.AmountCents > inv.BalanceCents {
			return httpx.Validation(map[string]string{
				"amount_cents": "exceeds the remaining balance of " + strconv.FormatInt(inv.BalanceCents, 10),
			})
		}

		p := &Payment{
			ID:          id.New(),
			InvoiceID:   invoiceID,
			Method:      method,
			AmountCents: in.AmountCents,
			PaidAt:      paidAt,
			RecordedBy:  uid(actor),
		}
		if in.Note != "" {
			p.Note = &in.Note
		}
		if err := s.repo.Insert(ctx, tx, p); err != nil {
			return err
		}
		if err := s.invoices.ApplyPayment(ctx, tx, invoiceID, in.AmountCents, time.Now().UTC()); err != nil {
			var transition invoices.ErrTransition
			if errors.As(err, &transition) {
				return httpx.Conflict("this invoice cannot accept that payment")
			}
			return err
		}
		updated, err := s.invoices.Get(ctx, tx, invoiceID)
		if err != nil {
			return err
		}
		after = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID: uid(actor), ActorRole: role(actor),
			Action: "payment.record", EntityType: "invoice",
			EntityID: invoiceID, After: p, Meta: meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return after, nil
}

func (s *Service) Get(ctx context.Context, paymentID uuid.UUID) (*Payment, error) {
	p, err := s.repo.Get(ctx, s.repo.pool, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, httpx.NotFound("payment not found")
		}
		return nil, err
	}
	return p, nil
}

func (s *Service) List(ctx context.Context, invoiceID *uuid.UUID, page pagination.Page) ([]Payment, int, error) {
	return s.repo.List(ctx, invoiceID, page)
}

func (s *Service) ListByCustomer(ctx context.Context, customerID uuid.UUID, page pagination.Page) ([]Payment, int, error) {
	return s.repo.ListByCustomer(ctx, customerID, page)
}

// Handlers ---------------------------------------------------------------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Create(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	meta := auditMeta(c)
	var in RecordInput
	if err := httpx.BindJSON(c, &in); err != nil {
		httpx.Fail(c, err)
		return
	}
	inv, err := h.svc.Record(c.Request.Context(), actor, meta, in)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, gin.H{"payment_recorded": true, "invoice": inv})
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	var invoiceID *uuid.UUID
	if raw := c.Query("invoice_id"); raw != "" {
		if parsed, err := uuid.Parse(raw); err == nil {
			invoiceID = &parsed
		} else {
			httpx.Fail(c, httpx.BadRequest("invalid invoice_id"))
			return
		}
	}
	items, total, err := h.svc.List(c.Request.Context(), invoiceID, page)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid payment id"))
		return
	}
	p, err := h.svc.Get(c.Request.Context(), paymentID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, p)
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
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

func scan(row pgx.Row) (*Payment, error) {
	var (
		p          Payment
		paidAt     pgtype.Timestamptz
		note       pgtype.Text
		recordedBy pgtype.UUID
	)
	err := row.Scan(&p.ID, &p.InvoiceID, &p.Method, &p.AmountCents, &paidAt, &note,
		&recordedBy, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if paidAt.Valid {
		p.PaidAt = paidAt.Time
	}
	if note.Valid {
		v := note.String
		p.Note = &v
	}
	if recordedBy.Valid {
		v := uuid.UUID(recordedBy.Bytes)
		p.RecordedBy = &v
	}
	return &p, nil
}
