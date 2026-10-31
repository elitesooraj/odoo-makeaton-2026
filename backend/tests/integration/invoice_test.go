package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/brightloop/subscription-platform/backend/internal/platform/money"
)

// TestInvoiceFromSubscriptionAndPayment locks the money flow end to end,
// with the exact arithmetic from docs/API.md (the same numbers as the
// invoice unit tests, now proven through HTTP and Postgres):
//
//	line: 3 x 14900 = 44700 gross (the plan-seeded default line plus the
//	      2 units we added), no discount
//	tax:  18% exclusive on 44700 = 8046
//	total: 44700 + 8046 = 52746, balance starts equal to total
//
// Then payments: partial -> balance drops, final -> PAID, overpay -> 422.
func TestInvoiceFromSubscriptionAndPayment(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Acme Corp")

	sub := createSubscription(t, admin, customerID, fix.PlanID)
	addLine(t, admin, sub.ID, fix, 2)
	activate(t, admin, sub.ID)

	// Snapshot invoice straight from the subscription's lines. Creation
	// already computes the totals; confirm below only locks them.
	var draft invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID, "subscription_id": sub.ID,
	}, &draft)
	if draft.Status != "DRAFT" {
		t.Fatalf("new invoice status = %q, want DRAFT", draft.Status)
	}
	if draft.SubscriptionID == nil || *draft.SubscriptionID != sub.ID {
		t.Fatalf("invoice not linked to the subscription: %+v", draft.SubscriptionID)
	}
	if len(draft.Lines) != 2 {
		t.Fatalf("subscription lines not snapshotted: %+v", draft.Lines)
	}

	// Confirm locks the totals and mints the number.
	var confirmed invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+draft.ID+"/confirm", map[string]any{}, &confirmed)
	if confirmed.Status != "CONFIRMED" {
		t.Fatalf("status after confirm = %q, want CONFIRMED", confirmed.Status)
	}
	if !strings.HasPrefix(confirmed.Number, "INV-") {
		t.Fatalf("confirmed invoice number = %q, want INV-prefixed", confirmed.Number)
	}

	// The money model, number by number.
	if confirmed.SubtotalCents != 44700 {
		t.Errorf("subtotal = %d, want 44700 (3 x 14900)", confirmed.SubtotalCents)
	}
	if confirmed.TaxCents != 8046 {
		t.Errorf("tax_cents = %d, want 8046 (18%% of 44700)", confirmed.TaxCents)
	}
	if confirmed.TotalCents != 52746 {
		t.Errorf("total = %d, want 52746 (44700 + 8046 exclusive tax)", confirmed.TotalCents)
	}
	if confirmed.BalanceCents != confirmed.TotalCents {
		t.Errorf("balance = %d, want the full total %d before any payment",
			confirmed.BalanceCents, confirmed.TotalCents)
	}
	if len(confirmed.Lines) != 2 {
		t.Errorf("expected 2 snapshotted lines, got %d", len(confirmed.Lines))
	}

	// Editing after confirm is refused.
	admin.mustStatus(http.MethodPatch, "/api/v1/invoices/"+draft.ID,
		map[string]any{"notes": "too late"}, http.StatusConflict)

	// Partial payment: 10000 in, balance drops by exactly that.
	var afterPartial struct {
		Invoice invoiceDTO `json:"invoice"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/payments", map[string]any{
		"invoice_id": draft.ID, "amount_cents": 10000, "method": "CARD",
	}, &afterPartial)
	inv := afterPartial.Invoice
	if inv.BalanceCents != 42746 {
		t.Errorf("balance after 10000 payment = %d, want 42746 (52746 - 10000)", inv.BalanceCents)
	}
	if inv.Status != "CONFIRMED" {
		t.Errorf("status after partial payment = %q, want still CONFIRMED", inv.Status)
	}

	// Overpayment is a validation error naming the offending field.
	env := admin.mustStatus(http.MethodPost, "/api/v1/payments", map[string]any{
		"invoice_id": draft.ID, "amount_cents": 999999, "method": "CARD",
	}, http.StatusUnprocessableEntity)
	if _, ok := env.Error.Details["amount_cents"]; !ok {
		t.Fatalf("overpayment should flag amount_cents, got: %s", env.Error)
	}

	// Final payment flips the invoice to PAID with zero balance.
	var afterFinal struct {
		Invoice invoiceDTO `json:"invoice"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/payments", map[string]any{
		"invoice_id": draft.ID, "amount_cents": 42746, "method": "BANK_TRANSFER",
	}, &afterFinal)
	if afterFinal.Invoice.Status != "PAID" {
		t.Errorf("status after settling = %q, want PAID", afterFinal.Invoice.Status)
	}
	if afterFinal.Invoice.BalanceCents != 0 {
		t.Errorf("balance after settling = %d, want 0", afterFinal.Invoice.BalanceCents)
	}

	// A paid invoice accepts no further payments: any positive amount now
	// exceeds the remaining balance of 0.
	admin.mustStatus(http.MethodPost, "/api/v1/payments", map[string]any{
		"invoice_id": draft.ID, "amount_cents": 1, "method": "CASH",
	}, http.StatusUnprocessableEntity)
}

// TestAdHocInvoiceWithDiscountAndInclusiveTax covers the other creation
// mode (explicit lines, no subscription) with a percent discount and an
// inclusive tax, so both tax directions are proven through the API:
//
//	line: 1 x 18000 gross, 10% discount -> line total 16200
//	tax:  18% INCLUSIVE of 16200 (the tax included in that price, derived
//	      with the same IncludedTax primitive the unit tests hand-verify)
//	total: stays 16200 — inclusive tax is carved out, never added on top.
func TestAdHocInvoiceWithDiscountAndInclusiveTax(t *testing.T) {
	reset(t)
	admin := adminClient(t)

	// 18% inclusive tax.
	var tax struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/taxes", map[string]any{
		"name": "VAT 18 included", "rate_bp": 1800, "is_inclusive": true, "is_active": true,
	}, &tax)

	var discount struct {
		ID string `json:"id"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/discounts", map[string]any{
		"name": "Launch 10", "kind": "PERCENT", "value_bp": 1000, "is_active": true,
	}, &discount)

	customerID := createCustomer(t, admin, "Inclusive Corp")

	var inv invoiceDTO
	admin.mustOK(http.MethodPost, "/api/v1/invoices", map[string]any{
		"customer_id": customerID,
		"discount_id": discount.ID,
		"lines": []map[string]any{{
			"description": "Consulting", "quantity": 1,
			"unit_price_cents": 18000, "tax_id": tax.ID,
		}},
	}, &inv)
	admin.mustOK(http.MethodPost, "/api/v1/invoices/"+inv.ID+"/confirm", map[string]any{}, &inv)

	if inv.SubtotalCents != 18000 {
		t.Errorf("subtotal = %d, want 18000", inv.SubtotalCents)
	}
	if inv.DiscountCents != 1800 {
		t.Errorf("discount = %d, want 1800 (10%% of the gross)", inv.DiscountCents)
	}
	if inv.Lines[0].LineTotalCents != 16200 {
		t.Errorf("line total = %d, want 16200 (18000 - 1800)", inv.Lines[0].LineTotalCents)
	}

	wantIncludedTax := money.IncludedTax(16200, 1800) // hand-verified primitive
	if inv.Lines[0].TaxCents != wantIncludedTax {
		t.Errorf("included tax = %d, want %d", inv.Lines[0].TaxCents, wantIncludedTax)
	}
	if inv.TotalCents != 16200 {
		t.Errorf("total = %d, want 16200: inclusive tax is carved out, never added", inv.TotalCents)
	}
	if inv.TaxCents != inv.Lines[0].TaxCents {
		t.Errorf("invoice tax_cents = %d, want the per-line sum %d",
			inv.TaxCents, inv.Lines[0].TaxCents)
	}
}
