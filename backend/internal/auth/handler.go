package auth

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
)

type Handler struct {
	svc        *Service
	cookieName string
	cookieTTL  time.Duration
	secure     bool
}

func NewHandler(svc *Service, cookieName string, cookieTTL time.Duration, secure bool) *Handler {
	return &Handler{svc: svc, cookieName: cookieName, cookieTTL: cookieTTL, secure: secure}
}

type userDTO struct {
	ID                 string  `json:"id"`
	Email              string  `json:"email"`
	FullName           string  `json:"full_name"`
	Role               string  `json:"role"`
	Status             string  `json:"status"`
	CustomerID         *string `json:"customer_id"`
	EmailVerified      bool    `json:"email_verified"`
	MustChangePassword bool    `json:"must_change_password"`
}

func newUserDTO(u *User) userDTO {
	dto := userDTO{
		ID:                 u.ID.String(),
		Email:              u.Email,
		FullName:           u.FullName,
		Role:               u.Role,
		Status:             u.Status,
		EmailVerified:      u.EmailVerifiedAt != nil,
		MustChangePassword: u.MustChangePassword,
	}
	if u.CustomerID != nil {
		id := u.CustomerID.String()
		dto.CustomerID = &id
	}
	return dto
}

func newActorDTO(a *Actor) userDTO {
	return userDTO{
		ID:                 a.UserID.String(),
		Email:              a.Email,
		FullName:           a.FullName,
		Role:               a.Role,
		Status:             StatusActive,
		EmailVerified:      a.EmailVerified,
		MustChangePassword: a.MustChangePassword,
		CustomerID:         uuidPtrString(a.CustomerID),
	}
}

func uuidPtrString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	CompanyName string `json:"company_name"`
	FullName    string `json:"full_name"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}

	user, token, err := h.svc.Register(c.Request.Context(), RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		CompanyName: req.CompanyName,
		FullName:    req.FullName,
	}, requestMeta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}

	h.setSessionCookie(c, token)
	httpx.Created(c, newUserDTO(user))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}

	user, token, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, requestMeta(c))
	if err != nil {
		httpx.Fail(c, err)
		return
	}

	h.setSessionCookie(c, token)
	httpx.OK(c, newUserDTO(user))
}

func (h *Handler) Logout(c *gin.Context) {
	raw, _ := c.Cookie(h.cookieName)
	if raw != "" {
		if err := h.svc.Logout(c.Request.Context(), raw); err != nil {
			httpx.Fail(c, err)
			return
		}
	}
	h.clearSessionCookie(c)
	httpx.NoContent(c)
}

type meResponse struct {
	User        userDTO  `json:"user"`
	Permissions []string `json:"permissions"`
}

func (h *Handler) Me(c *gin.Context) {
	actor, ok := CurrentActor(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthenticated("authentication required"))
		return
	}
	permissions := make([]string, 0, len(actor.Permissions))
	for key := range actor.Permissions {
		permissions = append(permissions, key)
	}
	sort.Strings(permissions)

	httpx.OK(c, meResponse{User: newActorDTO(actor), Permissions: permissions})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	actor, ok := CurrentActor(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthenticated("authentication required"))
		return
	}
	var req changePasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), actor, req.CurrentPassword, req.NewPassword); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.ForgotPassword(c.Request.Context(), req.Email, requestMeta(c)); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.Accepted(c, map[string]string{
		"message": "if that email is registered, a reset link has been sent",
	})
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		httpx.Fail(c, err)
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func (h *Handler) ResendVerification(c *gin.Context) {
	actor, ok := CurrentActor(c)
	if !ok {
		httpx.Fail(c, httpx.Unauthenticated("authentication required"))
		return
	}
	if err := h.svc.ResendVerification(c.Request.Context(), actor); err != nil {
		httpx.Fail(c, err)
		return
	}
	httpx.NoContent(c)
}

func requestMeta(c *gin.Context) RequestMeta {
	return RequestMeta{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
}

func (h *Handler) setSessionCookie(c *gin.Context, rawToken string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, rawToken, int(h.cookieTTL.Seconds()), "/", "", h.secure, true)
}

func (h *Handler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, "", -1, "/", "", h.secure, true)
}
