package customers

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type customerDTO struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Email          string          `json:"email,omitempty"`
	Phone          string          `json:"phone,omitempty"`
	BillingAddress json.RawMessage `json:"billing_address"`
	TaxID          string          `json:"tax_id,omitempty"`
	Status         string          `json:"status"`
	Notes          string          `json:"notes,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func newDTO(c *Customer) customerDTO {
	dto := customerDTO{
		ID:             c.ID.String(),
		Name:           c.Name,
		BillingAddress: c.BillingAddress,
		Status:         c.Status,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
	if c.Email != nil {
		dto.Email = *c.Email
	}
	if c.Phone != nil {
		dto.Phone = *c.Phone
	}
	if c.TaxID != nil {
		dto.TaxID = *c.TaxID
	}
	if c.Notes != nil {
		dto.Notes = *c.Notes
	}
	if len(dto.BillingAddress) == 0 {
		dto.BillingAddress = json.RawMessage(`{}`)
	}
	return dto
}

func (h *Handler) Create(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	var req CreateInput
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	customer, err := h.svc.Create(c.Request.Context(), actor, auditMeta(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, newDTO(customer))
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	customers, total, err := h.svc.List(c.Request.Context(), PageFilter(c.Query("search"), c.Query("status"), page))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	items := make([]customerDTO, 0, len(customers))
	for i := range customers {
		items = append(items, newDTO(&customers[i]))
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid customer id"))
		return
	}
	customer, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, newDTO(customer))
}

func (h *Handler) Update(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid customer id"))
		return
	}
	var req UpdateInput
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	customer, err := h.svc.Update(c.Request.Context(), actor, auditMeta(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, newDTO(customer))
}

func (h *Handler) Deactivate(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid customer id"))
		return
	}
	customer, err := h.svc.Deactivate(c.Request.Context(), actor, auditMeta(c), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, newDTO(customer))
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
}
