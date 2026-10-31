package users

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

type Filter struct {
	Search string
	Role   string
	Status string
	Page   pagination.Page
}

type Override struct {
	Permission string
	Effect     string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const userColumns = `id, email, password_hash, full_name, role, status, customer_id,
	email_verified_at, must_change_password, last_login_at, created_at, updated_at`

func (r *Repository) List(ctx context.Context, f Filter) ([]auth.User, int, error) {
	const where = `
		WHERE ($1 = '' OR lower(email) LIKE '%' || lower($1) || '%'
		       OR lower(full_name) LIKE '%' || lower($1) || '%')
		  AND (($2 = '' AND role <> 'CUSTOMER') OR role = $2)
		  AND ($3 = '' OR status = $3)`

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`+where, f.Search, f.Role, f.Status).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, `SELECT `+userColumns+` FROM users`+where+`
		ORDER BY created_at DESC, id
		LIMIT $4 OFFSET $5`, f.Search, f.Role, f.Status, f.Page.PerPage, f.Page.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]auth.User, 0)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, *u)
	}
	return result, total, rows.Err()
}

func (r *Repository) Update(ctx context.Context, q db.Querier, userID uuid.UUID, fullName, role, status string) error {
	_, err := q.Exec(ctx, `UPDATE users
		SET full_name = $2, role = $3, status = $4
		WHERE id = $1`, userID, fullName, role, status)
	return err
}

func (r *Repository) ListOverrides(ctx context.Context, userID uuid.UUID) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT permission_key, effect
		FROM user_permissions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := make(map[string]string)
	for rows.Next() {
		var key, effect string
		if err := rows.Scan(&key, &effect); err != nil {
			return nil, err
		}
		overrides[key] = effect
	}
	return overrides, rows.Err()
}

func (r *Repository) ReplaceOverrides(ctx context.Context, q db.Querier, userID, grantedBy uuid.UUID, overrides []Override) error {
	if _, err := q.Exec(ctx, `DELETE FROM user_permissions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, o := range overrides {
		if _, err := q.Exec(ctx, `INSERT INTO user_permissions
			(user_id, permission_key, effect, granted_by)
			VALUES ($1, $2, $3, $4)`,
			userID, o.Permission, o.Effect, grantedBy); err != nil {
			return err
		}
	}
	return nil
}

func scanUser(row pgx.Row) (*auth.User, error) {
	var (
		u         auth.User
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
			return nil, auth.ErrNotFound
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

// PageFilter builds a repository filter from query parameters.
func PageFilter(search, role, status string, page pagination.Page) Filter {
	return Filter{
		Search: strings.TrimSpace(search),
		Role:   strings.ToUpper(strings.TrimSpace(role)),
		Status: strings.ToUpper(strings.TrimSpace(status)),
		Page:   page,
	}
}
