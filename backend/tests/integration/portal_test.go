package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
)

// TestPortalCustomerScoping proves the CUSTOMER role is identity-scoped and
// read-only: a customer only ever sees their own subscriptions, invoices and
// payments; foreign resources come back as 404 (existence not leaked); and
// the back-office routes reject them with 403.
func TestPortalCustomerScoping(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")

	// Two independent customers, registered through the public flow.
	custA, userA := customerClient(t, "alpha@example.com")
	custB, _ := customerClient(t, "beta@example.com")
	if userA.CustomerID == nil {
		t.Fatal("registration did not link a customer")
	}
	customerAID := *userA.CustomerID

	// Admin builds a subscription + confirmed, unpaid invoice for customer A.
	sub := createSubscription(t, admin, customerAID, fix.PlanID)
	addLine(t, admin, sub.ID, fix, 1)
	activate(t, admin, sub.ID)
	var invoice invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerAID, "subscription_id": sub.ID,
	}, &invoice)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+invoice.ID+"/confirm", map[string]any{}, nil)

	// Customer A's landing page reflects exactly one subscription and one
	// unpaid invoice.
	var me struct {
		CustomerID     string `json:"customer_id"`
		Subscriptions  int    `json:"subscriptions"`
		ActiveStatus   int    `json:"active_status"`
		Invoices       int    `json:"invoices"`
		UnpaidInvoices int    `json:"unpaid_invoices"`
	}
	custA.mustOK(http.MethodGet, "/api/v1/portal/me", nil, &me)
	if me.CustomerID != customerAID {
		t.Errorf("portal me customer_id = %s, want %s", me.CustomerID, customerAID)
	}
	if me.Subscriptions != 1 || me.ActiveStatus != 1 || me.Invoices != 1 || me.UnpaidInvoices != 1 {
		t.Errorf("portal me counts = %+v, want 1/1/1/1", me)
	}

	// A sees their own subscription and invoice.
	var subs []subscriptionDTO
	custA.mustOK(http.MethodGet, "/api/v1/portal/subscriptions", nil, &subs)
	if len(subs) != 1 || subs[0].ID != sub.ID {
		t.Fatalf("customer A subscriptions = %+v", subs)
	}
	var own invoiceDTO
	custA.mustOK(http.MethodGet, "/api/v1/portal/invoices/"+invoice.ID, nil, &own)
	if own.ID != invoice.ID {
		t.Fatalf("customer A cannot read own invoice")
	}

	// Customer B sees nothing of A's data: empty lists and 404 on the
	// foreign invoice (existence is not leaked).
	var bSubs []subscriptionDTO
	custB.mustOK(http.MethodGet, "/api/v1/portal/subscriptions", nil, &bSubs)
	if len(bSubs) != 0 {
		t.Errorf("customer B sees %d subscriptions, want 0", len(bSubs))
	}
	env := custB.mustStatus(http.MethodGet, "/api/v1/portal/invoices/"+invoice.ID, nil, http.StatusNotFound)
	if env.Error.Code != "NOT_FOUND" {
		t.Errorf("foreign invoice should be 404 NOT_FOUND, got %s", env.Error)
	}

	// The portal is read-only: there are no write verbs on it, and the
	// back-office routes reject a CUSTOMER with 403.
	custA.mustStatus(http.MethodPost, "/api/v1/products",
		map[string]any{"name": "x"}, http.StatusForbidden)
	custA.mustStatus(http.MethodGet, "/api/v1/subscriptions", nil, http.StatusForbidden)

	// A STAFF account (no customer profile) is also rejected by the portal.
	ensureStaff(t)
	staff := newClient(t)
	staff.mustOK(http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "staff@brightloop.test", "password": "Staff!Pass1",
	}, nil)
	staff.mustStatus(http.MethodGet, "/api/v1/portal/me", nil, http.StatusForbidden)
}

// ensureStaff inserts a STAFF account the same way the seed job does, so
// portal-rejection tests have a non-customer identity to log in as.
func ensureStaff(t *testing.T) {
	t.Helper()
	hash, err := auth.HashPassword("Staff!Pass1")
	if err != nil {
		t.Fatalf("hash staff password: %v", err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO users
		(id, email, password_hash, full_name, role, status, email_verified_at)
		VALUES ($1, $2, $3, 'Test Staff', 'STAFF', 'ACTIVE', now())
		ON CONFLICT (lower(email)) DO NOTHING`,
		id.New(), "staff@brightloop.test", hash)
	if err != nil {
		t.Fatalf("ensure staff: %v", err)
	}
}
