// Package portal is the CUSTOMER-only surface. Every route is scoped by the
// authenticated actor's CustomerID: a customer can only ever read their own
// subscriptions, invoices and payments. There are no write endpoints here.
package portal

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/payments"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
)

// Service exposes the shared repositories with customer scoping.
type Service struct {
	pool *pgxpool.Pool
	subs *subscriptions.Service
	inv  *invoices.Service
	pay  *payments.Service
}

func NewService(pool *pgxpool.Pool, subs *subscriptions.Service, inv *invoices.Service, pay *payments.Service) *Service {
	return &Service{pool: pool, subs: subs, inv: inv, pay: pay}
}

func customerID(c *gin.Context) (uuid.UUID, bool) {
	actor, _ := auth.CurrentActor(c)
	if actor == nil || actor.CustomerID == nil {
		httpx.Fail(c, httpx.Forbidden("your account is not linked to a customer profile"))
		return uuid.Nil, false
	}
	return *actor.CustomerID, true
}

func (s *Service) ownSubscription(ctx context.Context, customerID, id uuid.UUID) (*subscriptions.Subscription, error) {
	sub, err := s.subs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sub.CustomerID != customerID {
		return nil, httpx.NotFound("subscription not found")
	}
	return sub, nil
}

func (s *Service) ownInvoice(ctx context.Context, customerID, id uuid.UUID) (*invoices.Invoice, error) {
	inv, err := s.inv.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if inv.CustomerID != customerID {
		return nil, httpx.NotFound("invoice not found")
	}
	return inv, nil
}

// Handlers ---------------------------------------------------------------

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Me returns the linked customer profile plus counts for the landing page.
func (h *Handler) Me(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	cid, ok := customerID(c)
	if !ok {
		return
	}
	pg := pagination.Parse(c)
	openInvoices, invoiceTotal, invErr := h.svc.inv.List(c.Request.Context(), invoices.Filter{
		CustomerID: &cid, Page: pg})
	openSubs, subTotal, subErr := h.svc.subs.List(c.Request.Context(), subscriptions.Filter{
		CustomerID: &cid, Page: pg})
	_ = invErr
	_ = subErr
	httpx.OK(c, gin.H{
		"customer_id":     cid,
		"email":           actor.Email,
		"full_name":       actor.FullName,
		"subscriptions":   subTotal,
		"active_status":   countStatus(openSubs, subscriptions.StatusActive),
		"invoices":        invoiceTotal,
		"unpaid_invoices": countUnpaid(openInvoices),
	})
}

func countStatus(subs []subscriptions.Subscription, status string) int {
	n := 0
	for _, s := range subs {
		if s.Status == status {
			n++
		}
	}
	return n
}

func countUnpaid(invs []invoices.Invoice) int {
	n := 0
	for _, i := range invs {
		if i.Status == invoices.StatusConfirmed && i.BalanceCents > 0 {
			n++
		}
	}
	return n
}

func (h *Handler) Subscriptions(c *gin.Context) {
	cid, ok := customerID(c)
	if !ok {
		return
	}
	pg := pagination.Parse(c)
	items, total, err := h.svc.subs.List(c.Request.Context(), subscriptions.Filter{
		CustomerID: &cid, Page: pg})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(pg, total))
}

func (h *Handler) Subscription(c *gin.Context) {
	cid, ok := customerID(c)
	if !ok {
		return
	}
	subID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid subscription id"))
		return
	}
	sub, err := h.svc.ownSubscription(c.Request.Context(), cid, subID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, sub)
}

func (h *Handler) Invoices(c *gin.Context) {
	cid, ok := customerID(c)
	if !ok {
		return
	}
	pg := pagination.Parse(c)
	items, total, err := h.svc.inv.List(c.Request.Context(), invoices.Filter{
		CustomerID: &cid, Page: pg})
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(pg, total))
}

func (h *Handler) Invoice(c *gin.Context) {
	cid, ok := customerID(c)
	if !ok {
		return
	}
	invID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid invoice id"))
		return
	}
	inv, err := h.svc.ownInvoice(c.Request.Context(), cid, invID)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, inv)
}

func (h *Handler) Payments(c *gin.Context) {
	cid, ok := customerID(c)
	if !ok {
		return
	}
	pg := pagination.Parse(c)
	items, total, err := h.svc.pay.ListByCustomer(c.Request.Context(), cid, pg)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.List(c, items, pagination.Meta(pg, total))
}
