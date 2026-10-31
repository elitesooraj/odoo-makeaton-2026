package subscriptions

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	// MaxSubscriptionNumberLength is the largest accepted subscription number.
	MaxSubscriptionNumberLength = 64
	// MaxSubscriptionNotesLength is the largest accepted notes value.
	MaxSubscriptionNotesLength = 2000
)

// ValidateSubscription checks the invariants of a subscription header. It
// returns a map of field name to message; an empty map means the subscription is
// valid. The caller's value is never modified.
func ValidateSubscription(s *Subscription) map[string]string {
	details := map[string]string{}
	if s == nil {
		details["subscription"] = "subscription is required"
		return details
	}

	if s.CustomerID == uuid.Nil {
		details["customer_id"] = "customer id is required"
	}
	if s.PlanID == uuid.Nil {
		details["plan_id"] = "plan id is required"
	}

	switch {
	case strings.TrimSpace(s.Number) == "":
		details["number"] = "subscription number is required"
	case utf8.RuneCountInString(s.Number) > MaxSubscriptionNumberLength:
		details["number"] = "subscription number must be at most 64 characters"
	}

	if !s.Status.Valid() {
		details["status"] = "status must be one of DRAFT, QUOTATION, CONFIRMED, ACTIVE, PAUSED, CLOSED"
	}

	if s.StartDate.IsZero() {
		details["start_date"] = "start date is required"
	}
	if s.EndDate != nil {
		switch {
		case s.EndDate.IsZero():
			details["end_date"] = "end date must be a valid date"
		case !s.StartDate.IsZero() && !s.EndDate.After(s.StartDate):
			details["end_date"] = "end date must be after the start date"
		}
	}

	if s.Notes != nil && utf8.RuneCountInString(*s.Notes) > MaxSubscriptionNotesLength {
		details["notes"] = "notes must be at most 2000 characters"
	}

	return details
}

// ValidateSubscriptionLine checks the invariants of a subscription line and
// verifies that LineTotalCents matches the server-side calculation. It returns a
// map of field name to message; an empty map means the line is valid. The
// caller's value is never modified.
func ValidateSubscriptionLine(l *SubscriptionLine) map[string]string {
	details := map[string]string{}
	if l == nil {
		details["subscription_line"] = "subscription line is required"
		return details
	}

	if l.ID == uuid.Nil {
		details["id"] = "line id is required"
	}
	if l.SubscriptionID == uuid.Nil {
		details["subscription_id"] = "subscription id is required"
	}
	if l.ProductID != nil && *l.ProductID == uuid.Nil {
		details["product_id"] = "product id must not be a zero uuid"
	}
	if l.VariantID != nil && *l.VariantID == uuid.Nil {
		details["variant_id"] = "variant id must not be a zero uuid"
	}

	if strings.TrimSpace(l.ProductName) == "" {
		details["product_name"] = "product name is required"
	}

	if l.Quantity <= 0 {
		details["quantity"] = "quantity must be greater than zero"
	}
	if l.UnitPriceCents < 0 {
		details["unit_price_cents"] = "unit price must not be negative"
	}
	if l.TaxRateBp < 0 || l.TaxRateBp > MaxTaxRateBp {
		details["tax_rate_bp"] = "tax rate must be between 0 and 10000 basis points"
	}
	if l.TaxCents < 0 {
		details["tax_cents"] = "tax amount must not be negative"
	}

	// Only compare the total once the inputs themselves are valid, otherwise the
	// calculation error would mask the real field error.
	if len(details) == 0 {
		amounts, err := CalculateLine(l.Quantity, l.UnitPriceCents, l.TaxRateBp)
		switch {
		case err != nil:
			details["line_total_cents"] = "line total could not be calculated: " + err.Error()
		case l.LineTotalCents != amounts.TotalCents:
			details["line_total_cents"] = "line total must equal subtotal plus tax (" +
				strconv.FormatInt(amounts.TotalCents, 10) + " cents)"
		}
	}

	if l.PeriodStart != nil && l.PeriodEnd != nil && !l.PeriodEnd.After(*l.PeriodStart) {
		details["period_end"] = "period end must be after the period start"
	}

	return details
}
