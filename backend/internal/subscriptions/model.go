package subscriptions

import (
	"time"

	"github.com/google/uuid"
)

// Subscription is a customer's enrollment in a plan for a billing period.
// Money is not stored on the header; it is derived from the subscription lines.
type Subscription struct {
	ID         uuid.UUID  `json:"id"`
	Number     string     `json:"number"`
	CustomerID uuid.UUID  `json:"customer_id"`
	PlanID     uuid.UUID  `json:"plan_id"`
	Status     Status     `json:"status"`
	StartDate  time.Time  `json:"start_date"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// SubscriptionLine is one priced item on a subscription. ProductName and
// VariantName are snapshots so historical invoices do not change when the
// catalog changes. All monetary values are integer minor units (cents).
type SubscriptionLine struct {
	ID             uuid.UUID  `json:"id"`
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	ProductID      *uuid.UUID `json:"product_id,omitempty"`
	VariantID      *uuid.UUID `json:"variant_id,omitempty"`
	ProductName    string     `json:"product_name"`
	VariantName    *string    `json:"variant_name,omitempty"`
	Quantity       int32      `json:"quantity"`
	UnitPriceCents int64      `json:"unit_price_cents"`
	TaxRateBp      int32      `json:"tax_rate_bp"`
	TaxCents       int64      `json:"tax_cents"`
	LineTotalCents int64      `json:"line_total_cents"`
	PeriodStart    *time.Time `json:"period_start,omitempty"`
	PeriodEnd      *time.Time `json:"period_end,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
