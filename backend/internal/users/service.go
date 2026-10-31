package users

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/authz"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/validate"
)

var errNotFound = errors.New("user not found")

type CreateInput struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type UpdateInput struct {
	FullName *string `json:"full_name"`
	Role     *string `json:"role"`
	Status   *string `json:"status"`
}

type Service struct {
	repo     *Repository
	authRepo *auth.Repository
	audit    *audit.Recorder
}

func NewService(repo *Repository, authRepo *auth.Repository, recorder *audit.Recorder) *Service {
	return &Service{repo: repo, authRepo: authRepo, audit: recorder}
}

// CreateStaff creates an internal user with a one-time temporary password.
// The password is returned in the response exactly once; it is never emailed.
func (s *Service) CreateStaff(ctx context.Context, actor *auth.Actor, meta audit.Meta, in CreateInput) (*auth.User, string, error) {
	email := validate.NormalizeEmail(in.Email)
	role := strings.ToUpper(strings.TrimSpace(in.Role))
	if role == "" {
		role = authz.RoleStaff
	}

	details := map[string]string{}
	if !validate.Email(email) {
		details["email"] = "enter a valid email address"
	}
	if role != authz.RoleStaff && role != authz.RoleAdmin {
		details["role"] = "role must be STAFF or ADMIN"
	}
	if len(details) > 0 {
		return nil, "", httpx.Validation(details)
	}

	if _, err := s.authRepo.FindByEmail(ctx, s.repo.pool, email); err == nil {
		return nil, "", httpx.Conflict("a user with this email already exists")
	} else if !errors.Is(err, auth.ErrNotFound) {
		return nil, "", err
	}

	tempPassword, err := GenerateTempPassword()
	if err != nil {
		return nil, "", httpx.Internal(err)
	}
	hash, err := auth.HashPassword(tempPassword)
	if err != nil {
		return nil, "", httpx.Internal(err)
	}

	now := time.Now().UTC()
	user := &auth.User{
		ID:                 id.New(),
		Email:              email,
		PasswordHash:       hash,
		FullName:           strings.TrimSpace(in.FullName),
		Role:               role,
		Status:             auth.StatusActive,
		EmailVerifiedAt:    &now,
		MustChangePassword: true,
	}

	var stored *auth.User
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.authRepo.CreateUser(ctx, tx, user); err != nil {
			return err
		}
		// Re-read inside the transaction so the audit snapshot carries the
		// database timestamps instead of Go zero values.
		created, err := s.authRepo.FindByID(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		stored = created
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID:    &actor.UserID,
			ActorRole:  actor.Role,
			Action:     "user.create",
			EntityType: "user",
			EntityID:   created.ID,
			After:      created,
			Meta:       meta,
		})
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, "", httpx.Conflict("a user with this email already exists")
		}
		return nil, "", err
	}
	return stored, tempPassword, nil
}

func (s *Service) Update(ctx context.Context, actor *auth.Actor, meta audit.Meta, userID uuid.UUID, in UpdateInput) (*auth.User, error) {
	current, err := s.authRepo.FindByID(ctx, s.repo.pool, userID)
	if err != nil {
		return nil, httpError(err)
	}
	before := *current
	details := map[string]string{}

	if in.FullName != nil {
		current.FullName = strings.TrimSpace(*in.FullName)
	}
	if in.Role != nil {
		role := strings.ToUpper(strings.TrimSpace(*in.Role))
		if role != authz.RoleStaff && role != authz.RoleAdmin {
			details["role"] = "role must be STAFF or ADMIN"
		}
		if current.ID == actor.UserID && role != current.Role {
			details["role"] = "you cannot change your own role"
		}
		current.Role = role
	}
	if in.Status != nil {
		status := strings.ToUpper(strings.TrimSpace(*in.Status))
		if status != auth.StatusActive && status != auth.StatusDisabled {
			details["status"] = "status must be ACTIVE or DISABLED"
		}
		if current.ID == actor.UserID && status == auth.StatusDisabled {
			details["status"] = "you cannot disable your own account"
		}
		current.Status = status
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	var stored *auth.User
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.Update(ctx, tx, current.ID, current.FullName, current.Role, current.Status); err != nil {
			return err
		}
		if current.Status == auth.StatusDisabled {
			if err := s.authRepo.RevokeAllSessions(ctx, tx, current.ID); err != nil {
				return err
			}
		}
		updated, err := s.authRepo.FindByID(ctx, tx, current.ID)
		if err != nil {
			return err
		}
		stored = updated
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID:    &actor.UserID,
			ActorRole:  actor.Role,
			Action:     "user.update",
			EntityType: "user",
			EntityID:   updated.ID,
			Before:     before,
			After:      updated,
			Meta:       meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

// SetPermissions replaces a staff member's GRANT/REVOKE overrides. Only staff
// can receive overrides; admin is implicit and customers have no back office.
func (s *Service) SetPermissions(ctx context.Context, actor *auth.Actor, meta audit.Meta, userID uuid.UUID, overrides []Override) ([]string, error) {
	target, err := s.authRepo.FindByID(ctx, s.repo.pool, userID)
	if err != nil {
		return nil, httpError(err)
	}
	if target.Role == authz.RoleCustomer {
		return nil, httpx.BadRequest("customer accounts do not use back-office permissions")
	}

	known := authz.Keys()
	seen := make(map[string]struct{}, len(overrides))
	details := map[string]string{}
	for _, o := range overrides {
		effect := strings.ToUpper(strings.TrimSpace(o.Effect))
		if _, ok := known[o.Permission]; !ok {
			details["overrides"] = "unknown permission: " + o.Permission
			continue
		}
		if effect != "GRANT" && effect != "REVOKE" {
			details["overrides"] = "effect must be GRANT or REVOKE"
			continue
		}
		if _, dup := seen[o.Permission]; dup {
			details["overrides"] = "duplicate permission: " + o.Permission
			continue
		}
		seen[o.Permission] = struct{}{}
	}
	if len(details) > 0 {
		return nil, httpx.Validation(details)
	}

	before, err := s.repo.ListOverrides(ctx, userID)
	if err != nil {
		return nil, err
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.ReplaceOverrides(ctx, tx, userID, actor.UserID, overrides); err != nil {
			return err
		}
		return s.audit.Record(ctx, tx, audit.Event{
			ActorID:    &actor.UserID,
			ActorRole:  actor.Role,
			Action:     "user.permissions.replace",
			EntityType: "user",
			EntityID:   userID,
			Before:     before,
			After:      overrides,
			Meta:       meta,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.effectivePermissionKeys(ctx, userID, target.Role)
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (*auth.User, []string, map[string]string, error) {
	user, err := s.authRepo.FindByID(ctx, s.repo.pool, userID)
	if err != nil {
		return nil, nil, nil, httpError(err)
	}
	permissions, err := s.effectivePermissionKeys(ctx, userID, user.Role)
	if err != nil {
		return nil, nil, nil, err
	}
	overrides, err := s.repo.ListOverrides(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	return user, permissions, overrides, nil
}

func (s *Service) List(ctx context.Context, f Filter) ([]auth.User, int, error) {
	return s.repo.List(ctx, f)
}

func (s *Service) effectivePermissionKeys(ctx context.Context, userID uuid.UUID, role string) ([]string, error) {
	set, err := s.authRepo.EffectivePermissions(ctx, s.repo.pool, userID, role)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	// Deterministic output for the UI.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys, nil
}

func httpError(err error) error {
	if errors.Is(err, auth.ErrNotFound) {
		return httpx.NotFound("user not found")
	}
	return err
}

func GenerateTempPassword() (string, error) {
	const (
		upper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		lower   = "abcdefghijkmnopqrstuvwxyz"
		digits  = "23456789"
		special = "!@#$%^&*"
	)
	all := upper + lower + digits + special
	classes := []string{upper, lower, digits, special}

	buf := make([]byte, 16)
	for i, class := range classes {
		pick, err := randomIndex(len(class))
		if err != nil {
			return "", err
		}
		buf[i] = class[pick]
	}
	for i := len(classes); i < len(buf); i++ {
		pick, err := randomIndex(len(all))
		if err != nil {
			return "", err
		}
		buf[i] = all[pick]
	}
	for i := len(buf) - 1; i > 0; i-- {
		j, err := randomIndex(i + 1)
		if err != nil {
			return "", err
		}
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf), nil
}

func randomIndex(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func isUniqueViolation(err error) bool {
	type pgError interface{ SQLState() string }
	var pgErr pgError
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}
