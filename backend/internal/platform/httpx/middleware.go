package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const HeaderRequestID = "X-Request-ID"

// RequestIDMiddleware accepts an upstream request ID or generates one, echoes
// it in the response header and stores it for the envelope and audit log.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set(requestIDKey, requestID)
		c.Writer.Header().Set(HeaderRequestID, requestID)
		c.Next()
	}
}

// LoggerMiddleware writes one structured line per request.
func LoggerMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestID(c),
			"ip", c.ClientIP(),
		}
		if v, ok := c.Get("actor_user_id"); ok {
			attrs = append(attrs, "user_id", v)
		}

		if c.Writer.Status() >= http.StatusInternalServerError {
			log.Error("http_request", attrs...)
			return
		}
		log.Info("http_request", attrs...)
	}
}

// RecoveryMiddleware converts a panic into the standard 500 envelope.
func RecoveryMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered",
					"panic", fmt.Sprint(recovered),
					"path", c.Request.URL.Path,
					"request_id", RequestID(c),
				)
				Fail(c, Internal(fmt.Errorf("panic: %v", recovered)))
			}
		}()
		c.Next()
	}
}

// CORSMiddleware allows credentialed requests from the configured origins.
// It deliberately does not use a wildcard: cookies require an exact origin.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := toSet(allowedOrigins)
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; origin != "" && ok {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// OriginCheckMiddleware is the CSRF defence for cookie-authenticated writes.
// SameSite=Lax already blocks most cross-site writes; this rejects any request
// that arrives with a foreign Origin. Requests without an Origin header
// (curl, server-to-server) are allowed.
func OriginCheckMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowed := toSet(allowedOrigins)
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		if origin := c.GetHeader("Origin"); origin != "" {
			if _, ok := allowed[origin]; !ok {
				Fail(c, Forbidden("request origin is not allowed"))
				return
			}
		}
		c.Next()
	}
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}
