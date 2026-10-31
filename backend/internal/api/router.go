// Package api composes the HTTP surface. Domain modules own their handlers;
// this package only mounts them under the versioned prefix with the shared
// middleware chain.
package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/auth"
	"github.com/brightloop/subscription-platform/backend/internal/authz"
	"github.com/brightloop/subscription-platform/backend/internal/billing"
	"github.com/brightloop/subscription-platform/backend/internal/customers"
	"github.com/brightloop/subscription-platform/backend/internal/discounts"
	"github.com/brightloop/subscription-platform/backend/internal/invoices"
	"github.com/brightloop/subscription-platform/backend/internal/payments"
	"github.com/brightloop/subscription-platform/backend/internal/plans"
	"github.com/brightloop/subscription-platform/backend/internal/platform/config"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/portal"
	"github.com/brightloop/subscription-platform/backend/internal/products"
	"github.com/brightloop/subscription-platform/backend/internal/quotations"
	"github.com/brightloop/subscription-platform/backend/internal/reports"
	"github.com/brightloop/subscription-platform/backend/internal/subscriptions"
	"github.com/brightloop/subscription-platform/backend/internal/taxes"
	"github.com/brightloop/subscription-platform/backend/internal/users"
)

type Deps struct {
	AuthHandler          *auth.Handler
	AuthService          *auth.Service
	UsersHandler         *users.Handler
	CustomersHandler     *customers.Handler
	ProductsHandler      *products.Handler
	PlansHandler         *plans.Handler
	TaxesHandler         *taxes.Handler
	DiscountsHandler     *discounts.Handler
	SubscriptionsHandler *subscriptions.Handler
	InvoicesHandler      *invoices.Handler
	PaymentsHandler      *payments.Handler
	QuotationsHandler    *quotations.Handler
	BillingHandler       *billing.Handler
	ReportsHandler       *reports.Handler
	PortalHandler        *portal.Handler
}

func NewRouter(cfg config.Config, pool *pgxpool.Pool, deps Deps) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	log := slog.Default()
	router := gin.New()
	router.Use(
		httpx.RequestIDMiddleware(),
		httpx.LoggerMiddleware(log),
		httpx.RecoveryMiddleware(log),
		httpx.CORSMiddleware(cfg.CORSAllowedOrigins),
		httpx.OriginCheckMiddleware(cfg.CORSAllowedOrigins),
	)
	// Trust no proxy headers by default: ClientIP is the socket address.
	// Set real proxies here when the API runs behind a load balancer.
	_ = router.SetTrustedProxies(nil)

	router.GET("/healthz", func(c *gin.Context) {
		httpx.OK(c, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.Fail(c, httpx.Internal(err))
			return
		}
		httpx.OK(c, gin.H{"status": "ready"})
	})

	v1 := router.Group("/api/v1")

	// Public: registration, login, password reset and email verification.
	authGroup := v1.Group("/auth")
	{
		authGroup.POST("/register", deps.AuthHandler.Register)
		authGroup.POST("/login", deps.AuthHandler.Login)
		authGroup.POST("/logout", deps.AuthHandler.Logout)
		authGroup.POST("/forgot-password", deps.AuthHandler.ForgotPassword)
		authGroup.POST("/reset-password", deps.AuthHandler.ResetPassword)
		authGroup.POST("/verify-email", deps.AuthHandler.VerifyEmail)
	}

	// Everything below requires a valid session cookie.
	protected := v1.Group("", auth.RequireAuth(deps.AuthService, cfg.SessionCookieName))
	{
		protected.GET("/auth/me", deps.AuthHandler.Me)
		protected.POST("/auth/change-password", deps.AuthHandler.ChangePassword)
		protected.POST("/auth/resend-verification", deps.AuthHandler.ResendVerification)
		protected.GET("/permissions", auth.RequirePermission(authz.UsersRead), deps.UsersHandler.Catalog)

		userRoutes := protected.Group("/users")
		{
			userRoutes.GET("", auth.RequirePermission(authz.UsersRead), deps.UsersHandler.List)
			userRoutes.POST("", auth.RequirePermission(authz.UsersWrite), deps.UsersHandler.Create)
			userRoutes.GET("/:id", auth.RequirePermission(authz.UsersRead), deps.UsersHandler.Get)
			userRoutes.PATCH("/:id", auth.RequirePermission(authz.UsersWrite), deps.UsersHandler.Update)
			userRoutes.PUT("/:id/permissions", auth.RequirePermission(authz.UsersWrite), deps.UsersHandler.SetPermissions)
		}

		customerRoutes := protected.Group("/customers")
		{
			customerRoutes.GET("", auth.RequirePermission(authz.CustomersRead), deps.CustomersHandler.List)
			customerRoutes.POST("", auth.RequirePermission(authz.CustomersWrite), deps.CustomersHandler.Create)
			customerRoutes.GET("/:id", auth.RequirePermission(authz.CustomersRead), deps.CustomersHandler.Get)
			customerRoutes.PATCH("/:id", auth.RequirePermission(authz.CustomersWrite), deps.CustomersHandler.Update)
			customerRoutes.DELETE("/:id", auth.RequirePermission(authz.CustomersWrite), deps.CustomersHandler.Deactivate)
		}

		productRoutes := protected.Group("/products")
		{
			productRoutes.GET("", auth.RequirePermission(authz.ProductsRead), deps.ProductsHandler.List)
			productRoutes.POST("", auth.RequirePermission(authz.ProductsWrite), deps.ProductsHandler.Create)
			productRoutes.GET("/:id", auth.RequirePermission(authz.ProductsRead), deps.ProductsHandler.Get)
			productRoutes.PATCH("/:id", auth.RequirePermission(authz.ProductsWrite), deps.ProductsHandler.Update)
			productRoutes.DELETE("/:id", auth.RequirePermission(authz.ProductsWrite), deps.ProductsHandler.Deactivate)
			productRoutes.POST("/:id/variants", auth.RequirePermission(authz.ProductsWrite), deps.ProductsHandler.AddVariant)
			productRoutes.PATCH("/:id/variants/:variant_id", auth.RequirePermission(authz.ProductsWrite), deps.ProductsHandler.UpdateVariant)
			productRoutes.DELETE("/:id/variants/:variant_id", auth.RequirePermission(authz.ProductsWrite), deps.ProductsHandler.DeactivateVariant)
		}

		planRoutes := protected.Group("/plans")
		{
			planRoutes.GET("", auth.RequirePermission(authz.PlansRead), deps.PlansHandler.List)
			planRoutes.POST("", auth.RequirePermission(authz.PlansWrite), deps.PlansHandler.Create)
			planRoutes.GET("/:id", auth.RequirePermission(authz.PlansRead), deps.PlansHandler.Get)
			planRoutes.PATCH("/:id", auth.RequirePermission(authz.PlansWrite), deps.PlansHandler.Update)
			planRoutes.DELETE("/:id", auth.RequirePermission(authz.PlansWrite), deps.PlansHandler.Deactivate)
		}

		taxRoutes := protected.Group("/taxes")
		{
			taxRoutes.GET("", auth.RequirePermission(authz.TaxesRead), deps.TaxesHandler.List)
			taxRoutes.POST("", auth.RequirePermission(authz.TaxesWrite), deps.TaxesHandler.Create)
			taxRoutes.GET("/:id", auth.RequirePermission(authz.TaxesRead), deps.TaxesHandler.Get)
			taxRoutes.PATCH("/:id", auth.RequirePermission(authz.TaxesWrite), deps.TaxesHandler.Update)
			taxRoutes.DELETE("/:id", auth.RequirePermission(authz.TaxesWrite), deps.TaxesHandler.Deactivate)
		}

		discountRoutes := protected.Group("/discounts")
		{
			discountRoutes.GET("", auth.RequirePermission(authz.DiscountsRead), deps.DiscountsHandler.List)
			discountRoutes.POST("", auth.RequirePermission(authz.DiscountsWrite), deps.DiscountsHandler.Create)
			discountRoutes.GET("/:id", auth.RequirePermission(authz.DiscountsRead), deps.DiscountsHandler.Get)
			discountRoutes.PATCH("/:id", auth.RequirePermission(authz.DiscountsWrite), deps.DiscountsHandler.Update)
			discountRoutes.DELETE("/:id", auth.RequirePermission(authz.DiscountsWrite), deps.DiscountsHandler.Deactivate)
		}

		subscriptionRoutes := protected.Group("/subscriptions")
		{
			subscriptionRoutes.GET("", auth.RequirePermission(authz.SubscriptionsRead), deps.SubscriptionsHandler.List)
			subscriptionRoutes.POST("", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Create)
			subscriptionRoutes.GET("/:id", auth.RequirePermission(authz.SubscriptionsRead), deps.SubscriptionsHandler.Get)
			subscriptionRoutes.PATCH("/:id", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Update)
			subscriptionRoutes.POST("/:id/confirm", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Confirm)
			subscriptionRoutes.POST("/:id/activate", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Activate)
			subscriptionRoutes.POST("/:id/pause", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Pause)
			subscriptionRoutes.POST("/:id/resume", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Resume)
			subscriptionRoutes.POST("/:id/close", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.Close)
			subscriptionRoutes.POST("/:id/lines", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.AddLine)
			subscriptionRoutes.PATCH("/:id/lines/:line_id", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.UpdateLine)
			subscriptionRoutes.DELETE("/:id/lines/:line_id", auth.RequirePermission(authz.SubscriptionsWrite), deps.SubscriptionsHandler.DeleteLine)
		}

		invoiceRoutes := protected.Group("/invoices")
		{
			invoiceRoutes.GET("", auth.RequirePermission(authz.InvoicesRead), deps.InvoicesHandler.List)
			invoiceRoutes.POST("", auth.RequirePermission(authz.InvoicesWrite), deps.InvoicesHandler.Create)
			invoiceRoutes.GET("/:id", auth.RequirePermission(authz.InvoicesRead), deps.InvoicesHandler.Get)
			invoiceRoutes.PATCH("/:id", auth.RequirePermission(authz.InvoicesWrite), deps.InvoicesHandler.Update)
			invoiceRoutes.POST("/:id/confirm", auth.RequirePermission(authz.InvoicesWrite), deps.InvoicesHandler.Confirm)
			invoiceRoutes.POST("/:id/cancel", auth.RequirePermission(authz.InvoicesWrite), deps.InvoicesHandler.Cancel)
			invoiceRoutes.POST("/:id/send", auth.RequirePermission(authz.InvoicesWrite), deps.InvoicesHandler.Send)
		}

		paymentRoutes := protected.Group("/payments")
		{
			paymentRoutes.GET("", auth.RequirePermission(authz.PaymentsRead), deps.PaymentsHandler.List)
			paymentRoutes.POST("", auth.RequirePermission(authz.PaymentsWrite), deps.PaymentsHandler.Create)
			paymentRoutes.GET("/:id", auth.RequirePermission(authz.PaymentsRead), deps.PaymentsHandler.Get)
		}

		quotationRoutes := protected.Group("/quotations/templates")
		{
			quotationRoutes.GET("", auth.RequirePermission(authz.SubscriptionsRead), deps.QuotationsHandler.List)
			quotationRoutes.POST("", auth.RequirePermission(authz.SubscriptionsWrite), deps.QuotationsHandler.Create)
			quotationRoutes.GET("/:id", auth.RequirePermission(authz.SubscriptionsRead), deps.QuotationsHandler.Get)
			quotationRoutes.PATCH("/:id", auth.RequirePermission(authz.SubscriptionsWrite), deps.QuotationsHandler.Update)
			quotationRoutes.DELETE("/:id", auth.RequirePermission(authz.SubscriptionsWrite), deps.QuotationsHandler.Deactivate)
			quotationRoutes.POST("/:id/instantiate", auth.RequirePermission(authz.SubscriptionsWrite), deps.QuotationsHandler.Instantiate)
		}

		protected.POST("/billing/run", auth.RequirePermission(authz.SubscriptionsWrite), deps.BillingHandler.Run)

		reportRoutes := protected.Group("/reports")
		{
			reportRoutes.GET("/summary", auth.RequirePermission(authz.ReportsRead), deps.ReportsHandler.Summary)
			reportRoutes.GET("/revenue", auth.RequirePermission(authz.ReportsRead), deps.ReportsHandler.Revenue)
			reportRoutes.GET("/subscriptions", auth.RequirePermission(authz.ReportsRead), deps.ReportsHandler.SubscriptionsByStatus)
			reportRoutes.GET("/overdue", auth.RequirePermission(authz.ReportsRead), deps.ReportsHandler.Overdue)
		}

		// Customer portal: identity-scoped, role-checked, read-only.
		portalRoutes := protected.Group("/portal", auth.RequireRole(authz.RoleCustomer))
		{
			portalRoutes.GET("/me", deps.PortalHandler.Me)
			portalRoutes.GET("/subscriptions", deps.PortalHandler.Subscriptions)
			portalRoutes.GET("/subscriptions/:id", deps.PortalHandler.Subscription)
			portalRoutes.GET("/invoices", deps.PortalHandler.Invoices)
			portalRoutes.GET("/invoices/:id", deps.PortalHandler.Invoice)
			portalRoutes.GET("/payments", deps.PortalHandler.Payments)
		}
	}

	router.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.NotFound("endpoint not found"))
	})

	return router
}
