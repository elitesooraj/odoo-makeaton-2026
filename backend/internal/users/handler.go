package users

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/authz"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

type userDTO struct {
	ID                 string     `json:"id"`
	Email              string     `json:"email"`
	FullName           string     `json:"full_name"`
	Role               string     `json:"role"`
	Status             string     `json:"status"`
	EmailVerified      bool       `json:"email_verified"`
	MustChangePassword bool       `json:"must_change_password"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func newUserDTO(u *auth.User) userDTO {
	return userDTO{
		ID:                 u.ID.String(),
		Email:              u.Email,
		FullName:           u.FullName,
		Role:               u.Role,
		Status:             u.Status,
		EmailVerified:      u.EmailVerifiedAt != nil,
		MustChangePassword: u.MustChangePassword,
		LastLoginAt:        u.LastLoginAt,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
	}
}

func (h *Handler) Create(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	var req CreateInput
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	user, tempPassword, err := h.svc.CreateStaff(c.Request.Context(), actor, auditMeta(c), req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Created(c, gin.H{
		"user":               newUserDTO(user),
		"temporary_password": tempPassword,
	})
}

func (h *Handler) List(c *gin.Context) {
	page := pagination.Parse(c)
	users, total, err := h.svc.List(c.Request.Context(), PageFilter(c.Query("search"), c.Query("role"), c.Query("status"), page))
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	items := make([]userDTO, 0, len(users))
	for i := range users {
		items = append(items, newUserDTO(&users[i]))
	}
	httpx.List(c, items, pagination.Meta(page, total))
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	user, permissions, overrides, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{
		"user":        newUserDTO(user),
		"permissions": permissions,
		"overrides":   overrides,
	})
}

func (h *Handler) Update(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req UpdateInput
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	user, err := h.svc.Update(c.Request.Context(), actor, auditMeta(c), id, req)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, newUserDTO(user))
}

type permissionOverride struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
}

type setPermissionsRequest struct {
	Overrides []permissionOverride `json:"overrides"`
}

func (h *Handler) SetPermissions(c *gin.Context) {
	actor, _ := auth.CurrentActor(c)
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req setPermissionsRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	overrides := make([]Override, 0, len(req.Overrides))
	for _, o := range req.Overrides {
		overrides = append(overrides, Override{Permission: o.Permission, Effect: o.Effect})
	}
	permissions, err := h.svc.SetPermissions(c.Request.Context(), actor, auditMeta(c), id, overrides)
	if err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.OK(c, gin.H{"permissions": permissions})
}

func (h *Handler) Catalog(c *gin.Context) {
	httpx.OK(c, authz.Catalog)
}

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Fail(c, httpx.BadRequest("invalid user id"))
		return uuid.Nil, false
	}
	return id, true
}

func auditMeta(c *gin.Context) audit.Meta {
	meta := httpx.ClientMetaFrom(c)
	return audit.Meta{IP: meta.IP, UserAgent: meta.UserAgent, RequestID: meta.RequestID}
}
