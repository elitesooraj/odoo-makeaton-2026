package subscriptions

import (
	"errors"
	"math"
)

// Money is always handled in integer minor units (cents). Basis points are
// used for tax rates, where 10000 basis points equals 100%.
const (
	// BasisPointsDenominator is the number of basis points in 100%.
	BasisPointsDenominator int64 = 10000
	// MaxTaxRateBp is the largest accepted tax rate, i.e. 100%.
	MaxTaxRateBp int32 = 10000
)

var (
	// ErrInvalidQuantity reports a quantity that is not positive.
	ErrInvalidQuantity = errors.New("quantity must be greater than zero")
	// ErrInvalidAmount reports a negative monetary input.
	ErrInvalidAmount = errors.New("amount must not be negative")
	// ErrInvalidTaxRate reports a tax rate outside 0..10000 basis points.
	ErrInvalidTaxRate = errors.New("tax rate must be between 0 and 10000 basis points")
	// ErrOverflow reports a calculation that would not fit in an int64.
	ErrOverflow = errors.New("money calculation overflowed")
)

// LineAmounts holds the calculated money values for a single subscription line.
type LineAmounts struct {
	SubtotalCents int64
	TaxCents      int64
	TotalCents    int64
}

// LineSubtotalCents returns quantity * unitPriceCents, rejecting a non-positive
// quantity, a negative price and any int64 overflow.
func LineSubtotalCents(quantity int32, unitPriceCents int64) (int64, error) {
	if quantity <= 0 {
		return 0, ErrInvalidQuantity
	}
	if unitPriceCents < 0 {
		return 0, ErrInvalidAmount
	}

	subtotal, overflow := mulNonNegative(int64(quantity), unitPriceCents)
	if overflow {
		return 0, ErrOverflow
	}
	return subtotal, nil
}

// LineTaxCents returns the tax for subtotalCents at taxRateBp. The result is
// rounded half up to the nearest cent: a fractional part of exactly half a cent
// rounds away from zero (for the non-negative amounts allowed here, upwards).
// For example, 5 cents at 1250 bp (12.5%) gives 0.625 cents and rounds to 1 cent.
func LineTaxCents(subtotalCents int64, taxRateBp int32) (int64, error) {
	if subtotalCents < 0 {
		return 0, ErrInvalidAmount
	}
	if taxRateBp < 0 || taxRateBp > MaxTaxRateBp {
		return 0, ErrInvalidTaxRate
	}

	product, overflow := mulNonNegative(subtotalCents, int64(taxRateBp))
	if overflow {
		return 0, ErrOverflow
	}

	tax := product / BasisPointsDenominator
	if remainder := product % BasisPointsDenominator; remainder*2 >= BasisPointsDenominator {
		tax++
	}
	return tax, nil
}

// LineTotalCents returns subtotalCents + taxCents, rejecting negative inputs and
// int64 overflow.
func LineTotalCents(subtotalCents, taxCents int64) (int64, error) {
	if subtotalCents < 0 || taxCents < 0 {
		return 0, ErrInvalidAmount
	}
	if taxCents > math.MaxInt64-subtotalCents {
		return 0, ErrOverflow
	}
	return subtotalCents + taxCents, nil
}

// CalculateLine applies the full line calculation: subtotal, then tax, then
// total. It is the single source of truth used by validation.
func CalculateLine(quantity int32, unitPriceCents int64, taxRateBp int32) (LineAmounts, error) {
	subtotal, err := LineSubtotalCents(quantity, unitPriceCents)
	if err != nil {
		return LineAmounts{}, err
	}
	tax, err := LineTaxCents(subtotal, taxRateBp)
	if err != nil {
		return LineAmounts{}, err
	}
	total, err := LineTotalCents(subtotal, tax)
	if err != nil {
		return LineAmounts{}, err
	}

	return LineAmounts{
		SubtotalCents: subtotal,
		TaxCents:      tax,
		TotalCents:    total,
	}, nil
}

// mulNonNegative multiplies two non-negative int64 values and reports whether
// the product would overflow.
func mulNonNegative(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	if a > math.MaxInt64/b {
		return 0, true
	}
	return a * b, false
}
