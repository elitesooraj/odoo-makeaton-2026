package subscriptions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

const (
	StatusDraft     = "DRAFT"
	StatusQuotation = "QUOTATION"
	StatusConfirmed = "CONFIRMED"
	StatusActive    = "ACTIVE"
	StatusPaused    = "PAUSED"
	StatusClosed    = "CLOSED"
)

var ErrNotFound = errors.New("subscription not found")

// PlanFlags carries the plan properties the state machine needs: line seeding,
// billing cadence and which transitions the plan allows.
type PlanFlags struct {
	ID              uuid.UUID
	ProductID       uuid.UUID
	TaxID           *uuid.UUID
	Name            string
	Interval        string
	PriceCents      int64
	MinimumQuantity int
	AutoClose       bool
	AutoRenew       bool
	AllowPause      bool
	AllowEarlyClose bool
	IsActive        bool
}

// PlanLoader is satisfied by plans.Service.
type PlanLoader interface {
	Get(ctx context.Context, planID uuid.UUID) (*PlanSpec, error)
}

// PlanSpec is the neutral plan view this package consumes, avoiding an import
// cycle with the plans package.
type PlanSpec struct {
	ID              uuid.UUID  `json:"id"`
	ProductID       uuid.UUID  `json:"product_id"`
	TaxID           *uuid.UUID `json:"tax_id"`
	Name            string     `json:"name"`
	Interval        string     `json:"interval"`
	PriceCents      int64      `json:"price_cents"`
	MinimumQuantity int        `json:"minimum_quantity"`
	AutoClose       bool       `json:"auto_close"`
	AutoRenew       bool       `json:"auto_renew"`
	AllowPause      bool       `json:"allow_pause"`
	AllowEarlyClose bool       `json:"allow_early_close"`
	IsActive        bool       `json:"is_active"`
}

type Subscription struct {
	ID               uuid.UUID  `json:"id"`
	Number           string     `json:"number"`
	CustomerID       uuid.UUID  `json:"customer_id"`
	PlanID           uuid.UUID  `json:"plan_id"`
	Status           string     `json:"status"`
	StartDate        time.Time  `json:"start_date"`
	ExpiryDate       *time.Time `json:"expiry_date"`
	PaymentTermsDays int        `json:"payment_terms_days"`
	Quantity         int        `json:"quantity"`
	DiscountID       *uuid.UUID `json:"discount_id"`
	NextBillingDate  *time.Time `json:"next_billing_date"`
	QuoteExpiresAt   *time.Time `json:"quote_expires_at"`
	Notes            *string    `json:"notes"`
	CreatedBy        *uuid.UUID `json:"created_by"`
	ActivatedAt      *time.Time `json:"activated_at"`
	PausedAt         *time.Time `json:"paused_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	Lines            []Line     `json:"lines,omitempty"`
	// Plan is populated by ListForBilling (JOIN) so the billing worker does
	// not need a separate PlanFlagsByID round-trip per subscription.
	Plan      *PlanSpec `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Line struct {
	ID             uuid.UUID  `json:"id"`
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	ProductID      uuid.UUID  `json:"product_id"`
	VariantID      *uuid.UUID `json:"variant_id"`
	Description    *string    `json:"description"`
	Quantity       int        `json:"quantity"`
	UnitPriceCents int64      `json:"unit_price_cents"`
	TaxID          *uuid.UUID `json:"tax_id"`
	Position       int        `json:"position"`
}

type Filter struct {
	CustomerID *uuid.UUID
	Status     string
	Search     string
	Page       pagination.Page
}

type LineInput struct {
	ProductID      string  `json:"product_id"`
	VariantID      *string `json:"variant_id"`
	Description    string  `json:"description"`
	Quantity       int     `json:"quantity"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	TaxID          *string `json:"tax_id"`
}

type CreateInput struct {
	CustomerID       string      `json:"customer_id"`
	PlanID           string      `json:"plan_id"`
	Status           string      `json:"status"`
	StartDate        string      `json:"start_date"`
	ExpiryDate       *string     `json:"expiry_date"`
	PaymentTermsDays int         `json:"payment_terms_days"`
	Quantity         int         `json:"quantity"`
	DiscountID       *string     `json:"discount_id"`
	QuoteExpiresAt   *string     `json:"quote_expires_at"`
	Notes            string      `json:"notes"`
	Lines            []LineInput `json:"lines"`
}

type UpdateInput struct {
	StartDate        string  `json:"start_date"`
	ExpiryDate       *string `json:"expiry_date"`
	PaymentTermsDays *int    `json:"payment_terms_days"`
	Quantity         *int    `json:"quantity"`
	DiscountID       *string `json:"discount_id"`
	NextBillingDate  *string `json:"next_billing_date"`
	QuoteExpiresAt   *string `json:"quote_expires_at"`
	Notes            string  `json:"notes"`
}
