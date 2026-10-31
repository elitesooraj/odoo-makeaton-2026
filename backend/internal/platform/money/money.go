// Package money holds the arithmetic shared by billing, invoices and reports.
// All amounts are integer minor units (cents); no floats are used anywhere.
package money

import (
	"math"
	"math/big"
	"time"
)

// Shared monetary limits prevent int64 overflow in all arithmetic paths.
const (
	MaxAmountCents int64 = 1_000_000_000_000 // 10 billion currency units (e.g. $10B)
	MaxQuantity    int   = 1_000_000
	MaxBasisPoints int   = 10_000
)

// exactInt computes value * basisPoints / divisor with half-up rounding,
// using arbitrary-precision intermediates so int64 overflow is impossible.
func exactInt(value int64, bp int, divisor int64) int64 {
	v := big.NewInt(value)
	b := big.NewInt(int64(bp))
	div := big.NewInt(divisor)
	prod := new(big.Int).Mul(v, b)
	if bp > 0 && value > 0 {
		prod.Add(prod, big.NewInt(divisor/2))
	}
	prod.Div(prod, div)
	if !prod.IsInt64() {
		return math.MaxInt64
	}
	return prod.Int64()
}

// Percent returns value * basisPoints / 10000, rounded half up.
func Percent(value int64, basisPoints int) int64 {
	if value <= 0 || basisPoints <= 0 {
		return 0
	}
	return exactInt(value, basisPoints, 10000)
}

// IncludedTax returns the tax contained inside a tax-inclusive amount:
// value * bp / (10000 + bp), rounded half up.
func IncludedTax(value int64, basisPoints int) int64 {
	if value <= 0 || basisPoints <= 0 {
		return 0
	}
	den := int64(10000 + basisPoints)
	v := big.NewInt(value)
	bpBig := big.NewInt(int64(basisPoints))
	denBig := big.NewInt(den)
	num := new(big.Int).Mul(v, bpBig)
	num.Mul(num, big.NewInt(2))
	num.Add(num, denBig)
	num.Div(num, new(big.Int).Mul(denBig, big.NewInt(2)))
	if !num.IsInt64() {
		return math.MaxInt64
	}
	return num.Int64()
}

// Allocate splits amount across weights proportionally. The arithmetic uses
// 128-bit intermediates (via big.Int) so overflow is impossible; callers
// must still enforce amount <= MaxAmountCents to keep results within int64.
func Allocate(amount int64, weights []int64) []int64 {
	out := make([]int64, len(weights))
	var total int64
	for _, w := range weights {
		total += w
	}
	if total <= 0 || amount <= 0 {
		return out
	}

	amountBig := big.NewInt(amount)
	totalBig := big.NewInt(total)

	var allocated int64
	remainders := make([]int64, len(weights))
	for i, w := range weights {
		wBig := big.NewInt(w)
		raw := new(big.Int).Mul(amountBig, wBig)
		raw.Div(raw, totalBig)
		out[i] = raw.Int64()
		raw2 := new(big.Int).Mul(amountBig, wBig)
		raw2.Mod(raw2, totalBig)
		remainders[i] = raw2.Int64()
		allocated += out[i]
	}

	left := amount - allocated
	order := make([]int, len(weights))
	for i := range order {
		order[i] = i
	}
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && remainders[order[j]] > remainders[order[j-1]]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	for i := 0; i < len(order) && left > 0; i++ {
		out[order[i]]++
		left--
	}
	return out
}

// AddInterval advances a date by one billing interval.
func AddInterval(t time.Time, interval string) time.Time {
	switch interval {
	case "DAILY":
		return t.AddDate(0, 0, 1)
	case "WEEKLY":
		return t.AddDate(0, 0, 7)
	case "YEARLY":
		return t.AddDate(1, 0, 0)
	default: // MONTHLY
		return t.AddDate(0, 1, 0)
	}
}

// MonthlyEquivalent normalises a price to a monthly figure for MRR.
func MonthlyEquivalent(cents int64, interval string) int64 {
	v := big.NewInt(cents)
	switch interval {
	case "DAILY":
		v.Mul(v, big.NewInt(30))
	case "WEEKLY":
		v.Mul(v, big.NewInt(52))
		v.Div(v, big.NewInt(12))
	case "YEARLY":
		v.Div(v, big.NewInt(12))
	}
	if !v.IsInt64() {
		return math.MaxInt64
	}
	return v.Int64()
}

func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
