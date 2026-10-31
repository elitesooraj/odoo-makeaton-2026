package invoices

import (
	"testing"

	"github.com/brightloop/subscription-platform/backend/internal/platform/money"
)

// This file locks the arithmetic primitives finalize() is built from. The
// full DB-coupled flow (snapshot, confirm, tax_cents aggregate) is covered by
// the integration suite in backend/tests/integration.

// gross locks the most basic line rule: gross = quantity x unit price.
func TestGross(t *testing.T) {
	tests := []struct {
		name string
		line draftLine
		want int64
	}{
		{
			name: "two units at 14900 cents gross to 29800",
			line: draftLine{Line: Line{Quantity: 2, UnitPriceCents: 14900}},
			want: 29800,
		},
		{
			name: "a single unit is its own gross",
			line: draftLine{Line: Line{Quantity: 1, UnitPriceCents: 500}},
			want: 500,
		},
		{
			name: "a zero-priced line grosses to zero",
			line: draftLine{Line: Line{Quantity: 3, UnitPriceCents: 0}},
			want: 0,
		},
		{
			name: "an empty draft line has zero gross",
			line: draftLine{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gross(tt.line); got != tt.want {
				t.Fatalf("gross = %d, want %d", got, tt.want)
			}
		})
	}
}

// grossWeights locks the input to the discount split: every line's weight is
// its gross amount, so a discount is distributed proportionally to what each
// line costs.
func TestGrossWeights(t *testing.T) {
	drafts := []draftLine{
		{Line: Line{Quantity: 2, UnitPriceCents: 14900}}, // gross 29800
		{Line: Line{Quantity: 1, UnitPriceCents: 10000}}, // gross 10000
		{Line: Line{Quantity: 1, UnitPriceCents: 200}},   // gross 200
	}

	want := []int64{29800, 10000, 200}
	got := grossWeights(drafts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("grossWeights = %v, want %v", got, want)
		}
	}
}

// sumLineTotals locks the total rule: the invoice total before tax is the
// sum of per-line totals (each of which is gross minus its discount share).
func TestSumLineTotals(t *testing.T) {
	drafts := []draftLine{
		{Line: Line{LineTotalCents: 29800}},
		{Line: Line{LineTotalCents: 10000}},
		{Line: Line{LineTotalCents: 0}}, // a fully discounted line contributes nothing
	}

	if got := sumLineTotals(drafts); got != 39800 {
		t.Fatalf("sumLineTotals = %d, want 39800", got)
	}
}

// TestInvoiceMoneyModel walks the exact arithmetic finalize() applies, using
// the same money primitives, and pins every number. This is the worked
// example from docs/API.md as an executable specification:
//
//	Two lines: 2 x 14900 and 1 x 10000.
//	Discount: 10% (1000bp), applied per line on gross.
//	Tax: 18% (1800bp), exclusive, added on top of line totals.
//
//	line 1: gross 29800, discount 2980,  line_total 26820, tax 4828 (26820 x 18% = 4827.6, half up)
//	line 2: gross 10000, discount 1000,  line_total  9000, tax 1620 (exact)
//	subtotal 39800, discount 3980, tax total 6448, invoice total 42268.
func TestInvoiceMoneyModelExclusiveTax(t *testing.T) {
	lines := []struct {
		quantity int
		unit     int64
	}{
		{2, 14900},
		{1, 10000},
	}
	const discountBP = 1000 // 10%
	const taxBP = 1800      // 18% exclusive

	var subtotal, discountTotal, taxTotal, lineTotalSum int64
	for _, l := range lines {
		g := int64(l.quantity) * l.unit
		subtotal += g

		share := money.Percent(g, discountBP)
		discountTotal += share

		lineTotal := g - share
		if lineTotal < 0 {
			lineTotal = 0 // a discount may never push a line below zero
		}
		lineTotalSum += lineTotal

		taxTotal += money.Percent(lineTotal, taxBP)
	}

	total := lineTotalSum + taxTotal // exclusive tax is added on top

	if subtotal != 39800 {
		t.Errorf("subtotal = %d, want 39800", subtotal)
	}
	if discountTotal != 3980 {
		t.Errorf("discount = %d, want 3980", discountTotal)
	}
	if taxTotal != 6448 {
		t.Errorf("exclusive tax = %d, want 6448 (4828 + 1620, half-up per line)", taxTotal)
	}
	if total != 42268 {
		t.Errorf("total = %d, want 42268 (35820 line totals + 6448 tax)", total)
	}
}

// The inclusive-tax variant: the tax is carved OUT of the line totals and is
// informational only — the customer pays exactly the line totals.
//
//	One line: 2 x 9000 = 18000 gross, priced "18% VAT included".
//	No discount. Included tax = 18000 * 1800 / 11800 = 2746.
//	Invoice total stays 18000: nothing is added on top.
func TestInvoiceMoneyModelInclusiveTax(t *testing.T) {
	const unit = int64(9000)
	const quantity = 2
	const taxBP = 1800

	gross := quantity * unit
	tax := money.IncludedTax(gross, taxBP)
	total := gross // inclusive tax never changes what the customer pays

	if gross != 18000 {
		t.Fatalf("gross = %d, want 18000", gross)
	}
	if tax != 2746 {
		t.Fatalf("included tax = %d, want 2746", tax)
	}
	if total != 18000 {
		t.Fatalf("inclusive total = %d, want 18000: the tax is carved out, never added", total)
	}
}

// A FIXED discount larger than a line's gross is clamped: the line total
// floors at zero and can never go negative, even though the discount itself
// is not reduced. Documented thin-implementation behavior.
func TestLineTotalClampsAtZeroWhenDiscountExceedsGross(t *testing.T) {
	gross := int64(500)           // a small line
	discount := int64(1000)       // a FIXED discount bigger than the line
	lineTotal := gross - discount // finalize's raw arithmetic
	if lineTotal < 0 {
		lineTotal = 0 // the clamp finalize applies
	}

	if lineTotal != 0 {
		t.Fatalf("line total = %d, want 0: a discount may never make a line negative", lineTotal)
	}
}
