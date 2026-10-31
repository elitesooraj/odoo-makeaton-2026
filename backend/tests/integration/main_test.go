// Package integration runs the business flow tests against a real Postgres
// and the real router over an httptest.Server — the same wiring cmd/api
// uses, exercised the same way a browser or Postman would: JSON in, cookie
// session, JSON envelope out.
//
// The suite is DB-gated: without TEST_DATABASE_URL every test skips with an
// explanation, so plain `go test ./...` (and `make test`) stays fast and
// dependency-free. Run it with `make test-integration` or:
//
//	TEST_DATABASE_URL=postgres://brightloop:brightloop@localhost:55432/brightloop_test?sslmode=disable \
//	  go test ./tests/integration/ -v
//
// Every test starts from a clean slate: reset() truncates the data tables
// (the RBAC catalog from the migrations is kept) and restarts the document
// number sequences, so SUB-000001 / INV-000001 are deterministic per test.
package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/api"
	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/billing"
	"github.com/brightloop/subscription-platform/backend/internal/customers"
	"github.com/brightloop/subscription-platform/backend/internal/discounts"
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/payments"
	"github.com/brightloop/subscription-platform/backend/internal/plans"
	"github.com/brightloop/subscription-platform/backend/internal/platform/config"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
	"github.com/brightloop/subscription-platform/backend/internal/platform/mailer"
	"github.com/brightloop/subscription-platform/backend/internal/portal"
	"github.com/brightloop/subscription-platform/backend/internal/products"
	"github.com/brightloop/subscription-platform/backend/internal/quotations"
	"github.com/brightloop/subscription-platform/backend/internal/reports"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
	"github.com/brightloop/subscription-platform/backend/internal/taxes"
	"github.com/brightloop/subscription-platform/backend/internal/users"
	"github.com/gin-gonic/gin"
)

// Shared harness state, built once in TestMain.
var (
	pool   *pgxpool.Pool
	server *httptest.Server
)

// The bootstrap admin every test logs in as. It is recreated by reset() so
// tests can never be locked out, and its password satisfies the policy.
const (
	adminEmail    = "admin@brightloop.test"
	adminPassword = "Adm1n!Str0ngPass"
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println("== integration tests skipped: TEST_DATABASE_URL is not set.")
		fmt.Println("== run them with `make test-integration` or point TEST_DATABASE_URL at a disposable database.")
		os.Exit(m.Run())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var err error
	pool, err = db.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: connect: %v\n", err)
		os.Exit(1)
	}

	if err := applyMigrations(ctx, filepath.Join("..", "..", "migrations")); err != nil {
		fmt.Fprintf(os.Stderr, "integration: migrations: %v\n", err)
		os.Exit(1)
	}

	server = httptest.NewServer(buildRouter())
	defer server.Close()

	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// guard skips a test when the harness could not start (no DSN). Tests call
// it first so `go test ./...` without a database still passes.
func guard(t *testing.T) {
	t.Helper()
	if pool == nil || server == nil {
		t.Skip("TEST_DATABASE_URL not set; integration tests need a database")
	}
}

// reset gives the calling test a pristine database: all business and auth
// data truncated, document sequences restarted, the bootstrap admin back.
func reset(t *testing.T) {
	t.Helper()
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// One TRUNCATE with CASCADE: order does not matter, FKs are followed.
	// role_permissions and permissions are deliberately NOT truncated — the
	// RBAC catalog belongs to the migrations, not to test data.
	_, err := pool.Exec(ctx, `TRUNCATE
		audit_logs, billing_runs, discount_applications, payments,
		invoice_lines, invoices, subscription_lines, subscriptions,
		quotation_template_lines, quotation_templates, discounts, plans, taxes,
		product_variants, products, customers, password_tokens, sessions, users
		CASCADE`)
	if err != nil {
		t.Fatalf("reset: truncate: %v", err)
	}

	// Standalone sequences are not owned by the tables, so RESTART IDENTITY
	// above does not touch them. Reset by hand for deterministic numbers.
	for _, seq := range []string{"subscription_number_seq", "invoice_number_seq"} {
		if _, err := pool.Exec(ctx, "ALTER SEQUENCE "+seq+" RESTART WITH 1"); err != nil {
			t.Fatalf("reset: restart %s: %v", seq, err)
		}
	}

	ensureAdmin(ctx, t)
}

// ensureAdmin inserts the bootstrap admin directly, exactly like cmd/seed:
// verified, active, ADMIN role. It is idempotent.
func ensureAdmin(ctx context.Context, t *testing.T) {
	t.Helper()

	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		t.Fatalf("hash admin password: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO users
		(id, email, password_hash, full_name, role, status, email_verified_at)
		VALUES ($1, $2, $3, 'Test Admin', 'ADMIN', 'ACTIVE', now())
		ON CONFLICT DO NOTHING`,
		id.New(), adminEmail, hash)
	if err != nil {
		t.Fatalf("ensure admin: %v", err)
	}
}

// applyMigrations runs every *.up.sql in order. It tolerates "already
// exists" failures so the suite can run repeatedly against the same
// database; any other failure is fatal.
func applyMigrations(ctx context.Context, dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no migrations found under %s", dir)
	}
	sort.Strings(paths)

	for _, path := range paths {
		sql, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		err = db.WithTx(ctx, pool, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, string(sql))
			return err
		})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue // applied on a previous run
			}
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// ensureStaff inserts a STAFF account with no customer profile, so the portal
// tests can prove that back-office accounts are refused the customer surface.
func ensureStaff(ctx context.Context, t *testing.T) {
	t.Helper()
	hash, err := auth.HashPassword("St@ffPass1")
	if err != nil {
		t.Fatalf("hash staff password: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO users
		(id, email, password_hash, full_name, role, status, email_verified_at)
		VALUES ($1, $2, $3, 'Test Staff', 'STAFF', 'ACTIVE', now())
		ON CONFLICT DO NOTHING`,
		id.New(), "staff@brightloop.test", hash)
	if err != nil {
		t.Fatalf("ensure staff: %v", err)
	}
}

// constructors, same adapters, same middleware chain. If the production
// wiring drifts, these tests stop representing reality.
func buildRouter() *gin.Engine {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	slog.SetDefault(logger)

	recorder := audit.NewRecorder(pool)
	customersSvc := customers.NewService(customers.NewRepository(pool), recorder)
	authSvc := auth.NewService(auth.NewRepository(pool), customersSvc,
		mailer.New(mailer.SMTPConfig{}, logger), "http://"+serverHost(), 24*time.Hour)
	usersSvc := users.NewService(users.NewRepository(pool), auth.NewRepository(pool), recorder)

	productsSvc := products.NewService(products.NewRepository(pool))
	plansSvc := plans.NewService(plans.NewRepository(pool), productsSvc)
	taxesSvc := taxes.NewService(taxes.NewRepository(pool))
	discountsSvc := discounts.NewService(discounts.NewRepository(pool))

	subscriptionsSvc := subscriptions.NewService(pool, recorder, api.NewPlanAdapter(plansSvc))
	invoicesSvc := invoices.NewService(pool, recorder, subscriptionsSvc, discountsSvc, api.NewTaxAdapter(taxesSvc))
	paymentsSvc := payments.NewService(payments.NewRepository(pool), invoicesSvc.Repo(), recorder)
	quotationsSvc := quotations.NewService(pool, recorder, subscriptionsSvc, api.NewPlanAdapter(plansSvc))
	billingSvc := billing.NewService(pool, subscriptionsSvc, invoicesSvc, recorder)

	return api.NewRouter(config.Config{
		Env:                "development",
		SessionCookieName:  "bl_session",
		SessionTTL:         24 * time.Hour,
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}, pool, api.Deps{
		AuthService:          authSvc,
		AuthHandler:          auth.NewHandler(authSvc, "bl_session", 24*time.Hour, false),
		UsersHandler:         users.NewHandler(usersSvc),
		CustomersHandler:     customers.NewHandler(customersSvc),
		ProductsHandler:      products.NewHandler(productsSvc),
		PlansHandler:         plans.NewHandler(plansSvc),
		TaxesHandler:         taxes.NewHandler(taxesSvc),
		DiscountsHandler:     discounts.NewHandler(discountsSvc),
		SubscriptionsHandler: subscriptions.NewHandler(subscriptionsSvc),
		InvoicesHandler:      invoices.NewHandler(invoicesSvc),
		PaymentsHandler:      payments.NewHandler(paymentsSvc),
		QuotationsHandler:    quotations.NewHandler(quotationsSvc),
		BillingHandler:       billing.NewHandler(billingSvc),
		ReportsHandler:       reports.NewHandler(reports.NewService(pool)),
		PortalHandler:        portal.NewHandler(portal.NewService(pool, subscriptionsSvc, invoicesSvc, paymentsSvc)),
	})
}

func serverHost() string {
	if server == nil {
		return "localhost"
	}
	return strings.TrimPrefix(server.URL, "http://")
}
