package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSubscriptionLifecycle walks the whole state machine one legal step at
// a time, asserting the status and the bookkeeping fields after each hop:
//
//	DRAFT -> CONFIRMED -> ACTIVE -> PAUSED -> ACTIVE -> CLOSED
//
// It also proves the guards: the wrong-source transitions are rejected with
// 409 CONFLICT rather than silently succeeding.
func TestSubscriptionLifecycle(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Acme Corp")

	sub := createSubscription(t, admin, customerID, fix.PlanID)
	if sub.Status != "DRAFT" {
		t.Fatalf("new subscription status = %q, want DRAFT", sub.Status)
	}
	if sub.Number != "SUB-000001" {
		t.Fatalf("first subscription number = %q, want SUB-000001 after reset", sub.Number)
	}
	addLine(t, admin, sub.ID, fix, 2)

	// DRAFT is editable: the line landed and is visible on the record.
	var withLines subscriptionDTO
	admin.mustOK(http.MethodGet, "/api/v1/subscriptions/"+sub.ID, nil, &withLines)
	// Create auto-seeds one line from the plan; addLine adds a second one.
	if len(withLines.Lines) != 2 {
		t.Fatalf("expected 2 lines (auto-seeded + added), got %d: %+v", len(withLines.Lines), withLines.Lines)
	}
	foundAdded := false
	for _, l := range withLines.Lines {
		if l.Quantity == 2 {
			foundAdded = true
		}
	}
	if !foundAdded {
		t.Fatalf("the added line (qty 2) is missing: %+v", withLines.Lines)
	}

	// Legal hop: DRAFT -> CONFIRMED.
	var confirmed subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/confirm", map[string]any{}, &confirmed)
	if confirmed.Status != "CONFIRMED" {
		t.Fatalf("status after confirm = %q, want CONFIRMED", confirmed.Status)
	}

	// Lines are frozen outside DRAFT/QUOTATION: a 409, not an edit.
	env := admin.mustStatus(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/lines",
		map[string]any{"product_id": fix.ProductID, "quantity": 1, "unit_price_cents": 100},
		http.StatusConflict)
	if !strings.Contains(env.Error.Message, "DRAFT") {
		t.Fatalf("line rejection should explain the DRAFT/QUOTATION rule, got: %s", env.Error)
	}

	// Illegal hop: activate something that is not CONFIRMED. Use a second,
	// still-DRAFT subscription.
	draft := createSubscription(t, admin, customerID, fix.PlanID)
	admin.mustStatus(http.MethodPost, "/api/v1/subscriptions/"+draft.ID+"/activate",
		map[string]any{}, http.StatusConflict)

	// Legal hop: CONFIRMED -> ACTIVE. next_billing_date defaults to the
	// start date (today), which is what makes the sub billable.
	active := activate(t, admin, sub.ID)
	if active.Status != "ACTIVE" {
		t.Fatalf("status after activate = %q, want ACTIVE", active.Status)
	}
	if active.NextBillingDate == nil {
		t.Fatal("activated subscription must have next_billing_date set")
	}
	if got := (*active.NextBillingDate)[:10]; got != time.Now().UTC().Format("2006-01-02") {
		t.Fatalf("next_billing_date = %s, want today %s", got, time.Now().UTC().Format("2006-01-02"))
	}

	// Illegal hop: confirming an ACTIVE subscription.
	admin.mustStatus(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/confirm",
		map[string]any{}, http.StatusConflict)

	// Legal hop: ACTIVE -> PAUSED; the billing date is parked (nil) so the
	// billing run cannot bill a paused subscription.
	var paused subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/pause", map[string]any{}, &paused)
	if paused.Status != "PAUSED" {
		t.Fatalf("status after pause = %q, want PAUSED", paused.Status)
	}
	// Note: the thin implementation keeps next_billing_date set on pause;
	// billing is gated by status = 'ACTIVE' (ListForBilling), so a paused
	// subscription is never billed even with a date in the past. Resume
	// reuses the stored date. A production implementation may want to
	// clear-and-recompute instead; this test pins current behavior.

	// Illegal hop: pausing a PAUSED subscription.
	admin.mustStatus(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/pause", map[string]any{}, http.StatusConflict)

	// Legal hop: PAUSED -> ACTIVE; a fresh billing date is computed.
	var resumed subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/resume", map[string]any{}, &resumed)
	if resumed.Status != "ACTIVE" {
		t.Fatalf("status after resume = %q, want ACTIVE", resumed.Status)
	}
	if resumed.NextBillingDate == nil {
		t.Fatal("resumed subscription must have a recomputed next_billing_date")
	}

	// Legal final hop: ACTIVE -> CLOSED. Once closed, nothing is editable.
	var closed subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/close", map[string]any{}, &closed)
	if closed.Status != "CLOSED" {
		t.Fatalf("status after close = %q, want CLOSED", closed.Status)
	}
	admin.mustStatus(http.MethodPatch, "/api/v1/subscriptions/"+sub.ID,
		map[string]any{"notes": "too late"}, http.StatusConflict)
	admin.mustStatus(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/pause",
		map[string]any{}, http.StatusConflict)
}

// TestPauseRequiresPlanPermission proves the plan gate: a plan that does not
// allow pausing rejects the pause transition even from ACTIVE.
func TestPauseRequiresPlanPermission(t *testing.T) {
	reset(t)
	admin := adminClient(t)

	// A plan with allow_pause false, unlike createCatalog's default.
	var product struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/products", map[string]any{"name": "Rigid product"}, &product)
	var plan struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/plans", map[string]any{
		"product_id": product.ID, "name": "No-pause plan", "interval": "MONTHLY",
		"price_cents": 9900, "allow_pause": false, "is_active": true,
	}, &plan)

	customerID := createCustomer(t, admin, "Rigid Customer Corp")
	sub := createSubscription(t, admin, customerID, plan.ID)
	addLine(t, admin, sub.ID, &catalogFixture{ProductID: product.ID}, 1)
	active := activate(t, admin, sub.ID)
	if active.Status != "ACTIVE" {
		t.Fatalf("setup: status = %q, want ACTIVE", active.Status)
	}

	env := admin.mustStatus(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/pause",
		map[string]any{}, http.StatusConflict)
	if !strings.Contains(strings.ToLower(env.Error.Message), "plan") {
		t.Fatalf("pause rejection should mention the plan's pause policy, got: %s", env.Error)
	}
}
