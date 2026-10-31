package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/authz"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/mailer"
	"github.com/brightloop/subscription-platform/backend/internal/platform/ratelimit"
	"github.com/brightloop/subscription-platform/backend/internal/platform/validate"
)

// CustomerCreator is implemented by the customers service. Registration has to
// create the customer and the portal login atomically, so it receives the
// transaction rather than opening its own.
type CustomerCreator interface {
	CreateForRegistration(ctx context.Context, q db.Querier, name, email string) (uuid.UUID, error)
}

type RequestMeta struct {
	IP        string
	UserAgent string
}

type RegisterInput struct {
	Email       string
	Password    string
	CompanyName string
	FullName    string
}

type Service struct {
	repo         *Repository
	customers    CustomerCreator
	mailer       mailer.Mailer
	appBaseURL   string
	sessionTTL   time.Duration
	loginLimiter *ratelimit.Limiter
	resetLimiter *ratelimit.Limiter
}

func NewService(repo *Repository, customers CustomerCreator, m mailer.Mailer, appBaseURL string, sessionTTL time.Duration) *Service {
	return &Service{
		repo:         repo,
		customers:    customers,
		mailer:       m,
		appBaseURL:   appBaseURL,
		sessionTTL:   sessionTTL,
		loginLimiter: ratelimit.New(10, 15*time.Minute),
		resetLimiter: ratelimit.New(5, 15*time.Minute),
	}
}

// dummyPasswordHash makes a failed lookup cost roughly the same as a wrong
// password, so response timing does not reveal which emails exist.
var dummyPasswordHash string

func init() {
	h, err := HashPassword("Brightloop!dummy1")
	if err == nil {
		dummyPasswordHash = h
	}
}

func (s *Service) Register(ctx context.Context, in RegisterInput, meta RequestMeta) (*User, string, error) {
	email := validate.NormalizeEmail(in.Email)
	company := strings.TrimSpace(in.CompanyName)
	fullName := strings.TrimSpace(in.FullName)

	details := map[string]string{}
	if !validate.Email(email) {
		details["email"] = "enter a valid email address"
	}
	if company == "" {
		details["company_name"] = "company name is required"
	}
	for k, v := range validate.Password(in.Password) {
		details[k] = v
	}
	if len(details) > 0 {
		return nil, "", httpx.Validation(details)
	}

	if _, err := s.repo.FindByEmail(ctx, s.repo.pool, email); err == nil {
		return nil, "", httpx.Conflict("an account with this email already exists")
	} else if !errors.Is(err, ErrNotFound) {
		return nil, "", err
	}

	passwordHash, err := HashPassword(in.Password)
	if err != nil {
		return nil, "", httpx.Internal(err)
	}

	var (
		user     *User
		rawToken string
	)
	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		customerID, err := s.customers.CreateForRegistration(ctx, tx, company, email)
		if err != nil {
			return err
		}

		u := &User{
			ID:           id.New(),
			Email:        email,
			PasswordHash: passwordHash,
			FullName:     fullName,
			Role:         authz.RoleCustomer,
			Status:       StatusActive,
			CustomerID:   &customerID,
		}
		if err := s.repo.CreateUser(ctx, tx, u); err != nil {
			return err
		}

		session, token, err := s.newSession(u.ID, meta)
		if err != nil {
			return err
		}
		if err := s.repo.CreateSession(ctx, tx, session); err != nil {
			return err
		}

		user, rawToken = u, token
		return nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, "", httpx.Conflict("an account with this email already exists")
		}
		return nil, "", err
	}

	s.sendVerificationEmail(ctx, user)
	return user, rawToken, nil
}

func (s *Service) Login(ctx context.Context, email, password string, meta RequestMeta) (*User, string, error) {
	email = validate.NormalizeEmail(email)

	if !s.loginLimiter.Allow("login:" + meta.IP + ":" + email) {
		return nil, "", httpx.RateLimited("too many login attempts, try again in a few minutes")
	}

	user, err := s.repo.FindByEmail(ctx, s.repo.pool, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			VerifyPassword(dummyPasswordHash, password)
			return nil, "", httpx.Unauthenticated("invalid email or password")
		}
		return nil, "", err
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return nil, "", httpx.Unauthenticated("invalid email or password")
	}
	if user.Status != StatusActive {
		return nil, "", httpx.Forbidden("this account is disabled")
	}

	session, rawToken, err := s.newSession(user.ID, meta)
	if err != nil {
		return nil, "", err
	}
	if err := s.repo.CreateSession(ctx, s.repo.pool, session); err != nil {
		return nil, "", err
	}
	if err := s.repo.TouchLogin(ctx, s.repo.pool, user.ID); err != nil {
		slog.WarnContext(ctx, "could not update last_login_at", "user_id", user.ID, "error", err)
	}
	return user, rawToken, nil
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (*Actor, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrUnauthenticated
	}

	rec, err := s.repo.SessionByTokenHash(ctx, hashOpaqueToken(rawToken))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, err
	}
	if rec.RevokedAt != nil || time.Now().After(rec.ExpiresAt) || rec.User.Status != StatusActive {
		return nil, ErrUnauthenticated
	}

	permissions, err := s.repo.EffectivePermissions(ctx, s.repo.pool, rec.User.ID, rec.User.Role)
	if err != nil {
		return nil, err
	}
	s.repo.TouchSession(ctx, rec.ID)

	return &Actor{
		UserID:             rec.User.ID,
		SessionID:          rec.ID,
		Email:              rec.User.Email,
		FullName:           rec.User.FullName,
		Role:               rec.User.Role,
		CustomerID:         rec.User.CustomerID,
		EmailVerified:      rec.User.EmailVerifiedAt != nil,
		MustChangePassword: rec.User.MustChangePassword,
		Permissions:        permissions,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	return s.repo.RevokeSession(ctx, s.repo.pool, hashOpaqueToken(rawToken))
}

func (s *Service) ForgotPassword(ctx context.Context, email string, meta RequestMeta) error {
	if !s.resetLimiter.Allow("forgot:" + meta.IP) {
		return nil
	}
	email = validate.NormalizeEmail(email)
	if !validate.Email(email) {
		return nil
	}

	user, err := s.repo.FindByEmail(ctx, s.repo.pool, email)
	if err != nil {
		// Never reveal whether the address exists.
		return nil
	}

	raw, hash, err := newOpaqueToken()
	if err != nil {
		return err
	}
	if err := s.repo.CreatePasswordToken(ctx, s.repo.pool, user.ID, TokenPurposeReset, hash, time.Now().Add(30*time.Minute)); err != nil {
		return err
	}

	link := s.appBaseURL + "/reset-password?token=" + raw
	if err := s.mailer.Send(ctx, mailer.Message{
		To:      []string{user.Email},
		Subject: "Reset your Brightloop password",
		Text: "We received a request to reset your password.\n\n" +
			"Open this link within 30 minutes to choose a new password:\n\n" + link +
			"\n\nIf you did not request this, you can ignore this email.",
	}); err != nil {
		slog.ErrorContext(ctx, "password reset email failed", "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if details := validate.Password(newPassword); len(details) > 0 {
		return httpx.Validation(details)
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return httpx.Internal(err)
	}

	err = db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		userID, err := s.repo.ConsumePasswordToken(ctx, tx, TokenPurposeReset, hashOpaqueToken(rawToken))
		if err != nil {
			return err
		}
		if err := s.repo.UpdatePassword(ctx, tx, userID, hash); err != nil {
			return err
		}
		return s.repo.RevokeAllSessions(ctx, tx, userID)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.BadRequest("this reset link is invalid or has expired")
		}
		return err
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, actor *Actor, currentPassword, newPassword string) error {
	user, err := s.repo.FindByID(ctx, s.repo.pool, actor.UserID)
	if err != nil {
		return err
	}
	if !VerifyPassword(user.PasswordHash, currentPassword) {
		return httpx.Validation(map[string]string{"current_password": "current password is incorrect"})
	}
	if details := validate.Password(newPassword); len(details) > 0 {
		return httpx.Validation(details)
	}
	if currentPassword == newPassword {
		return httpx.Validation(map[string]string{"new_password": "new password must differ from the current password"})
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return httpx.Internal(err)
	}

	return db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := s.repo.UpdatePassword(ctx, tx, user.ID, hash); err != nil {
			return err
		}
		return s.repo.RevokeOtherSessions(ctx, tx, user.ID, actor.SessionID)
	})
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	err := db.WithTx(ctx, s.repo.pool, func(tx pgx.Tx) error {
		userID, err := s.repo.ConsumePasswordToken(ctx, tx, TokenPurposeVerify, hashOpaqueToken(rawToken))
		if err != nil {
			return err
		}
		return s.repo.SetEmailVerified(ctx, tx, userID)
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return httpx.BadRequest("this verification link is invalid or has expired")
		}
		return err
	}
	return nil
}

func (s *Service) ResendVerification(ctx context.Context, actor *Actor) error {
	user, err := s.repo.FindByID(ctx, s.repo.pool, actor.UserID)
	if err != nil {
		return err
	}
	if user.EmailVerifiedAt != nil {
		return nil
	}
	s.sendVerificationEmail(ctx, user)
	return nil
}

func (s *Service) sendVerificationEmail(ctx context.Context, user *User) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		slog.ErrorContext(ctx, "could not create verification token", "user_id", user.ID, "error", err)
		return
	}
	if err := s.repo.CreatePasswordToken(ctx, s.repo.pool, user.ID, TokenPurposeVerify, hash, time.Now().Add(48*time.Hour)); err != nil {
		slog.ErrorContext(ctx, "could not store verification token", "user_id", user.ID, "error", err)
		return
	}

	link := s.appBaseURL + "/verify-email?token=" + raw
	if err := s.mailer.Send(ctx, mailer.Message{
		To:      []string{user.Email},
		Subject: "Verify your Brightloop email",
		Text: "Welcome to Brightloop.\n\n" +
			"Confirm your email address by opening this link:\n\n" + link +
			"\n\nYou can keep using your account, but some actions will ask you to verify first.",
	}); err != nil {
		slog.ErrorContext(ctx, "verification email failed", "user_id", user.ID, "error", err)
	}
}

func (s *Service) newSession(userID uuid.UUID, meta RequestMeta) (Session, string, error) {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return Session{}, "", err
	}
	return Session{
		ID:        id.New(),
		UserID:    userID,
		TokenHash: hash,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	}, raw, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// compile-time check that the pool satisfies the repository's querier.
var _ db.Querier = (*pgxpool.Pool)(nil)
