package integration

import (
	"net/http"
	"testing"
)

// TestReportsAggregates locks the dashboard numbers after a known fixture,
// using a MONTHLY plan so MRR is exact (no 52/12 rounding).
func TestReportsAggregates(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "MONTHLY")
	customerID := createCustomer(t, admin, "Reports Corp")

	// One ACTIVE subscription on a 10000-cent monthly plan, quantity 1.
	sub := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, sub.ID, fix, 1)
	activate(t, admin, sub.ID)

	// One overdue unpaid invoice (due in the past).
	var inv invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID, "subscription_id": sub.ID,
		"due_date": "2026-01-01",
	}, &inv)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+inv.ID+"/confirm", map[string]any{}, &inv)

	// One paid invoice on a second subscription.
	sub2 := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, sub2.ID, fix, 1)
	activate(t, admin, sub2.ID)
	var inv2 invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID, "subscription_id": sub2.ID,
	}, &inv2)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+inv2.ID+"/confirm", map[string]any{}, &inv2)
	admin.mustOK(http.MethodPost, "/api/v1/payments", map[string]any{
		"invoice_id": inv2.ID, "amount_cents": inv2.TotalCents, "method": "CARD",
	}, nil)

	var summary struct {
		ActiveSubscriptions   int   `json:"active_subscriptions"`
		MonthlyRecurringCents int64 `json:"mrr_cents"`
		OutstandingCents      int64 `json:"outstanding_cents"`
		OverdueCount          int   `json:"overdue_invoices"`
		OverdueCents          int64 `json:"overdue_cents"`
		CollectedThisMonth    int64 `json:"collected_this_month_cents"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/summary", nil, &summary)

	if summary.ActiveSubscriptions != 2 {
		t.Errorf("active subscriptions = %d, want 2", summary.ActiveSubscriptions)
	}
	// The plan price is 14900 cents/month; MRR must reflect it for both subs.
	if summary.MonthlyRecurringCents != 2*14900 {
		t.Errorf("mrr = %d, want %d (2 x 14900 monthly)", summary.MonthlyRecurringCents, 2*14900)
	}
	if summary.OutstandingCents != inv.TotalCents {
		t.Errorf("outstanding = %d, want %d (only the unpaid invoice)", summary.OutstandingCents, inv.TotalCents)
	}
	if summary.OverdueCount != 1 || summary.OverdueCents != inv.TotalCents {
		t.Errorf("overdue = %d/%d, want 1/%d", summary.OverdueCount, summary.OverdueCents, inv.TotalCents)
	}
	if summary.CollectedThisMonth != inv2.TotalCents {
		t.Errorf("collected this month = %d, want %d", summary.CollectedThisMonth, inv2.TotalCents)
	}

	// Status counts: 2 ACTIVE.
	var byStatus []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/subscriptions", nil, &byStatus)
	found := false
	for _, s := range byStatus {
		if s.Status == "ACTIVE" && s.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("ACTIVE count in by-status report is not 2: %+v", byStatus)
	}

	// Overdue endpoint lists the invoice, oldest first.
	var overdue []struct {
		ID       string `json:"id"`
		DaysLate int    `json:"days_late"`
		Balance  int64  `json:"balance_cents"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/overdue", nil, &overdue)
	if len(overdue) != 1 || overdue[0].ID != inv.ID {
		t.Fatalf("overdue list = %+v, want invoice %s", overdue, inv.ID)
	}
	if overdue[0].DaysLate <= 0 {
		t.Errorf("days_late = %d, want > 0 (due date is in the past)", overdue[0].DaysLate)
	}
	if overdue[0].Balance != inv.TotalCents {
		t.Errorf("overdue balance = %d, want %d", overdue[0].Balance, inv.TotalCents)
	}

	// Revenue series: 12 points, last one is this month and shows the payment.
	var revenue []struct {
		Month         string `json:"month"`
		PaidCents     int64  `json:"paid_cents"`
		InvoicedCents int64  `json:"invoiced_cents"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/revenue?months=12", nil, &revenue)
	if len(revenue) != 12 {
		t.Fatalf("revenue series length = %d, want 12", len(revenue))
	}
	last := revenue[len(revenue)-1]
	if last.Month != timeNowMonth() {
		t.Errorf("last revenue month = %s, want %s", last.Month, timeNowMonth())
	}
	if last.PaidCents != inv2.TotalCents {
		t.Errorf("paid this month = %d, want %d", last.PaidCents, inv2.TotalCents)
	}
}

func timeNowMonth() string { return todayPlusDays(0)[:7] }
