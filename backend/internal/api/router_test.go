package api

import (
	"sort"
	"testing"
	"time"

	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/billing"
	"github.com/brightloop/subscription-platform/backend/internal/customers"
	"github.com/brightloop/subscription-platform/backend/internal/discounts"
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/payments"
	"github.com/brightloop/subscription-platform/backend/internal/plans"
	"github.com/brightloop/subscription-platform/backend/internal/platform/config"
	"github.com/brightloop/subscription-platform/backend/internal/portal"
	"github.com/brightloop/subscription-platform/backend/internal/products"
	"github.com/brightloop/subscription-platform/backend/internal/quotations"
	"github.com/brightloop/subscription-platform/backend/internal/reports"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
	"github.com/brightloop/subscription-platform/backend/internal/taxes"
	"github.com/brightloop/subscription-platform/backend/internal/users"
)

// TestRouteContract pins the entire HTTP surface: every route, with its
// method, exactly as documented in docs/API.md. It builds the real router
// (nil services are fine — handlers are never invoked) and compares the
// registered routes against the table below.
//
// Why this matters: a route group typo, a dropped middleware line or an
// accidental path rename silently changes the API contract the frontend
// builds against. This test fails on ANY difference in either direction —
// a route disappearing OR an unexpected route appearing.
//
// Permission annotations are comments, not assertions: the behavioral proof
// (401/403/404) lives in the integration suite.
func TestRouteContract(t *testing.T) {
	router := NewRouter(config.Config{
		Env:                "development",
		SessionCookieName:  "bl_session",
		SessionTTL:         time.Hour,
		CORSAllowedOrigins: []string{"http://localhost:5173"},
	}, nil, Deps{
		AuthHandler:          auth.NewHandler(nil, "bl_session", time.Hour, false),
		UsersHandler:         users.NewHandler(nil),
		CustomersHandler:     customers.NewHandler(nil),
		ProductsHandler:      products.NewHandler(nil),
		PlansHandler:         plans.NewHandler(nil),
		TaxesHandler:         taxes.NewHandler(nil),
		DiscountsHandler:     discounts.NewHandler(nil),
		SubscriptionsHandler: subscriptions.NewHandler(nil),
		InvoicesHandler:      invoices.NewHandler(nil),
		PaymentsHandler:      payments.NewHandler(nil),
		QuotationsHandler:    quotations.NewHandler(nil),
		BillingHandler:       billing.NewHandler(nil),
		ReportsHandler:       reports.NewHandler(nil),
		PortalHandler:        portal.NewHandler(nil),
	})

	got := map[string]bool{}
	for _, r := range router.Routes() {
		key := r.Method + " " + r.Path
		if got[key] {
			t.Errorf("route registered twice: %s", key)
		}
		got[key] = true
	}

	// The contract, mirroring docs/API.md section by section.
	want := []string{
		// Infrastructure.
		"GET /healthz",
		"GET /readyz",

		// Auth: public.
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"POST /api/v1/auth/forgot-password",
		"POST /api/v1/auth/reset-password",
		"POST /api/v1/auth/verify-email",

		// Auth: session required.
		"GET /api/v1/auth/me",
		"POST /api/v1/auth/change-password",
		"POST /api/v1/auth/resend-verification",

		// Users and permissions: users.* permissions.
		"GET /api/v1/permissions",
		"GET /api/v1/users",
		"POST /api/v1/users",
		"GET /api/v1/users/:id",
		"PATCH /api/v1/users/:id",
		"PUT /api/v1/users/:id/permissions",

		// Customers: customers.* permissions.
		"GET /api/v1/customers",
		"POST /api/v1/customers",
		"GET /api/v1/customers/:id",
		"PATCH /api/v1/customers/:id",
		"DELETE /api/v1/customers/:id",

		// Catalog: products.* permissions (variants included).
		"GET /api/v1/products",
		"POST /api/v1/products",
		"GET /api/v1/products/:id",
		"PATCH /api/v1/products/:id",
		"DELETE /api/v1/products/:id",
		"POST /api/v1/products/:id/variants",
		"PATCH /api/v1/products/:id/variants/:variant_id",
		"DELETE /api/v1/products/:id/variants/:variant_id",

		// Plans: plans.* permissions.
		"GET /api/v1/plans",
		"POST /api/v1/plans",
		"GET /api/v1/plans/:id",
		"PATCH /api/v1/plans/:id",
		"DELETE /api/v1/plans/:id",

		// Taxes: taxes.* permissions.
		"GET /api/v1/taxes",
		"POST /api/v1/taxes",
		"GET /api/v1/taxes/:id",
		"PATCH /api/v1/taxes/:id",
		"DELETE /api/v1/taxes/:id",

		// Discounts: discounts.* permissions.
		"GET /api/v1/discounts",
		"POST /api/v1/discounts",
		"GET /api/v1/discounts/:id",
		"PATCH /api/v1/discounts/:id",
		"DELETE /api/v1/discounts/:id",

		// Subscriptions: subscriptions.* permissions; lifecycle + lines.
		"GET /api/v1/subscriptions",
		"POST /api/v1/subscriptions",
		"GET /api/v1/subscriptions/:id",
		"PATCH /api/v1/subscriptions/:id",
		"POST /api/v1/subscriptions/:id/confirm",
		"POST /api/v1/subscriptions/:id/activate",
		"POST /api/v1/subscriptions/:id/pause",
		"POST /api/v1/subscriptions/:id/resume",
		"POST /api/v1/subscriptions/:id/close",
		"POST /api/v1/subscriptions/:id/lines",
		"PATCH /api/v1/subscriptions/:id/lines/:line_id",
		"DELETE /api/v1/subscriptions/:id/lines/:line_id",

		// Invoices: invoices.* permissions.
		"GET /api/v1/invoices",
		"POST /api/v1/invoices",
		"GET /api/v1/invoices/:id",
		"PATCH /api/v1/invoices/:id",
		"POST /api/v1/invoices/:id/confirm",
		"POST /api/v1/invoices/:id/cancel",
		"POST /api/v1/invoices/:id/send",

		// Payments: payments.* permissions.
		"GET /api/v1/payments",
		"POST /api/v1/payments",
		"GET /api/v1/payments/:id",

		// Quotation templates: subscriptions.* permissions.
		"GET /api/v1/quotations/templates",
		"POST /api/v1/quotations/templates",
		"GET /api/v1/quotations/templates/:id",
		"PATCH /api/v1/quotations/templates/:id",
		"DELETE /api/v1/quotations/templates/:id",
		"POST /api/v1/quotations/templates/:id/instantiate",

		// Billing: subscriptions.write.
		"POST /api/v1/billing/run",

		// Reports: reports.read.
		"GET /api/v1/reports/summary",
		"GET /api/v1/reports/revenue",
		"GET /api/v1/reports/subscriptions",
		"GET /api/v1/reports/overdue",

		// Customer portal: CUSTOMER role only, read-only.
		"GET /api/v1/portal/me",
		"GET /api/v1/portal/subscriptions",
		"GET /api/v1/portal/subscriptions/:id",
		"GET /api/v1/portal/invoices",
		"GET /api/v1/portal/invoices/:id",
		"GET /api/v1/portal/payments",
	}

	wantSet := map[string]bool{}
	for _, w := range want {
		if wantSet[w] {
			t.Errorf("duplicate in the expected table: %s", w)
		}
		wantSet[w] = true
	}

	var missing, unexpected []string
	for _, w := range want {
		if !got[w] {
			missing = append(missing, w)
		}
	}
	for g := range got {
		if !wantSet[g] {
			unexpected = append(unexpected, g)
		}
	}

	sort.Strings(missing)
	sort.Strings(unexpected)

	for _, m := range missing {
		t.Errorf("route missing from the router: %s", m)
	}
	for _, u := range unexpected {
		t.Errorf("route registered but not in the contract: %s", u)
	}

	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("route contract drift: %d missing, %d unexpected (listed above)", len(missing), len(unexpected))
	}

	t.Logf("route contract verified: %d routes", len(want))
}
