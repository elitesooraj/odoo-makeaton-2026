package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brightloop/subscription-platform/backend/internal/api"
	"github.com/brightloop/subscription-platform/backend/internal/audit"
	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/billing"
	"github.com/brightloop/subscription-platform/backend/internal/customers"
	"github.com/brightloop/subscription-platform/backend/internal/discounts"
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/payments"
	"github.com/brightloop/subscription-platform/backend/internal/plans"
	"github.com/brightloop/subscription-platform/backend/internal/platform/config"
	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/mailer"
	"github.com/brightloop/subscription-platform/backend/internal/portal"
	"github.com/brightloop/subscription-platform/backend/internal/products"
	"github.com/brightloop/subscription-platform/backend/internal/quotations"
	"github.com/brightloop/subscription-platform/backend/internal/reports"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
	"github.com/brightloop/subscription-platform/backend/internal/taxes"
	"github.com/brightloop/subscription-platform/backend/internal/users"
	"github.com/google/uuid"
)

// planAdapter and taxAdapter convert package-specific plan/tax views into the
// neutral interfaces the subscriptions and invoices services consume.
type planAdapter struct{ svc *plans.Service }

func (a planAdapter) Get(ctx context.Context, planID uuid.UUID) (*subscriptions.PlanSpec, error) {
	p, err := a.svc.Get(ctx, planID)
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

type taxAdapter struct{ svc *taxes.Service }

func (a taxAdapter) Get(ctx context.Context, taxID uuid.UUID) (*invoices.TaxDetail, error) {
	t, err := a.svc.Get(ctx, taxID)
	if err != nil {
		return nil, err
	}
	return &invoices.TaxDetail{RateBP: t.RateBP, IsInclusive: t.IsInclusive}, nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

// Set at build time with -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "unknown"
)

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	mail := mailer.New(mailer.SMTPConfig{
		Host:     cfg.SMTP.Host,
		Port:     cfg.SMTP.Port,
		Username: cfg.SMTP.Username,
		Password: cfg.SMTP.Password,
		From:     cfg.SMTP.From,
	}, logger)

	recorder := audit.NewRecorder(pool)

	customerRepo := customers.NewRepository(pool)
	customersSvc := customers.NewService(customerRepo, recorder)

	authRepo := auth.NewRepository(pool)
	authSvc := auth.NewService(authRepo, customersSvc, mail, cfg.AppBaseURL, cfg.SessionTTL)

	usersSvc := users.NewService(users.NewRepository(pool), authRepo, recorder)

	// Business modules.
	productsSvc := products.NewService(products.NewRepository(pool))
	productHandler := products.NewHandler(productsSvc)

	plansSvc := plans.NewService(plans.NewRepository(pool), productsSvc)
	planHandler := plans.NewHandler(plansSvc)

	taxesSvc := taxes.NewService(taxes.NewRepository(pool))
	taxHandler := taxes.NewHandler(taxesSvc)

	discountsSvc := discounts.NewService(discounts.NewRepository(pool))
	discountHandler := discounts.NewHandler(discountsSvc)

	subscriptionsSvc := subscriptions.NewService(pool, recorder, planAdapter{plansSvc})
	subscriptionHandler := subscriptions.NewHandler(subscriptionsSvc)

	invoicesSvc := invoices.NewService(pool, recorder, subscriptionsSvc, discountsSvc, taxAdapter{taxesSvc})
	invoiceHandler := invoices.NewHandler(invoicesSvc)

	paymentsSvc := payments.NewService(payments.NewRepository(pool), invoicesSvc.Repo(), recorder)
	paymentHandler := payments.NewHandler(paymentsSvc)

	quotationsSvc := quotations.NewService(pool, recorder, subscriptionsSvc, planAdapter{plansSvc})
	quotationHandler := quotations.NewHandler(quotationsSvc)

	billingSvc := billing.NewService(pool, subscriptionsSvc, invoicesSvc, recorder)
	billingHandler := billing.NewHandler(billingSvc)

	reportsSvc := reports.NewService(pool)
	reportHandler := reports.NewHandler(reportsSvc)

	portalSvc := portal.NewService(pool, subscriptionsSvc, invoicesSvc, paymentsSvc)
	portalHandler := portal.NewHandler(portalSvc)

	router := api.NewRouter(cfg, pool, api.Deps{
		AuthService:          authSvc,
		AuthHandler:          auth.NewHandler(authSvc, cfg.SessionCookieName, cfg.SessionTTL, cfg.SecureCookies),
		UsersHandler:         users.NewHandler(usersSvc),
		CustomersHandler:     customers.NewHandler(customersSvc),
		ProductsHandler:      productHandler,
		PlansHandler:         planHandler,
		TaxesHandler:         taxHandler,
		DiscountsHandler:     discountHandler,
		SubscriptionsHandler: subscriptionHandler,
		InvoicesHandler:      invoiceHandler,
		PaymentsHandler:      paymentHandler,
		QuotationsHandler:    quotationHandler,
		BillingHandler:       billingHandler,
		ReportsHandler:       reportHandler,
		PortalHandler:        portalHandler,
	})

	server := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("api listening", "port", cfg.HTTPPort, "env", cfg.Env, "version", version, "commit", commit)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.Info("api stopped")
	return nil
}
