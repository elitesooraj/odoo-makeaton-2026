package auth

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/brightloop/subscription-platform/backend/internal/platform/httpx"
)

const actorContextKey = "auth.actor"

// RequireAuth validates the session cookie and attaches the Actor.
func RequireAuth(svc *Service, cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(cookieName)
		if err != nil || raw == "" {
			httpx.Fail(c, httpx.Unauthenticated("authentication required"))
			return
		}

		actor, err := svc.Authenticate(c.Request.Context(), raw)
		if err != nil {
			if errors.Is(err, ErrUnauthenticated) {
				httpx.Fail(c, httpx.Unauthenticated("authentication required"))
				return
			}
			httpx.Fail(c, err)
			return
		}

		c.Set(actorContextKey, actor)
		c.Set("actor_user_id", actor.UserID.String())
		c.Next()
	}
}

// RequirePermission enforces one catalog permission. Admin passes implicitly
// through Actor.Can.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := CurrentActor(c)
		if !ok {
			httpx.Fail(c, httpx.Unauthenticated("authentication required"))
			return
		}
		if !actor.Can(permission) {
			httpx.Fail(c, httpx.Forbidden("your account does not have permission to do this"))
			return
		}
		c.Next()
	}
}

// RequireRole is reserved for the customer portal, which is scoped by identity
// rather than by permission.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := CurrentActor(c)
		if !ok {
			httpx.Fail(c, httpx.Unauthenticated("authentication required"))
			return
		}
		if actor.Role != role {
			httpx.Fail(c, httpx.Forbidden("this endpoint is not available for your account type"))
			return
		}
		c.Next()
	}
}

func CurrentActor(c *gin.Context) (*Actor, bool) {
	v, ok := c.Get(actorContextKey)
	if !ok {
		return nil, false
	}
	actor, ok := v.(*Actor)
	return actor, ok
}
