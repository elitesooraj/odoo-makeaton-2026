package invoices

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

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
