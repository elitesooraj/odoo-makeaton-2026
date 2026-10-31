package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/brightloop/subscription-platform/backend/internal/authz"
)

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name  string
		actor *Actor
		want  int
	}{
		{
			name: "staff with granted permission",
			actor: &Actor{Role: authz.RoleStaff, Permissions: map[string]bool{
				authz.CustomersRead: true,
			}},
			want: http.StatusOK,
		},
		{
			name:  "staff without permission",
			actor: &Actor{Role: authz.RoleStaff, Permissions: map[string]bool{}},
			want:  http.StatusForbidden,
		},
		{
			name:  "admin bypasses the permission table",
			actor: &Actor{Role: authz.RoleAdmin, Permissions: map[string]bool{}},
			want:  http.StatusOK,
		},
		{
			name:  "customer has no back-office access",
			actor: &Actor{Role: authz.RoleCustomer, Permissions: map[string]bool{}},
			want:  http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(actorContextKey, tt.actor)
				c.Next()
			})
			router.GET("/probe", RequirePermission(authz.CustomersRead), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/probe", nil)
			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestRequireAuthWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/probe", RequireAuth(&Service{}, "bl_session"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/probe", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
