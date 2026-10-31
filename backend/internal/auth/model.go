package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/authz"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrUnauthenticated = errors.New("unauthenticated")
)

const (
	StatusActive   = "ACTIVE"
	StatusDisabled = "DISABLED"

	TokenPurposeReset  = "RESET"
	TokenPurposeVerify = "VERIFY"
)

// User is the persistence model. It is never serialized directly because it
// carries the password hash.
type User struct {
	ID                 uuid.UUID  `json:"id"`
	Email              string     `json:"email"`
	PasswordHash       string     `json:"-"`
	FullName           string     `json:"full_name"`
	Role               string     `json:"role"`
	Status             string     `json:"status"`
	CustomerID         *uuid.UUID `json:"customer_id"`
	EmailVerifiedAt    *time.Time `json:"email_verified_at"`
	MustChangePassword bool       `json:"must_change_password"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Actor is the authenticated principal attached to a request. Permissions are
// resolved once at authentication time and cached for the request.
type Actor struct {
	UserID             uuid.UUID
	SessionID          uuid.UUID
	Email              string
	FullName           string
	Role               string
	CustomerID         *uuid.UUID
	EmailVerified      bool
	MustChangePassword bool
	Permissions        map[string]bool
}

// Can reports whether the actor holds a permission. Admin is implicit; a
// customer has no back-office permissions at all.
func (a *Actor) Can(permission string) bool {
	if a == nil {
		return false
	}
	if a.Role == authz.RoleAdmin {
		return true
	}
	return a.Permissions[permission]
}

func (a *Actor) IsCustomer() bool {
	return a != nil && a.Role == authz.RoleCustomer
}
