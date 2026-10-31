package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// billingRunDTO mirrors the POST /billing/run summary.
type billingRunDTO struct {
	AsOf     time.Time `json:"as_of"`
	Billed   int       `json:"billed"`
	Skipped  int       `json:"skipped"`
	Closed   int       `json:"closed"`
	Failed   int       `json:"failed"`
	Invoices []string  `json:"invoices"`
	Results  []struct {
		SubscriptionID string  `json:"subscription_id"`
		PeriodStart    string  `json:"period_start"`
		PeriodEnd      string  `json:"period_end"`
		InvoiceID      *string `json:"invoice_id"`
		Status         string  `json:"status"`
		Detail         string  `json:"detail"`
	} `json:"results"`
}

// TestBillingRunBillsDueSubscriptionsAndIsIdempotent locks the billing
// engine's three promises:
//
//  1. a run bills every ACTIVE subscription whose next_billing_date is due,
//     snapshots a CONFIRMED invoice for the period and advances the date by
//     the plan interval (WEEKLY here: +7 days, no month-end quirks);
//  2. re-running is a no-op: the run reports SKIPPED with a reason, never a
//     duplicate invoice (billing_runs + invoices unique keys);
//  3. a subscription already invoiced ad hoc for the period is SKIPPED too.
func TestBillingRunBillsDueSubscriptionsAndIsIdempotent(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Billable Corp")

	// Sub A: clean, due today (activate sets next_billing_date = today).
	subA := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, subA.ID, fix, 2)
	activate(t, admin, subA.ID)

	var first billingRunDTO
	admin.mustOK(http.MethodPost, "/api/v1/billing/run", map[string]any{}, &first)

	if first.Failed != 0 {
		t.Fatalf("first run reported failures: %+v", first.Results)
	}
	if first.Billed != 1 || len(first.Invoices) != 1 {
		t.Fatalf("first run billed = %d, want 1 (results: %+v)", first.Billed, first.Results)
	}
	res := first.Results[0]
	if res.SubscriptionID != subA.ID || res.Status != "CREATED" || res.InvoiceID == nil {
		t.Fatalf("unexpected result for sub A: %+v", res)
	}
	if res.PeriodStart[:10] != todayUTC() {
		t.Errorf("period_start = %s, want today %s", res.PeriodStart, todayUTC())
	}
	if res.PeriodEnd[:10] != todayPlusDays(7) {
		t.Errorf("period_end = %s, want %s (WEEKLY interval)", res.PeriodEnd, todayPlusDays(7))
	}

	// The invoice is CONFIRMED, linked, and carries the sub's line money.
	var inv invoiceDTO
	admin.mustOK(http.MethodGet, "/api/v1/invoices/"+*res.InvoiceID, nil, &inv)
	if inv.Status != "CONFIRMED" {
		t.Errorf("billing invoice status = %q, want CONFIRMED", inv.Status)
	}
	if inv.SubscriptionID == nil || *inv.SubscriptionID != subA.ID {
		t.Errorf("billing invoice not linked to the subscription")
	}
	if inv.TotalCents != 52746 {
		t.Errorf("billing invoice total = %d, want 52746 (3 x 14900 + 18%% tax)", inv.TotalCents)
	}

	// next_billing_date advanced by one WEEKLY interval.
	var afterRun subscriptionDTO
	admin.mustOK(http.MethodGet, "/api/v1/subscriptions/"+subA.ID, nil, &afterRun)
	if afterRun.NextBillingDate == nil || (*afterRun.NextBillingDate)[:10] != todayPlusDays(7) {
		t.Errorf("next_billing_date after run = %v, want %s", afterRun.NextBillingDate, todayPlusDays(7))
	}

	// Sub B: due today AND already invoiced ad hoc for today's period.
	subB := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, subB.ID, fix, 1)
	activate(t, admin, subB.ID)
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID, "subscription_id": subB.ID,
	}, nil)

	// Second run: A is no longer due (date advanced), B is skipped because
	// a confirmed invoice already exists for the period.
	var second billingRunDTO
	admin.mustOK(http.MethodPost, "/api/v1/billing/run", map[string]any{}, &second)
	if second.Failed != 0 {
		t.Fatalf("second run reported failures: %+v", second.Results)
	}
	if second.Billed != 0 {
		t.Errorf("second run billed = %d, want 0", second.Billed)
	}
	if second.Skipped != 1 {
		t.Fatalf("second run skipped = %d, want 1 (results: %+v)", second.Skipped, second.Results)
	}
	if !strings.Contains(second.Results[0].Detail, "already") {
		t.Errorf("skip reason should explain the duplicate guard, got: %q", second.Results[0].Detail)
	}

	// Exactly one billing invoice exists per subscription per period: the
	// second run must not have created anything new.
	var list []invoiceDTO
	admin.mustOK(http.MethodGet, "/api/v1/invoices?subscription_id="+subB.ID, nil, &list)
	if len(list) != 1 {
		t.Errorf("subscription B has %d invoices after two runs, want 1", len(list))
	}
}

// TestBillingRunIgnoresNotDueAndPaused proves the selection filter: a
// subscription billed to the next period is left alone, and a PAUSED
// subscription (no next_billing_date) is never picked up.
func TestBillingRunIgnoresNotDueAndPaused(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Quiet Corp")

	// One paused subscription: activated, then paused.
	sub := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, sub.ID, fix, 1)
	activate(t, admin, sub.ID)
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+sub.ID+"/pause", map[string]any{}, nil)

	var run billingRunDTO
	admin.mustOK(http.MethodPost, "/api/v1/billing/run", map[string]any{}, &run)
	if run.Billed != 0 || run.Failed != 0 || run.Skipped != 0 {
		t.Fatalf("nothing is due; expected an empty run, got %+v", run.Results)
	}
}

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }
func todayPlusDays(d int) string {
	return time.Now().UTC().AddDate(0, 0, d).Format("2006-01-02")
}
