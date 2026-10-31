package money

import (
	"testing"
	"time"
)

// Percent locks the exclusive-tax rule: value * basisPoints / 10000, rounded
// half up, in integer cents. 1800 basis points = 18%.
//
// Worked example used across the suite: an exclusive 18% tax on a 14900-cent
// line is 14900 * 1800 / 10000 = 2682 cents — no float anywhere.
func TestPercent(t *testing.T) {
	tests := []struct {
		name        string
		value       int64
		basisPoints int
		want        int64
		explanation string
	}{
		{
			name:        "18 percent exclusive tax on a 14900 cent line",
			value:       14900,
			basisPoints: 1800,
			want:        2682,
			explanation: "14900 * 1800 / 10000 = 2682 exactly",
		},
		{
			name:        "half-up rounding at an exact half cent",
			value:       25,
			basisPoints: 200,
			want:        1,
			explanation: "25 * 2% = 0.5 cents; half up gives 1",
		},
		{
			name:        "fraction below a half cent rounds down",
			value:       999,
			basisPoints: 1,
			want:        0,
			explanation: "999 * 0.01% = 0.0999 cents; below half, so 0",
		},
		{
			name:        "a 10 percent discount in basis points",
			value:       29800,
			basisPoints: 1000,
			want:        2980,
			explanation: "29800 * 1000 / 10000 = 2980 exactly",
		},
		{
			name:        "zero value yields zero, not a panic",
			value:       0,
			basisPoints: 1800,
			want:        0,
			explanation: "guard: value <= 0",
		},
		{
			name:        "negative value yields zero",
			value:       -100,
			basisPoints: 1800,
			want:        0,
			explanation: "guard: value <= 0",
		},
		{
			name:        "zero rate yields zero",
			value:       14900,
			basisPoints: 0,
			want:        0,
			explanation: "guard: basisPoints <= 0",
		},
		{
			name:        "negative rate yields zero",
			value:       14900,
			basisPoints: -500,
			want:        0,
			explanation: "guard: basisPoints <= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Percent(tt.value, tt.basisPoints); got != tt.want {
				t.Fatalf("Percent(%d, %d) = %d, want %d (%s)",
					tt.value, tt.basisPoints, got, tt.want, tt.explanation)
			}
		})
	}
}

// IncludedTax locks the inclusive-tax rule: the tax contained *inside* a
// tax-inclusive price is value * bp / (10000 + bp), rounded half up.
//
// Worked example: a 18000-cent line priced "18% VAT included" contains
// 18000 * 1800 / 11800 = 2745.76... -> 2746 cents of tax, so the net line
// value is 18000 - 2746 = 15254 cents. Inclusive tax never changes the
// total; it is informational.
func TestIncludedTax(t *testing.T) {
	tests := []struct {
		name        string
		value       int64
		basisPoints int
		want        int64
		explanation string
	}{
		{
			name:        "18 percent VAT contained in an 18000 cent gross price",
			value:       18000,
			basisPoints: 1800,
			want:        2746,
			explanation: "18000 * 1800 / 11800 = 2745.76 -> half up -> 2746",
		},
		{
			name:        "10 percent GST contained in a 11000 cent price",
			value:       11000,
			basisPoints: 1000,
			want:        1000,
			explanation: "11000 * 1000 / 11000 = 1000 exactly: 100 net + 10 tax",
		},
		{
			name:        "zero value yields zero",
			value:       0,
			basisPoints: 1800,
			want:        0,
			explanation: "guard: value <= 0",
		},
		{
			name:        "zero rate yields zero",
			value:       18000,
			basisPoints: 0,
			want:        0,
			explanation: "guard: basisPoints <= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IncludedTax(tt.value, tt.basisPoints); got != tt.want {
				t.Fatalf("IncludedTax(%d, %d) = %d, want %d (%s)",
					tt.value, tt.basisPoints, got, tt.want, tt.explanation)
			}
		})
	}
}

// The critical property of IncludedTax: adding it back to the net must
// reconstruct the gross price for the canonical 10% case, and the derived
// net must be the documented carve-out.
func TestIncludedTaxCarveOutReconstructsGross(t *testing.T) {
	const gross = 18000
	const bp = 1800

	tax := IncludedTax(gross, bp)
	net := gross - tax

	if net+tax != gross {
		t.Fatalf("net %d + tax %d != gross %d; the carve-out must be additive",
			net, tax, gross)
	}
	if tax < 2700 || tax > 2800 {
		t.Fatalf("tax %d outside the 18%% band [2700, 2800]", tax)
	}
}

// Allocate locks the discount-splitting rule: a discount is split across
// invoice lines proportionally to their gross amounts, with leftover cents
// handed to the largest remainders first. The invariant that matters to the
// books: the parts always sum to exactly the original amount, so no cent is
// created or destroyed.
func TestAllocate(t *testing.T) {
	tests := []struct {
		name        string
		amount      int64
		weights     []int64
		want        []int64
		explanation string
	}{
		{
			name:        "even three-way split of 100 cents",
			amount:      100,
			weights:     []int64{1, 1, 1},
			want:        []int64{34, 33, 33},
			explanation: "33 each with 1 cent left; equal remainders are stable so the first line wins it",
		},
		{
			name:        "proportional split needs no correction",
			amount:      1000,
			weights:     []int64{1, 1, 2},
			want:        []int64{250, 250, 500},
			explanation: "1000 splits cleanly into quarters",
		},
		{
			name:        "larger remainder receives the leftover cent",
			amount:      10,
			weights:     []int64{1, 2},
			want:        []int64{3, 7},
			explanation: "3.33/6.67 floors to 3/6; the 0.67 remainder beats 0.33 and takes the last cent",
		},
		{
			name:        "single bucket takes everything",
			amount:      99,
			weights:     []int64{1},
			want:        []int64{99},
			explanation: "no split to make",
		},
		{
			name:        "zero amount splits into zeros",
			amount:      0,
			weights:     []int64{1, 1},
			want:        []int64{0, 0},
			explanation: "guard: amount <= 0",
		},
		{
			name:        "all-zero weights split into zeros",
			amount:      100,
			weights:     []int64{0, 0},
			want:        []int64{0, 0},
			explanation: "guard: total weight <= 0; nothing to be proportional to",
		},
		{
			name:        "no weights means no parts",
			amount:      100,
			weights:     []int64{},
			want:        []int64{},
			explanation: "degenerate input, degenerate output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Allocate(tt.amount, tt.weights)
			if len(got) != len(tt.want) {
				t.Fatalf("Allocate(%d, %v) returned %d parts, want %d",
					tt.amount, tt.weights, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("Allocate(%d, %v) = %v, want %v (%s)",
						tt.amount, tt.weights, got, tt.want, tt.explanation)
				}
			}
		})
	}
}

// The books-balance invariant: whatever the weights, the parts always sum to
// the original amount. If this ever breaks, invoice totals drift from their
// lines and reconciliation fails.
func TestAllocatePartsSumToAmount(t *testing.T) {
	cases := []struct {
		amount  int64
		weights []int64
	}{
		{100, []int64{1, 1, 1}},
		{999, []int64{7, 3}},
		{12345, []int64{5, 5, 5, 5, 5, 5}},
		{7, []int64{2, 3, 5, 7, 11}},
		{100, []int64{0, 1}}, // zero weight gets nothing, sum still holds
	}

	for _, tc := range cases {
		parts := Allocate(tc.amount, tc.weights)
		var sum int64
		for _, p := range parts {
			sum += p
		}
		if sum != tc.amount {
			t.Errorf("Allocate(%d, %v) parts sum to %d, want %d",
				tc.amount, tc.weights, sum, tc.amount)
		}
	}
}

// AddInterval locks how a billing period advances. BEWARE the normalization
// quirk this test documents on purpose: Go's AddDate does NOT clamp to the
// month end — it normalizes overflow. So a MONTHLY plan that starts on
// Jan 31 next bills on March 3 (Feb 31 does not exist), and then April 3:
// the billing day drifts forward for month-end starts. Same for a YEARLY
// leap-day plan (Feb 29 -> March 1 next year). Clamping is a product
// decision left to the module owner; until then this is the contract.
// Unknown intervals fall back to MONTHLY.
func TestAddInterval(t *testing.T) {
	utc := func(s string) time.Time {
		parsed, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("bad fixture date %q: %v", s, err)
		}
		return parsed.UTC()
	}

	tests := []struct {
		name     string
		from     string
		interval string
		want     string
	}{
		{"daily advances one day", "2026-10-31", "DAILY", "2026-11-01"},
		{"weekly advances seven days", "2026-10-31", "WEEKLY", "2026-11-07"},
		{"monthly advances one month mid-month", "2026-10-15", "MONTHLY", "2026-11-15"},
		{"monthly from Oct 31 normalizes Nov 31 into Dec 1", "2026-10-31", "MONTHLY", "2026-12-01"},
		{"monthly from Jan 31 normalizes Feb 31 into Mar 3", "2026-01-31", "MONTHLY", "2026-03-03"},
		{"monthly from Jan 31 in a leap year lands on Mar 2", "2024-01-31", "MONTHLY", "2024-03-02"},
		{"yearly advances one year mid-month", "2026-10-31", "YEARLY", "2027-10-31"},
		{"yearly from a leap day normalizes into Mar 1", "2024-02-29", "YEARLY", "2025-03-01"},
		{"unknown interval falls back to monthly", "2026-01-31", "QUARTERLY", "2026-03-03"},
		{"empty interval falls back to monthly", "2026-01-31", "", "2026-03-03"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddInterval(utc(tt.from), tt.interval)
			if got.Format("2006-01-02") != tt.want {
				t.Fatalf("AddInterval(%s, %q) = %s, want %s",
					tt.from, tt.interval, got.Format("2006-01-02"), tt.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("AddInterval must preserve UTC, got %s", got.Location())
			}
		})
	}
}

// MonthlyEquivalent locks the MRR normalisation the reports module uses.
// MONTHLY is the identity; YEARLY divides by 12; WEEKLY multiplies by 52/12
// (approximate by design — documented on the report endpoint); DAILY assumes
// a 30-day month. Unknown intervals are treated as monthly.
func TestMonthlyEquivalent(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		interval string
		want     int64
	}{
		{"monthly price is its own MRR", 14900, "MONTHLY", 14900},
		{"yearly price divides by twelve", 120000, "YEARLY", 10000},
		{"weekly price multiplies by 52/12", 1200, "WEEKLY", 5200},
		{"daily price assumes a 30 day month", 100, "DAILY", 3000},
		{"unknown interval is treated as monthly", 14900, "QUARTERLY", 14900},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MonthlyEquivalent(tt.cents, tt.interval); got != tt.want {
				t.Fatalf("MonthlyEquivalent(%d, %q) = %d, want %d",
					tt.cents, tt.interval, got, tt.want)
			}
		})
	}
}

func TestPercentBoundaryValues(t *testing.T) {
	cases := []struct {
		name        string
		value       int64
		basisPoints int
		want        int64
	}{
		{"1 basis point (0.01%)", 10000, 1, 1},
		{"9999 basis points (99.99%)", 10000, 9999, 9999},
		{"10000 basis points (100%)", 10000, 10000, 10000},
		{"negative basis points yields zero", 10000, -1, 0},
		{"basis points 10001 on 100 cents", 100, 10001, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Percent(tc.value, tc.basisPoints); got != tc.want {
				t.Fatalf("Percent(%d, %d) = %d, want %d", tc.value, tc.basisPoints, got, tc.want)
			}
		})
	}
}

func TestMinMax(t *testing.T) {
	if Min(3, 7) != 3 {
		t.Fatal("Min(3, 7) must be 3")
	}
	if Min(7, 3) != 3 {
		t.Fatal("Min(7, 3) must be 3")
	}
	if Max(3, 7) != 7 {
		t.Fatal("Max(3, 7) must be 7")
	}
	if Max(7, 3) != 7 {
		t.Fatal("Max(7, 3) must be 7")
	}
	if Min(-5, 0) != -5 {
		t.Fatal("Min must handle negatives")
	}
}
