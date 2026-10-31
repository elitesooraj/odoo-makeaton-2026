// Package money holds the arithmetic shared by billing, invoices and reports.
// All amounts are integer minor units (cents); no floats are used anywhere.
package money

import "time"

// Percent returns value * basisPoints / 10000, rounded half up.
func Percent(value int64, basisPoints int) int64 {
	if value <= 0 || basisPoints <= 0 {
		return 0
	}
	return (value*int64(basisPoints) + 5000) / 10000
}

// IncludedTax returns the tax contained inside a tax-inclusive amount:
// value * bp / (10000 + bp), rounded half up.
func IncludedTax(value int64, basisPoints int) int64 {
	if value <= 0 || basisPoints <= 0 {
		return 0
	}
	den := int64(10000 + basisPoints)
	return (2*value*int64(basisPoints) + den) / (2 * den)
}

// Allocate splits amount across weights proportionally, rounding half up per
// bucket and distributing leftover cents to the largest weights first. The sum
// of the result is exactly amount, so invoice math stays balanced.
func Allocate(amount int64, weights []int64) []int64 {
	out := make([]int64, len(weights))
	var total int64
	for _, w := range weights {
		total += w
	}
	if total <= 0 || amount <= 0 {
		return out
	}

	// Base shares plus remainder bookkeeping.
	var allocated int64
	remainders := make([]int64, len(weights))
	for i, w := range weights {
		raw := amount * w
		out[i] = raw / total
		remainders[i] = raw % total
		allocated += out[i]
	}

	left := amount - allocated
	// Order by largest remainder (stable so equal shares behave predictably).
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

// AddInterval advances a date by one billing interval. Note: like Go's
// AddDate it normalizes overflow instead of clamping to the month end, so
// Jan 31 + MONTHLY lands on March 3 and month-end starts drift forward.
// See money_test.go for the documented cases; clamping is a product
// decision left to the module owner.
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

// MonthlyEquivalent normalises a price to a monthly figure for MRR. Results are
// approximate for weekly plans, which is documented on the report.
func MonthlyEquivalent(cents int64, interval string) int64 {
	switch interval {
	case "DAILY":
		return cents * 30
	case "WEEKLY":
		return cents * 52 / 12
	case "YEARLY":
		return cents / 12
	default:
		return cents
	}
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
