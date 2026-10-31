package integration

import (
	"net/http"
	"testing"
	"time"
)

// TestReportsAndOverdue proves the read-only aggregates behave the way the
// dashboard expects, using only a single fixture so the expected numbers are
// trivial to derive.
//
// Fixture: one ACTIVE MONTHLY subscription priced 10000 cents, one confirmed
// unpaid invoice for 10000 cents with a due date in the past, and one paid
// invoice of 5000 cents paid today.
func TestReportsAndOverdue(t *testing.T) {
	reset(t)
	admin := adminClient(t)

	// A MONTHLY plan so MRR is an exact 10000 (no weekly approximation).
	var product struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/products", map[string]any{"name": "Plan product"}, &product)
	var tax struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/taxes", map[string]any{
		"name": "VAT", "rate_bp": 0, "is_active": true,
	}, &tax)
	var plan struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/plans", map[string]any{
		"product_id": product.ID, "name": "Monthly plan", "interval": "MONTHLY",
		"price_cents": 10000, "is_active": true, "tax_id": tax.ID,
	}, &plan)

	customerID := createCustomer(t, admin, "Report Corp")

	// Active subscription contributes its MRR.
	sub := createSubscription(t, admin, customerID, plan.ID)
	addLine(t, admin, sub.ID, &catalogFixture{ProductID: product.ID, TaxID: tax.ID}, 1)
	activate(t, admin, sub.ID)

	// One confirmed, overdue, unpaid invoice (due date in the past).
	var overdue invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID, "due_date": "2026-01-01",
		"lines": []map[string]any{{"description": "Overdue", "quantity": 1, "unit_price_cents": 10000}},
	}, &overdue)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+overdue.ID+"/confirm", map[string]any{}, nil)

	// One confirmed, fully paid invoice.
	var paid invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID,
		"lines":       []map[string]any{{"description": "Paid", "quantity": 1, "unit_price_cents": 5000}},
	}, &paid)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+paid.ID+"/confirm", map[string]any{}, nil)
	admin.mustOK(http.MethodPost, "/api/v1/payments", map[string]any{
		"invoice_id": paid.ID, "amount_cents": 5000, "method": "CASH",
	}, nil)

	// Summary: one active sub, MRR 10000, one unpaid invoice of 10000, one
	// overdue invoice of 10000, 5000 collected this month.
	var summary struct {
		ActiveSubscriptions     int   `json:"active_subscriptions"`
		MonthlyRecurringCents   int64 `json:"mrr_cents"`
		OutstandingCents        int64 `json:"outstanding_cents"`
		OverdueCount            int   `json:"overdue_invoices"`
		OverdueCents            int64 `json:"overdue_cents"`
		CollectedThisMonthCents int64 `json:"collected_this_month_cents"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/summary", nil, &summary)
	if summary.ActiveSubscriptions != 1 {
		t.Errorf("active subscriptions = %d, want 1", summary.ActiveSubscriptions)
	}
	if summary.MonthlyRecurringCents != 10000 {
		t.Errorf("MRR = %d, want 10000 (one MONTHLY plan at 10000)", summary.MonthlyRecurringCents)
	}
	if summary.OutstandingCents != 10000 {
		t.Errorf("outstanding = %d, want 10000 (the unpaid invoice)", summary.OutstandingCents)
	}
	if summary.OverdueCount != 1 || summary.OverdueCents != 10000 {
		t.Errorf("overdue = %d/%d, want 1/10000", summary.OverdueCount, summary.OverdueCents)
	}
	if summary.CollectedThisMonthCents != 5000 {
		t.Errorf("collected this month = %d, want 5000", summary.CollectedThisMonthCents)
	}

	// Overdue list: the unpaid invoice is named, with days late > 0.
	var overdueList []struct {
		Number       string `json:"number"`
		DaysLate     int    `json:"days_late"`
		BalanceCents int64  `json:"balance_cents"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/overdue", nil, &overdueList)
	if len(overdueList) != 1 {
		t.Fatalf("overdue list length = %d, want 1", len(overdueList))
	}
	if overdueList[0].Number != overdue.Number || overdueList[0].DaysLate <= 0 || overdueList[0].BalanceCents != 10000 {
		t.Errorf("overdue entry wrong: %+v", overdueList[0])
	}

	// Revenue series: 12 points ending in the current month, with the paid
	// invoice landing in the final bucket.
	var revenue []struct {
		Month     string `json:"month"`
		PaidCents int64  `json:"paid_cents"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/revenue?months=12", nil, &revenue)
	if len(revenue) != 12 {
		t.Fatalf("revenue series length = %d, want 12", len(revenue))
	}
	last := revenue[len(revenue)-1]
	if last.Month != time.Now().UTC().Format("2006-01") {
		t.Errorf("last revenue month = %s, want current month", last.Month)
	}
	if last.PaidCents != 5000 {
		t.Errorf("current-month paid = %d, want 5000", last.PaidCents)
	}

	// Subscription status counts: exactly one ACTIVE.
	var statusCounts []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/reports/subscriptions", nil, &statusCounts)
	found := false
	for _, sc := range statusCounts {
		if sc.Status == "ACTIVE" && sc.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("no ACTIVE count of 1 in %v", statusCounts)
	}
}
