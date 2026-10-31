package integration

import (
	"net/http"
	"testing"
	"time"
)

// templateDTO mirrors a template response.
type templateDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ValidityDays int    `json:"validity_days"`
	IsActive     bool   `json:"is_active"`
	Lines        []struct {
		PlanID   string `json:"plan_id"`
		Quantity int    `json:"quantity"`
	} `json:"lines"`
}

// TestQuotationTemplateLifecycleAndInstantiate proves the quotation flow:
// a reusable template expands into a subscription in QUOTATION status, with
// the template's lines and validity window carried over.
func TestQuotationTemplateLifecycleAndInstantiate(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Quote Corp")

	// Create a template.
	var tmpl templateDTO
	admin.mustOK(http.MethodPost, "/api/v1/quotations/templates", map[string]any{
		"name": "Water weekly", "validity_days": 45,
		"lines": []map[string]any{{"plan_id": fix.PlanID, "quantity": 3}},
	}, &tmpl)
	if tmpl.Name != "Water weekly" || tmpl.ValidityDays != 45 || !tmpl.IsActive {
		t.Fatalf("template not stored as given: %+v", tmpl)
	}
	if len(tmpl.Lines) != 1 || tmpl.Lines[0].PlanID != fix.PlanID || tmpl.Lines[0].Quantity != 3 {
		t.Fatalf("template lines wrong: %+v", tmpl.Lines)
	}

	// List + Get round-trip.
	var listed []templateDTO
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates", nil, &listed)
	if len(listed) != 1 {
		t.Fatalf("list returned %d templates, want 1", len(listed))
	}
	var got templateDTO
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates/"+tmpl.ID, nil, &got)
	if got.ID != tmpl.ID {
		t.Fatalf("get returned %q, want %q", got.ID, tmpl.ID)
	}

	// Update (PATCH) the template name and lines.
	admin.mustOK(http.MethodPatch, "/api/v1/quotations/templates/"+tmpl.ID,
		map[string]any{"name": "Water weekly v2"}, nil)
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates/"+tmpl.ID, nil, &got)
	if got.Name != "Water weekly v2" {
		t.Errorf("patched name = %q, want v2", got.Name)
	}

	// Instantiate expands into a QUOTATION subscription.
	var quote subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/quotations/templates/"+tmpl.ID+"/instantiate",
		map[string]any{"customer_id": customerID}, &quote)
	if quote.Status != "QUOTATION" {
		t.Fatalf("instantiated status = %q, want QUOTATION", quote.Status)
	}
	if quote.QuoteExpiresAt == nil {
		t.Fatal("instantiated quote must carry quote_expires_at")
	}
	expires, err := time.Parse(time.RFC3339, *quote.QuoteExpiresAt)
	if err != nil {
		t.Fatalf("quote_expires_at %q is not RFC3339: %v", *quote.QuoteExpiresAt, err)
	}
	low := time.Now().UTC().AddDate(0, 0, 44)
	high := time.Now().UTC().AddDate(0, 0, 46)
	if expires.Before(low) || expires.After(high) {
		t.Fatalf("quote_expires_at = %s, want within %s..%s (45 days)",
			expires.Format(time.RFC3339), low.Format("2006-01-02"), high.Format("2006-01-02"))
	}
	if len(quote.Lines) != 1 || quote.Lines[0].Quantity != 3 {
		t.Fatalf("template lines not carried onto the quote: %+v", quote.Lines)
	}
	if quote.PlanID != fix.PlanID {
		t.Errorf("quote plan = %s, want %s", quote.PlanID, fix.PlanID)
	}

	// A quotation can be confirmed into a normal subscription.
	var confirmed subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+quote.ID+"/confirm",
		map[string]any{}, &confirmed)
	if confirmed.Status != "CONFIRMED" {
		t.Fatalf("confirmed quote status = %q, want CONFIRMED", confirmed.Status)
	}

	// Deactivate the template, then refuse to instantiate it.
	admin.mustOK(http.MethodDelete, "/api/v1/quotations/templates/"+tmpl.ID, nil, nil)
	var inactive templateDTO
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates/"+tmpl.ID, nil, &inactive)
	if inactive.IsActive {
		t.Error("template still active after DELETE")
	}
	admin.mustStatus(http.MethodPost, "/api/v1/quotations/templates/"+tmpl.ID+"/instantiate",
		map[string]any{"customer_id": customerID}, http.StatusConflict)
}

// timeNowPlusDays is a tiny helper kept here to avoid importing extra packages.
func timeNowPlusDays(d int) string {
	return todayPlusDays(d)
}
