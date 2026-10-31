// Package api glue: adapters that convert package-specific plan and tax views
// into the neutral interfaces the subscriptions, quotations and invoices
// services consume. They live here (not in cmd/api) so the HTTP entrypoint
// and the integration test harness wire the exact same adapters.
package api

import (
	"context"

	"github.com/google/uuid"

	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/plans"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
	"github.com/brightloop/subscription-platform/backend/internal/taxes"
)

// PlanAdapter satisfies subscriptions.PlanLoader and quotations.PlanLoader by
// delegating to the plans service and mapping to the neutral PlanSpec view.
type PlanAdapter struct{ Svc *plans.Service }

func NewPlanAdapter(svc *plans.Service) *PlanAdapter { return &PlanAdapter{Svc: svc} }

func (a *PlanAdapter) Get(ctx context.Context, planID uuid.UUID) (*subscriptions.PlanSpec, error) {
	p, err := a.Svc.Get(ctx, planID)
	if err != nil {
		return nil, err
	}
	return &subscriptions.PlanSpec{
		ID:              p.ID,
		ProductID:       p.ProductID,
		TaxID:           p.TaxID,
		Name:            p.Name,
		Interval:        p.Interval,
		PriceCents:      p.PriceCents,
		MinimumQuantity: p.MinimumQuantity,
		AutoClose:       p.AutoClose,
		AutoRenew:       p.AutoRenew,
		AllowPause:      p.AllowPause,
		AllowEarlyClose: p.AllowEarlyClose,
		IsActive:        p.IsActive,
	}, nil
}

// TaxAdapter satisfies invoices.TaxLoader by delegating to the taxes service.
type TaxAdapter struct{ Svc *taxes.Service }

func NewTaxAdapter(svc *taxes.Service) *TaxAdapter { return &TaxAdapter{Svc: svc} }

func (a *TaxAdapter) Get(ctx context.Context, taxID uuid.UUID) (*invoices.TaxDetail, error) {
	t, err := a.Svc.Get(ctx, taxID)
	if err != nil {
		return nil, err
	}
	return &invoices.TaxDetail{RateBP: t.RateBP, IsInclusive: t.IsInclusive}, nil
}
