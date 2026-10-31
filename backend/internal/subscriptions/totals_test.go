package subscriptions

import (
	"errors"
	"math"
	"testing"
)

func TestLineSubtotalCents(t *testing.T) {
	tests := []struct {
		name      string
		quantity  int32
		unitPrice int64
		want      int64
		wantErr   error
	}{
		{name: "normal", quantity: 3, unitPrice: 1999, want: 5997},
		{name: "zero price", quantity: 5, unitPrice: 0, want: 0},
		{name: "single unit", quantity: 1, unitPrice: 12345, want: 12345},
		{name: "large valid", quantity: 1000, unitPrice: 1_000_000, want: 1_000_000_000},
		{name: "max int32 quantity", quantity: math.MaxInt32, unitPrice: 1, want: math.MaxInt32},
		{name: "zero quantity", quantity: 0, unitPrice: 100, wantErr: ErrInvalidQuantity},
		{name: "negative quantity", quantity: -1, unitPrice: 100, wantErr: ErrInvalidQuantity},
		{name: "negative price", quantity: 1, unitPrice: -1, wantErr: ErrInvalidAmount},
		{
			name:      "overflow",
			quantity:  math.MaxInt32,
			unitPrice: math.MaxInt64,
			wantErr:   ErrOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LineSubtotalCents(tt.quantity, tt.unitPrice)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("LineSubtotalCents(%d, %d) error = %v, want %v", tt.quantity, tt.unitPrice, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("LineSubtotalCents(%d, %d) = %d, want %d", tt.quantity, tt.unitPrice, got, tt.want)
			}
		})
	}
}

func TestLineTaxCents(t *testing.T) {
	tests := []struct {
		name     string
		subtotal int64
		rateBp   int32
		want     int64
		wantErr  error
	}{
		{name: "normal", subtotal: 10000, rateBp: 1000, want: 1000},
		{name: "zero tax rate", subtotal: 10000, rateBp: 0, want: 0},
		{name: "zero subtotal", subtotal: 0, rateBp: 1000, want: 0},
		{name: "full rate", subtotal: 12345, rateBp: 10000, want: 12345},
		{name: "exact half rounds up", subtotal: 5, rateBp: 1000, want: 1},
		{name: "above half rounds up", subtotal: 5, rateBp: 1250, want: 1},
		{name: "below half rounds down", subtotal: 3, rateBp: 1000, want: 0},
		{name: "just below half rounds down", subtotal: 149, rateBp: 100, want: 1},
		{name: "just above half rounds up", subtotal: 151, rateBp: 100, want: 2},
		{name: "exact half at scale rounds up", subtotal: 150, rateBp: 100, want: 2},
		{name: "large valid", subtotal: 1_000_000_000_000, rateBp: 10000, want: 1_000_000_000_000},
		{name: "negative subtotal", subtotal: -1, rateBp: 1000, wantErr: ErrInvalidAmount},
		{name: "negative rate", subtotal: 100, rateBp: -1, wantErr: ErrInvalidTaxRate},
		{name: "rate above maximum", subtotal: 100, rateBp: 10001, wantErr: ErrInvalidTaxRate},
		{name: "overflow", subtotal: math.MaxInt64, rateBp: 10000, wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LineTaxCents(tt.subtotal, tt.rateBp)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("LineTaxCents(%d, %d) error = %v, want %v", tt.subtotal, tt.rateBp, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("LineTaxCents(%d, %d) = %d, want %d", tt.subtotal, tt.rateBp, got, tt.want)
			}
		})
	}
}

func TestLineTotalCents(t *testing.T) {
	tests := []struct {
		name     string
		subtotal int64
		tax      int64
		want     int64
		wantErr  error
	}{
		{name: "normal", subtotal: 1000, tax: 180, want: 1180},
		{name: "both zero", subtotal: 0, tax: 0, want: 0},
		{name: "zero tax", subtotal: 999, tax: 0, want: 999},
		{name: "max int64 without tax", subtotal: math.MaxInt64, tax: 0, want: math.MaxInt64},
		{name: "negative subtotal", subtotal: -1, tax: 0, wantErr: ErrInvalidAmount},
		{name: "negative tax", subtotal: 100, tax: -1, wantErr: ErrInvalidAmount},
		{name: "overflow", subtotal: math.MaxInt64, tax: 1, wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LineTotalCents(tt.subtotal, tt.tax)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("LineTotalCents(%d, %d) error = %v, want %v", tt.subtotal, tt.tax, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("LineTotalCents(%d, %d) = %d, want %d", tt.subtotal, tt.tax, got, tt.want)
			}
		})
	}
}

func TestCalculateLine(t *testing.T) {
	tests := []struct {
		name      string
		quantity  int32
		unitPrice int64
		rateBp    int32
		want      LineAmounts
		wantErr   error
	}{
		{
			name:      "normal",
			quantity:  2,
			unitPrice: 2500,
			rateBp:    1000,
			want:      LineAmounts{SubtotalCents: 5000, TaxCents: 500, TotalCents: 5500},
		},
		{
			name:      "zero price",
			quantity:  4,
			unitPrice: 0,
			rateBp:    1000,
			want:      LineAmounts{},
		},
		{
			name:      "zero tax",
			quantity:  1,
			unitPrice: 1999,
			rateBp:    0,
			want:      LineAmounts{SubtotalCents: 1999, TaxCents: 0, TotalCents: 1999},
		},
		{
			name:      "fractional cent rounds half up",
			quantity:  1,
			unitPrice: 5,
			rateBp:    1250,
			want:      LineAmounts{SubtotalCents: 5, TaxCents: 1, TotalCents: 6},
		},
		{
			name:      "fractional cent rounds down",
			quantity:  1,
			unitPrice: 3,
			rateBp:    1000,
			want:      LineAmounts{SubtotalCents: 3, TaxCents: 0, TotalCents: 3},
		},
		{
			name:      "zero quantity",
			quantity:  0,
			unitPrice: 100,
			rateBp:    1000,
			wantErr:   ErrInvalidQuantity,
		},
		{
			name:      "negative price",
			quantity:  1,
			unitPrice: -100,
			rateBp:    1000,
			wantErr:   ErrInvalidAmount,
		},
		{
			name:      "invalid tax rate",
			quantity:  1,
			unitPrice: 100,
			rateBp:    10001,
			wantErr:   ErrInvalidTaxRate,
		},
		{
			name:      "overflow",
			quantity:  math.MaxInt32,
			unitPrice: math.MaxInt64,
			rateBp:    1000,
			wantErr:   ErrOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateLine(tt.quantity, tt.unitPrice, tt.rateBp)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CalculateLine(%d, %d, %d) error = %v, want %v", tt.quantity, tt.unitPrice, tt.rateBp, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("CalculateLine(%d, %d, %d) = %+v, want %+v", tt.quantity, tt.unitPrice, tt.rateBp, got, tt.want)
			}
		})
	}
}
