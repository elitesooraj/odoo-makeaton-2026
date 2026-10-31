package invoices

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/platform/pagination"
)

const (
	StatusDraft     = "DRAFT"
	StatusConfirmed = "CONFIRMED"
	StatusPaid      = "PAID"
	StatusCancelled = "CANCELLED"
)

var ErrNotFound = errors.New("invoice not found")

type Invoice struct {
	ID             uuid.UUID  `json:"id"`
	Number         string     `json:"number"`
	CustomerID     uuid.UUID  `json:"customer_id"`
	SubscriptionID *uuid.UUID `json:"subscription_id"`
	Status         string     `json:"status"`
	IssueDate      time.Time  `json:"issue_date"`
	DueDate        time.Time  `json:"due_date"`
	PeriodStart    *time.Time `json:"period_start"`
	PeriodEnd      *time.Time `json:"period_end"`
	SubtotalCents  int64      `json:"subtotal_cents"`
	DiscountCents  int64      `json:"discount_cents"`
	TaxCents       int64      `json:"tax_cents"`
	TotalCents     int64      `json:"total_cents"`
	BalanceCents   int64      `json:"balance_cents"`
	Notes          *string    `json:"notes"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	SentAt         *time.Time `json:"sent_at"`
	PaidAt         *time.Time `json:"paid_at"`
	CancelledAt    *time.Time `json:"cancelled_at"`
	CreatedBy      *uuid.UUID `json:"created_by"`
	Lines          []Line     `json:"lines,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Line struct {
	ID             uuid.UUID  `json:"id"`
	InvoiceID      uuid.UUID  `json:"invoice_id"`
	ProductID      *uuid.UUID `json:"product_id"`
	VariantID      *uuid.UUID `json:"variant_id"`
	Description    string     `json:"description"`
	Quantity       int        `json:"quantity"`
	UnitPriceCents int64      `json:"unit_price_cents"`
	TaxRateBP      int        `json:"tax_rate_bp"`
	TaxCents       int64      `json:"tax_cents"`
	DiscountCents  int64      `json:"discount_cents"`
	LineTotalCents int64      `json:"line_total_cents"`
	Position       int        `json:"position"`
}

type Filter struct {
	CustomerID     *uuid.UUID
	SubscriptionID *uuid.UUID
	Status         string
	Search         string
	Overdue        bool
	Page           pagination.Page
}

type ErrTransition struct{ to string }

type TaxDetail struct {
	RateBP      int
	IsInclusive bool
}

type TaxLoader interface {
	Get(ctx context.Context, taxID uuid.UUID) (*TaxDetail, error)
}

type LineInput struct {
	ProductID      *string `json:"product_id"`
	VariantID      *string `json:"variant_id"`
	Description    string  `json:"description"`
	Quantity       int     `json:"quantity"`
	UnitPriceCents int64   `json:"unit_price_cents"`
	TaxID          *string `json:"tax_id"`
}

type CreateInput struct {
	CustomerID     string      `json:"customer_id"`
	SubscriptionID *string     `json:"subscription_id"`
	IssueDate      *string     `json:"issue_date"`
	DueDate        *string     `json:"due_date"`
	PeriodStart    *string     `json:"period_start"`
	PeriodEnd      *string     `json:"period_end"`
	DiscountID     *string     `json:"discount_id"`
	Notes          string      `json:"notes"`
	Lines          []LineInput `json:"lines"`
}

type UpdateInput struct {
	DueDate     *string      `json:"due_date"`
	Notes       string       `json:"notes"`
	PeriodStart *string      `json:"period_start"`
	PeriodEnd   *string      `json:"period_end"`
	Lines       *[]LineInput `json:"lines"`
}

type draftLine struct {
	Line
	taxInclusive bool
}

func (e ErrTransition) Error() string { return "transition to " + e.to + " is not allowed" }
