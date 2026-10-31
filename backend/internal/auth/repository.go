package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/authz"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const userColumns = `id, email, password_hash, full_name, role, status, customer_id,
	email_verified_at, must_change_password, last_login_at, created_at, updated_at`

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	IP        string
	UserAgent string
	ExpiresAt time.Time
}

type SessionRecord struct {
	ID        uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
	User      *User
}

func (r *Repository) FindByEmail(ctx context.Context, q db.Querier, email string) (*User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email))
}

func (r *Repository) FindByID(ctx context.Context, q db.Querier, id uuid.UUID) (*User, error) {
	return scanUser(q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (r *Repository) CreateUser(ctx context.Context, q db.Querier, u *User) error {
	_, err := q.Exec(ctx, `INSERT INTO users
		(id, email, password_hash, full_name, role, status, customer_id,
		 email_verified_at, must_change_password, last_login_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.Role, u.Status,
		uuidArg(u.CustomerID), timeArg(u.EmailVerifiedAt), u.MustChangePassword, timeArg(u.LastLoginAt),
	)
	return err
}

func (r *Repository) CreateSession(ctx context.Context, q db.Querier, s Session) error {
	_, err := q.Exec(ctx, `INSERT INTO sessions
		(id, user_id, token_hash, ip, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		s.ID, s.UserID, s.TokenHash, emptyToNil(s.IP), emptyToNil(s.UserAgent), s.ExpiresAt,
	)
	return err
}

func (r *Repository) SessionByTokenHash(ctx context.Context, tokenHash []byte) (*SessionRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT s.id, s.expires_at, s.revoked_at,
		       u.id, u.email, u.full_name, u.role, u.status, u.customer_id,
		       u.email_verified_at, u.must_change_password
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1`, tokenHash)

	var (
		rec      SessionRecord
		user     User
		revoked  pgtype.Timestamptz
		customer pgtype.UUID
		verified pgtype.Timestamptz
	)
	err := row.Scan(
		&rec.ID, &rec.ExpiresAt, &revoked,
		&user.ID, &user.Email, &user.FullName, &user.Role, &user.Status, &customer,
		&verified, &user.MustChangePassword,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if revoked.Valid {
		t := revoked.Time
		rec.RevokedAt = &t
	}
	if customer.Valid {
		id := uuid.UUID(customer.Bytes)
		user.CustomerID = &id
	}
	if verified.Valid {
		t := verified.Time
		user.EmailVerifiedAt = &t
	}
	rec.User = &user
	return &rec, nil
}

func (r *Repository) TouchSession(ctx context.Context, sessionID uuid.UUID) {
	_, _ = r.pool.Exec(ctx, `UPDATE sessions SET last_seen_at = now()
		WHERE id = $1 AND last_seen_at < now() - interval '5 minutes'`, sessionID)
}

func (r *Repository) RevokeSession(ctx context.Context, q db.Querier, tokenHash []byte) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (r *Repository) RevokeAllSessions(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (r *Repository) RevokeOtherSessions(ctx context.Context, q db.Querier, userID, keepSessionID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE sessions SET revoked_at = now()
		WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL`, userID, keepSessionID)
	return err
}

func (r *Repository) UpdatePassword(ctx context.Context, q db.Querier, userID uuid.UUID, passwordHash string) error {
	_, err := q.Exec(ctx, `UPDATE users
		SET password_hash = $2, must_change_password = false
		WHERE id = $1`, userID, passwordHash)
	return err
}

func (r *Repository) TouchLogin(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID)
	return err
}

func (r *Repository) SetEmailVerified(ctx context.Context, q db.Querier, userID uuid.UUID) error {
	_, err := q.Exec(ctx, `UPDATE users SET email_verified_at = now()
		WHERE id = $1 AND email_verified_at IS NULL`, userID)
	return err
}

func (r *Repository) CreatePasswordToken(ctx context.Context, q db.Querier, userID uuid.UUID, purpose string, tokenHash []byte, expiresAt time.Time) error {
	_, err := q.Exec(ctx, `
		WITH invalidated AS (
			UPDATE password_tokens SET used_at = now()
			WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
		)
		INSERT INTO password_tokens (id, user_id, purpose, token_hash, expires_at)
		VALUES ($3, $1, $2, $4, $5)`,
		userID, purpose, id.New(), tokenHash, expiresAt,
	)
	return err
}

// ConsumePasswordToken atomically marks a token used and returns its owner.
func (r *Repository) ConsumePasswordToken(ctx context.Context, q db.Querier, purpose string, tokenHash []byte) (uuid.UUID, error) {
	var userID uuid.UUID
	err := q.QueryRow(ctx, `
		UPDATE password_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = $2
		  AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash, purpose).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return userID, nil
}

func (r *Repository) EffectivePermissions(ctx context.Context, q db.Querier, userID uuid.UUID, role string) (map[string]bool, error) {
	permissions := make(map[string]bool)

	// Admin sees the whole catalog, including permissions added by later
	// migrations that have not been mirrored into role_permissions.
	if role == authz.RoleAdmin {
		rows, err := q.Query(ctx, `SELECT key FROM permissions`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				return nil, err
			}
			permissions[key] = true
		}
		return permissions, rows.Err()
	}

	rows, err := q.Query(ctx, `
		SELECT permission_key FROM (
			SELECT rp.permission_key
			FROM role_permissions rp
			WHERE rp.role = $2
			UNION
			SELECT up.permission_key
			FROM user_permissions up
			WHERE up.user_id = $1 AND up.effect = 'GRANT'
			EXCEPT
			SELECT up.permission_key
			FROM user_permissions up
			WHERE up.user_id = $1 AND up.effect = 'REVOKE'
		) effective`, userID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		permissions[key] = true
	}
	return permissions, rows.Err()
}

func scanUser(row pgx.Row) (*User, error) {
	var (
		u         User
		customer  pgtype.UUID
		verified  pgtype.Timestamptz
		lastLogin pgtype.Timestamptz
	)
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Role, &u.Status,
		&customer, &verified, &u.MustChangePassword, &lastLogin, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if customer.Valid {
		id := uuid.UUID(customer.Bytes)
		u.CustomerID = &id
	}
	if verified.Valid {
		t := verified.Time
		u.EmailVerifiedAt = &t
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		u.LastLoginAt = &t
	}
	return &u, nil
}

func uuidArg(v *uuid.UUID) any {
	if v == nil {
		return nil
	}
	return *v
}

func timeArg(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
