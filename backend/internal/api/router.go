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
	"github.com/brightloop/subscription-platform/backend/internal/customers"
	"github.com/brightloop/subscription-platform/backend/internal/platform/config"
	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
	"github.com/brightloop/subscription-platform/backend/internal/users"
)

type Deps struct {
	AuthService      *auth.Service
	AuthHandler      *auth.Handler
	UsersHandler     *users.Handler
	CustomersHandler *customers.Handler
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

		userRoutes := protected.Group("/users")
		{
			userRoutes.GET("", auth.RequirePermission(authz.UsersRead), deps.UsersHandler.List)
			userRoutes.POST("", auth.RequirePermission(authz.UsersWrite), deps.UsersHandler.Create)
			userRoutes.GET("/:id", auth.RequirePermission(authz.UsersRead), deps.UsersHandler.Get)
			userRoutes.PATCH("/:id", auth.RequirePermission(authz.UsersWrite), deps.UsersHandler.Update)
			userRoutes.PUT("/:id/permissions", auth.RequirePermission(authz.UsersWrite), deps.UsersHandler.SetPermissions)
		}
		protected.GET("/permissions", auth.RequirePermission(authz.UsersRead), deps.UsersHandler.Catalog)

		customerRoutes := protected.Group("/customers")
		{
			customerRoutes.GET("", auth.RequirePermission(authz.CustomersRead), deps.CustomersHandler.List)
			customerRoutes.POST("", auth.RequirePermission(authz.CustomersWrite), deps.CustomersHandler.Create)
			customerRoutes.GET("/:id", auth.RequirePermission(authz.CustomersRead), deps.CustomersHandler.Get)
			customerRoutes.PATCH("/:id", auth.RequirePermission(authz.CustomersWrite), deps.CustomersHandler.Update)
			customerRoutes.DELETE("/:id", auth.RequirePermission(authz.CustomersWrite), deps.CustomersHandler.Deactivate)
		}
	}

	router.NoRoute(func(c *gin.Context) {
		httpx.Fail(c, httpx.NotFound("endpoint not found"))
	})

	return router
}
