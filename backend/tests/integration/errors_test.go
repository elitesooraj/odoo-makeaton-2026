package integration

import (
	"bytes"
	"net/http"
	"testing"
)

// TestErrorEnvelopeContract locks the error contract from
// docs/CONVENTIONS.md across the four most common failure modes, so a
// regression in httpx (wrong code, missing details, wrong status) is caught
// immediately rather than by a frontend developer.
func TestErrorEnvelopeContract(t *testing.T) {
	reset(t)

	// 401 UNAUTHENTICATED: no cookie at all.
	t.Run("unauthenticated request", func(t *testing.T) {
		c := newClient(t)
		env := c.mustStatus(http.MethodGet, "/api/v1/products", nil, http.StatusUnauthorized)
		if env.Error == nil || env.Error.Code != "UNAUTHENTICATED" {
			t.Fatalf("expected 401 UNAUTHENTICATED, got: %s", env.Error)
		}
		if env.Data != nil {
			t.Errorf("error response must not carry data: %s", env.Data)
		}
	})

	admin := adminClient(t)

	// 422 VALIDATION_FAILED with a field-keyed details map.
	t.Run("invalid uuid on a path parameter", func(t *testing.T) {
		env := admin.mustStatus(http.MethodGet, "/api/v1/products/not-a-uuid", nil, http.StatusBadRequest)
		// a malformed id is rejected by the handler before reaching the
		// service: 400 BAD_REQUEST, not 404.
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Fatalf("malformed product id should be 400 BAD_REQUEST, got: %s", env.Error)
		}
	})

	t.Run("missing required field on create", func(t *testing.T) {
		env := admin.mustStatus(http.MethodPost, "/api/v1/products",
			map[string]any{}, http.StatusUnprocessableEntity)
		if env.Error == nil || env.Error.Code != "VALIDATION_FAILED" {
			t.Fatalf("expected 422 VALIDATION_FAILED, got: %s", env.Error)
		}
		if _, ok := env.Error.Details["name"]; !ok {
			t.Fatalf("details should name the 'name' field, got: %v", env.Error.Details)
		}
	})

	t.Run("malformed JSON body", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/products",
			bytes.NewReader([]byte("{not json")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := admin.http.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("malformed JSON status = %d, want 422", resp.StatusCode)
		}
	})

	// 404 NOT_FOUND on an unknown route (the NoRoute handler).
	t.Run("unknown route", func(t *testing.T) {
		env := admin.mustStatus(http.MethodGet, "/api/v1/does-not-exist", nil, http.StatusNotFound)
		if env.Error == nil || env.Error.Code != "NOT_FOUND" {
			t.Fatalf("unknown route should be 404 NOT_FOUND, got: %s", env.Error)
		}
	})
}
