package subscriptions

import (
	"strings"
	"testing"
	"time"
)

func validSubscription() *Subscription {
	start := testTime(2026, time.January, 1)
	end := testTime(2026, time.December, 31)
	notes := "quarterly billing"

	return &Subscription{
		ID:         testUUID(1),
		Number:     "SUB-2026-0001",
		CustomerID: testUUID(2),
		PlanID:     testUUID(3),
		Status:     StatusDraft,
		StartDate:  start,
		EndDate:    &end,
		Notes:      &notes,
		CreatedAt:  start,
		UpdatedAt:  start,
	}
}

func TestValidateSubscription(t *testing.T) {
	start := testTime(2026, time.January, 1)
	beforeStart := testTime(2025, time.December, 31)
	zeroTime := time.Time{}

	tests := []struct {
		name      string
		mutate    func(*Subscription)
		wantField string
	}{
		{name: "valid", mutate: func(*Subscription) {}},
		{name: "valid without optional fields", mutate: func(s *Subscription) {
			s.EndDate = nil
			s.Notes = nil
		}},
		{name: "valid at maximum number length", mutate: func(s *Subscription) {
			s.Number = strings.Repeat("a", MaxSubscriptionNumberLength)
		}},
		{name: "valid at maximum notes length", mutate: func(s *Subscription) {
			notes := strings.Repeat("a", MaxSubscriptionNotesLength)
			s.Notes = &notes
		}},
		{name: "zero customer id", mutate: func(s *Subscription) {
			s.CustomerID = testUUID(0)
		}, wantField: "customer_id"},
		{name: "zero plan id", mutate: func(s *Subscription) {
			s.PlanID = testUUID(0)
		}, wantField: "plan_id"},
		{name: "empty number", mutate: func(s *Subscription) {
			s.Number = ""
		}, wantField: "number"},
		{name: "blank number", mutate: func(s *Subscription) {
			s.Number = "   "
		}, wantField: "number"},
		{name: "oversized number", mutate: func(s *Subscription) {
			s.Number = strings.Repeat("a", MaxSubscriptionNumberLength+1)
		}, wantField: "number"},
		{name: "empty status", mutate: func(s *Subscription) {
			s.Status = Status("")
		}, wantField: "status"},
		{name: "unknown status", mutate: func(s *Subscription) {
			s.Status = Status("SUSPENDED")
		}, wantField: "status"},
		{name: "missing start date", mutate: func(s *Subscription) {
			s.StartDate = zeroTime
		}, wantField: "start_date"},
		{name: "end date before start", mutate: func(s *Subscription) {
			s.EndDate = &beforeStart
		}, wantField: "end_date"},
		{name: "end date equal to start", mutate: func(s *Subscription) {
			s.EndDate = &start
		}, wantField: "end_date"},
		{name: "zero end date", mutate: func(s *Subscription) {
			s.EndDate = &zeroTime
		}, wantField: "end_date"},
		{name: "oversized notes", mutate: func(s *Subscription) {
			notes := strings.Repeat("a", MaxSubscriptionNotesLength+1)
			s.Notes = &notes
		}, wantField: "notes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSubscription()
			tt.mutate(s)

			details := ValidateSubscription(s)
			if tt.wantField == "" {
				if len(details) != 0 {
					t.Fatalf("ValidateSubscription() = %v, want no errors", details)
				}
				return
			}
			if details[tt.wantField] == "" {
				t.Fatalf("ValidateSubscription() = %v, want an error on %q", details, tt.wantField)
			}
			if len(details) != 1 {
				t.Fatalf("ValidateSubscription() = %v, want exactly one error on %q", details, tt.wantField)
			}
		})
	}
}

func TestValidateSubscriptionNil(t *testing.T) {
	details := ValidateSubscription(nil)
	if details["subscription"] == "" {
		t.Fatalf("ValidateSubscription(nil) = %v, want a subscription error", details)
	}
}

func TestValidateSubscriptionDoesNotMutate(t *testing.T) {
	s := validSubscription()
	s.Number = "  SUB-1  "
	notes := "  keep me  "
	s.Notes = &notes

	_ = ValidateSubscription(s)

	if s.Number != "  SUB-1  " {
		t.Fatalf("Number mutated to %q", s.Number)
	}
	if s.Notes == nil || *s.Notes != "  keep me  " {
		t.Fatalf("Notes mutated to %v", s.Notes)
	}
}

func validSubscriptionLine() *SubscriptionLine {
	productID := testUUID(4)
	variantID := testUUID(5)
	variantName := "Large"
	start := testTime(2026, time.January, 1)
	end := testTime(2026, time.February, 1)

	return &SubscriptionLine{
		ID:             testUUID(6),
		SubscriptionID: testUUID(7),
		ProductID:      &productID,
		VariantID:      &variantID,
		ProductName:    "Coffee Beans",
		VariantName:    &variantName,
		Quantity:       2,
		UnitPriceCents: 2500,
		TaxRateBp:      1000,
		TaxCents:       500,
		LineTotalCents: 5500,
		PeriodStart:    &start,
		PeriodEnd:      &end,
		CreatedAt:      start,
	}
}

func TestValidateSubscriptionLine(t *testing.T) {
	start := testTime(2026, time.January, 1)
	beforeStart := testTime(2025, time.December, 31)
	zeroTime := time.Time{}

	tests := []struct {
		name      string
		mutate    func(*SubscriptionLine)
		wantField string
	}{
		{name: "valid", mutate: func(*SubscriptionLine) {}},
		{name: "valid without optional fields", mutate: func(l *SubscriptionLine) {
			l.ProductID = nil
			l.VariantID = nil
			l.VariantName = nil
			l.PeriodStart = nil
			l.PeriodEnd = nil
		}},
		{name: "valid with only period start", mutate: func(l *SubscriptionLine) {
			l.PeriodEnd = nil
		}},
		{name: "valid with only period end", mutate: func(l *SubscriptionLine) {
			l.PeriodStart = nil
		}},
		{name: "valid with zero price and tax", mutate: func(l *SubscriptionLine) {
			l.UnitPriceCents = 0
			l.TaxCents = 0
			l.LineTotalCents = 0
		}},
		{name: "valid with rounded tax", mutate: func(l *SubscriptionLine) {
			l.Quantity = 1
			l.UnitPriceCents = 5
			l.TaxRateBp = 1250
			l.TaxCents = 1
			l.LineTotalCents = 6
		}},
		// Only the total is checked against the recalculation; the stored tax
		// snapshot is validated for sign but not for equality in this phase.
		{name: "valid when stored tax snapshot differs", mutate: func(l *SubscriptionLine) {
			l.TaxCents = 999
		}},
		{name: "zero line id", mutate: func(l *SubscriptionLine) {
			l.ID = testUUID(0)
		}, wantField: "id"},
		{name: "zero subscription id", mutate: func(l *SubscriptionLine) {
			l.SubscriptionID = testUUID(0)
		}, wantField: "subscription_id"},
		{name: "zero product id", mutate: func(l *SubscriptionLine) {
			zero := testUUID(0)
			l.ProductID = &zero
		}, wantField: "product_id"},
		{name: "zero variant id", mutate: func(l *SubscriptionLine) {
			zero := testUUID(0)
			l.VariantID = &zero
		}, wantField: "variant_id"},
		{name: "blank product name", mutate: func(l *SubscriptionLine) {
			l.ProductName = "   "
		}, wantField: "product_name"},
		{name: "zero quantity", mutate: func(l *SubscriptionLine) {
			l.Quantity = 0
		}, wantField: "quantity"},
		{name: "negative quantity", mutate: func(l *SubscriptionLine) {
			l.Quantity = -1
		}, wantField: "quantity"},
		{name: "negative unit price", mutate: func(l *SubscriptionLine) {
			l.UnitPriceCents = -1
		}, wantField: "unit_price_cents"},
		{name: "negative tax rate", mutate: func(l *SubscriptionLine) {
			l.TaxRateBp = -1
		}, wantField: "tax_rate_bp"},
		{name: "tax rate above maximum", mutate: func(l *SubscriptionLine) {
			l.TaxRateBp = 10001
		}, wantField: "tax_rate_bp"},
		{name: "negative tax", mutate: func(l *SubscriptionLine) {
			l.TaxCents = -1
		}, wantField: "tax_cents"},
		{name: "line total too high", mutate: func(l *SubscriptionLine) {
			l.LineTotalCents = 5501
		}, wantField: "line_total_cents"},
		{name: "line total too low", mutate: func(l *SubscriptionLine) {
			l.LineTotalCents = 5499
		}, wantField: "line_total_cents"},
		{name: "period end before start", mutate: func(l *SubscriptionLine) {
			l.PeriodEnd = &beforeStart
		}, wantField: "period_end"},
		{name: "period end equal to start", mutate: func(l *SubscriptionLine) {
			l.PeriodEnd = &start
		}, wantField: "period_end"},
		{name: "zero period dates", mutate: func(l *SubscriptionLine) {
			l.PeriodStart = &zeroTime
			l.PeriodEnd = &zeroTime
		}, wantField: "period_end"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := validSubscriptionLine()
			tt.mutate(l)

			details := ValidateSubscriptionLine(l)
			if tt.wantField == "" {
				if len(details) != 0 {
					t.Fatalf("ValidateSubscriptionLine() = %v, want no errors", details)
				}
				return
			}
			if details[tt.wantField] == "" {
				t.Fatalf("ValidateSubscriptionLine() = %v, want an error on %q", details, tt.wantField)
			}
			if len(details) != 1 {
				t.Fatalf("ValidateSubscriptionLine() = %v, want exactly one error on %q", details, tt.wantField)
			}
		})
	}
}

func TestValidateSubscriptionLineNil(t *testing.T) {
	details := ValidateSubscriptionLine(nil)
	if details["subscription_line"] == "" {
		t.Fatalf("ValidateSubscriptionLine(nil) = %v, want a subscription line error", details)
	}
}

func TestValidateSubscriptionLineDoesNotMutate(t *testing.T) {
	l := validSubscriptionLine()
	l.ProductName = "  Coffee Beans  "

	_ = ValidateSubscriptionLine(l)

	if l.ProductName != "  Coffee Beans  " {
		t.Fatalf("ProductName mutated to %q", l.ProductName)
	}
}
