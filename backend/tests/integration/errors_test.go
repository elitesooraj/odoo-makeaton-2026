package integration

import (
	"net/http"
	"testing"
)

// TestErrorEnvelopeContract locks the error response shape from
// docs/CONVENTIONS.md: { error: { code, message, details } } with the right
// status codes, so the frontend can branch on them reliably.
func TestErrorEnvelopeContract(t *testing.T) {
	reset(t)
	admin := adminClient(t)

	// Unauthenticated: no cookie -> 401 with UNAUTHENTICATED.
	c := newClient(t)
	env := c.mustStatus(http.MethodGet, "/api/v1/products", nil, http.StatusUnauthorized)
	if env.Error == nil || env.Error.Code != "UNAUTHENTICATED" {
		t.Fatalf("unauth error = %+v, want code UNAUTHENTICATED", env.Error)
	}
	if env.Error.Message == "" {
		t.Error("unauthenticated response has no message")
	}

	// Not found: a well-formed but unknown id -> 404 NOT_FOUND.
	env = admin.mustStatus(http.MethodGet, "/api/v1/products/"+nonexistentUUID, nil, http.StatusNotFound)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("not-found error = %+v, want code NOT_FOUND", env.Error)
	}

	// Unknown route -> 404 NOT_FOUND via NoRoute.
	env = admin.mustStatus(http.MethodGet, "/api/v1/definitely-not-a-route", nil, http.StatusNotFound)
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("unknown route error = %+v, want code NOT_FOUND", env.Error)
	}

	// Validation failure: missing required field -> 422 VALIDATION_FAILED with
	// a details map naming the field.
	env = admin.mustStatus(http.MethodPost, "/api/v1/products",
		map[string]any{}, http.StatusUnprocessableEntity)
	if env.Error == nil || env.Error.Code != "VALIDATION_FAILED" {
		t.Fatalf("validation error = %+v, want VALIDATION_FAILED", env.Error)
	}
	if _, ok := env.Error.Details["name"]; !ok {
		t.Errorf("validation details should name 'name', got %v", env.Error.Details)
	}

	// Malformed UUID in a path -> 400 BAD_REQUEST (rejected before lookup).
	admin.mustStatus(http.MethodGet, "/api/v1/products/not-a-uuid", nil, http.StatusBadRequest)

	// Conflict: editing a confirmed invoice is refused with 409 CONFLICT.
	// (set up a confirmed invoice first)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Conflict Corp")
	sub := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, sub.ID, fix, 1)
	activate(t, admin, sub.ID)
	var inv invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID, "subscription_id": sub.ID,
	}, &inv)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+inv.ID+"/confirm", map[string]any{}, &inv)
	env = admin.mustStatus(http.MethodPatch, "/api/v1/invoices/"+inv.ID,
		map[string]any{"notes": "too late"}, http.StatusConflict)
	if env.Error == nil || env.Error.Code != "CONFLICT" {
		t.Fatalf("conflict error = %+v, want code CONFLICT", env.Error)
	}

	// Duplicate billing run is rejected while one is already running. The
	// service guards this with a non-blocking mutex, so two concurrent calls
	// cannot both bill the same period.
	fix2 := createCatalog(t, admin, "WEEKLY")
	customerID2 := createCustomer(t, admin, "Concurrency Corp")
	sub2 := createSubscription(t, admin, customerID2, fix2.PlanID)
	addLine(t, admin, sub2.ID, fix2, 1)
	activate(t, admin, sub2.ID)

	type runOutcome struct {
		status int
		env    *envelope
	}
	outcomes := make(chan runOutcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			status, env := admin.do(http.MethodPost, "/api/v1/billing/run", map[string]any{})
			outcomes <- runOutcome{status: status, env: env}
		}()
	}

	var seen200, seen409 bool
	for i := 0; i < 2; i++ {
		o := <-outcomes
		if o.status == http.StatusOK {
			seen200 = true
		}
		if o.status == http.StatusConflict {
			seen409 = true
		}
	}
	if !seen200 || !seen409 {
		t.Fatalf("concurrent runs must yield one 200 and one 409; got 200=%v 409=%v",
			seen200, seen409)
	}
}

const nonexistentUUID = "019a0000-0000-7000-8000-000000000000"
