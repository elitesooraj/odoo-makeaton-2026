package subscriptions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// testUUID builds a deterministic, non-zero UUID for tests. Passing 0 yields
// uuid.Nil.
func testUUID(n byte) uuid.UUID {
	var id uuid.UUID
	id[15] = n
	return id
}

func testTime(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestZeroSubscriptionHasInvalidStatus(t *testing.T) {
	var s Subscription
	if s.Status.Valid() {
		t.Fatalf("zero Subscription Status = %q, want an invalid status", s.Status)
	}
}

func TestSubscriptionJSONShape(t *testing.T) {
	s := Subscription{
		ID:         testUUID(1),
		Number:     "SUB-2026-0001",
		CustomerID: testUUID(2),
		PlanID:     testUUID(3),
		Status:     StatusDraft,
		StartDate:  testTime(2026, time.January, 1),
		CreatedAt:  testTime(2026, time.January, 1),
		UpdatedAt:  testTime(2026, time.January, 1),
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal(Subscription) error = %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(Subscription) error = %v", err)
	}

	for _, key := range []string{
		"id", "number", "customer_id", "plan_id", "status",
		"start_date", "created_at", "updated_at",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("marshalled Subscription missing key %q", key)
		}
	}
	for _, key := range []string{"end_date", "notes"} {
		if _, ok := got[key]; ok {
			t.Errorf("marshalled Subscription has unexpected optional key %q", key)
		}
	}
}

func TestSubscriptionLineJSONShape(t *testing.T) {
	productID := testUUID(4)
	variantID := testUUID(5)
	variantName := "Large"
	start := testTime(2026, time.January, 1)
	end := testTime(2026, time.February, 1)

	line := SubscriptionLine{
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

	data, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("json.Marshal(SubscriptionLine) error = %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(SubscriptionLine) error = %v", err)
	}

	for _, key := range []string{
		"id", "subscription_id", "product_id", "variant_id", "product_name",
		"variant_name", "quantity", "unit_price_cents", "tax_rate_bp",
		"tax_cents", "line_total_cents", "period_start", "period_end",
		"created_at",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("marshalled SubscriptionLine missing key %q", key)
		}
	}
}

func TestSubscriptionLineJSONOmitsOptionalFields(t *testing.T) {
	line := SubscriptionLine{
		ID:             testUUID(6),
		SubscriptionID: testUUID(7),
		ProductName:    "Coffee Beans",
		Quantity:       1,
		UnitPriceCents: 100,
	}

	data, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("json.Marshal(SubscriptionLine) error = %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal(SubscriptionLine) error = %v", err)
	}

	for _, key := range []string{
		"product_id", "variant_id", "variant_name", "period_start", "period_end",
	} {
		if _, ok := got[key]; ok {
			t.Errorf("marshalled SubscriptionLine has unexpected optional key %q", key)
		}
	}
}
