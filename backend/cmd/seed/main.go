// Command seed creates the bootstrap admin and, optionally, a finance staff
// user with invoices.write and payments.write. It is idempotent: running it
// again never overwrites an existing password.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/authz"
	"github.com/brightloop/subscription-platform/backend/internal/platform/config"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/validate"
	"github.com/brightloop/subscription-platform/backend/internal/users"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	adminID, created, err := ensureUser(ctx, pool, cfg.AdminEmail, cfg.AdminPassword, "Brightloop Admin", authz.RoleAdmin)
	if err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	if created {
		logger.Info("admin created", "email", strings.ToLower(cfg.AdminEmail), "id", adminID)
	} else {
		logger.Info("admin already exists, left untouched", "email", strings.ToLower(cfg.AdminEmail))
	}

	if cfg.FinanceEmail == "" {
		logger.Info("FINANCE_EMAIL not set, skipping finance staff seed")
		return nil
	}

	financeID, created, err := ensureUser(ctx, pool, cfg.FinanceEmail, cfg.FinancePassword, "Finance Staff", authz.RoleStaff)
	if err != nil {
		return fmt.Errorf("finance staff: %w", err)
	}
	for _, permission := range []string{authz.InvoicesWrite, authz.PaymentsWrite} {
		if _, err := pool.Exec(ctx, `INSERT INTO user_permissions
			(user_id, permission_key, effect, granted_by)
			VALUES ($1, $2, 'GRANT', $3)
			ON CONFLICT (user_id, permission_key)
			DO UPDATE SET effect = 'GRANT'`, financeID, permission, adminID); err != nil {
			return fmt.Errorf("grant %s: %w", permission, err)
		}
	}
	logger.Info("finance staff ready",
		"email", strings.ToLower(cfg.FinanceEmail),
		"created", created,
		"grants", []string{authz.InvoicesWrite, authz.PaymentsWrite},
	)
	return nil
}

func ensureUser(ctx context.Context, pool *pgxpool.Pool, email, password, fullName, role string) (uuid.UUID, bool, error) {
	email = validate.NormalizeEmail(email)
	if !validate.Email(email) {
		return uuid.Nil, false, fmt.Errorf("invalid email %q", email)
	}

	var existing uuid.UUID
	err := pool.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&existing)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, err
	}

	generated := false
	if password == "" {
		generatedPassword, genErr := users.GenerateTempPassword()
		if genErr != nil {
			return uuid.Nil, false, genErr
		}
		password = generatedPassword
		generated = true
		fmt.Printf("\n  generated password for %s: %s\n  change it after first login\n\n", email, password)
	}
	if details := validate.Password(password); len(details) > 0 {
		return uuid.Nil, false, fmt.Errorf("password does not satisfy the policy: %v", details)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, false, err
	}

	userID := id.New()
	_, err = pool.Exec(ctx, `INSERT INTO users
		(id, email, password_hash, full_name, role, status, email_verified_at, must_change_password)
		VALUES ($1, $2, $3, $4, $5, 'ACTIVE', now(), $6)`,
		userID, email, hash, fullName, role, generated)
	if err != nil {
		return uuid.Nil, false, err
	}
	return userID, true, nil
}
