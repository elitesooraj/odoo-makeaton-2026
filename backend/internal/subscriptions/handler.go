package subscriptions

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
