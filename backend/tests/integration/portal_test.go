package integration

import (
	"context"
	"net/http"
	"testing"
)

// TestPortalCustomerScoping is the security test of the suite: the portal is
// read-only and identity-scoped. A customer can only ever see their own
// subscriptions/invoices/payments, and foreign resources disappear into a 404
// (existence is not leaked) rather than a 403.
func TestPortalCustomerScoping(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")

	// Two self-service customers.
	clientA, userA := customerClient(t, "alice@example.com")
	clientB, userB := customerClient(t, "bob@example.com")
	if *userA.CustomerID == *userB.CustomerID {
		t.Fatal("register must create a distinct customer per user")
	}

	// Admin builds a subscription + invoice for A only.
	customerIDA := *userA.CustomerID
	sub := createSubscription(t, admin, customerIDA, fix.PlanID)
	addLine(t, admin, sub.ID, fix, 1)
	activate(t, admin, sub.ID)
	var inv invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerIDA, "subscription_id": sub.ID,
	}, &inv)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+inv.ID+"/confirm", map[string]any{}, &inv)

	// A sees its own data.
	var meA struct {
		Subscriptions  int `json:"subscriptions"`
		Invoices       int `json:"invoices"`
		UnpaidInvoices int `json:"unpaid_invoices"`
	}
	clientA.mustOK(http.MethodGet, "/api/v1/portal/me", nil, &meA)
	if meA.Subscriptions != 1 || meA.Invoices != 1 || meA.UnpaidInvoices != 1 {
		t.Errorf("A portal counts = %+v, want 1 sub, 1 invoice, 1 unpaid", meA)
	}

	var aSubs []subscriptionDTO
	clientA.mustOK(http.MethodGet, "/api/v1/portal/subscriptions", nil, &aSubs)
	if len(aSubs) != 1 || aSubs[0].ID != sub.ID {
		t.Fatalf("A's portal subscription list wrong: %+v", aSubs)
	}
	var aSub subscriptionDTO
	clientA.mustOK(http.MethodGet, "/api/v1/portal/subscriptions/"+sub.ID, nil, &aSub)
	if aSub.ID != sub.ID {
		t.Errorf("A cannot fetch its own subscription")
	}

	var aInvs []invoiceDTO
	clientA.mustOK(http.MethodGet, "/api/v1/portal/invoices", nil, &aInvs)
	if len(aInvs) != 1 {
		t.Fatalf("A's portal invoice list = %d, want 1", len(aInvs))
	}
	var aInv invoiceDTO
	clientA.mustOK(http.MethodGet, "/api/v1/portal/invoices/"+inv.ID, nil, &aInv)
	if aInv.ID != inv.ID {
		t.Errorf("A cannot fetch its own invoice")
	}

	// B sees nothing of A's data, and a foreign id is a 404, not a 403.
	var bMe struct {
		Subscriptions  int `json:"subscriptions"`
		Invoices       int `json:"invoices"`
		UnpaidInvoices int `json:"unpaid_invoices"`
	}
	clientB.mustOK(http.MethodGet, "/api/v1/portal/me", nil, &bMe)
	if bMe.Subscriptions != 0 || bMe.Invoices != 0 || bMe.UnpaidInvoices != 0 {
		t.Errorf("B's portal must be empty, got %+v", bMe)
	}
	var bSubs []subscriptionDTO
	clientB.mustOK(http.MethodGet, "/api/v1/portal/subscriptions", nil, &bSubs)
	if len(bSubs) != 0 {
		t.Errorf("B sees %d of A's subscriptions", len(bSubs))
	}
	clientB.mustStatus(http.MethodGet, "/api/v1/portal/invoices/"+inv.ID, nil, http.StatusNotFound)
	clientB.mustStatus(http.MethodGet, "/api/v1/portal/subscriptions/"+sub.ID, nil, http.StatusNotFound)

	// The portal is read-only: no POST/PUT/PATCH/DELETE is even routed.
	clientA.mustStatus(http.MethodPost, "/api/v1/portal/subscriptions", map[string]any{}, http.StatusNotFound)
	clientA.mustStatus(http.MethodDelete, "/api/v1/portal/subscriptions/"+sub.ID, nil, http.StatusNotFound)

	// A CUSTOMER hitting a back-office route gets 403, not 404.
	clientA.mustStatus(http.MethodGet, "/api/v1/subscriptions", nil, http.StatusForbidden)
	clientA.mustStatus(http.MethodPost, "/api/v1/products", map[string]any{"name": "x"}, http.StatusForbidden)

	// A STAFF account (no customer profile) is refused the portal.
	staffClient := staffClient(t)
	staffClient.mustStatus(http.MethodGet, "/api/v1/portal/me", nil, http.StatusForbidden)
}

// staffClient logs in as a STAFF account inserted directly (no customer
// profile, so the portal must refuse it).
func staffClient(t *testing.T) *client {
	t.Helper()
	ensureStaff(context.Background(), t)
	c := newClient(t)
	c.mustOK(http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email": "staff@brightloop.test", "password": "St@ffPass1",
	}, nil)
	return c
}
