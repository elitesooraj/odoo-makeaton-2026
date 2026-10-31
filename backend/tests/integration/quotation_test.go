package integration

import (
	"net/http"
	"testing"
)

// TestQuotationTemplateLifecycle proves the template -> QUOTATION flow:
// a reusable line set can be created, listed, read, updated, and expanded
// into a subscription in QUOTATION status with a populated quote_expires_at.
func TestQuotationTemplateLifecycle(t *testing.T) {
	reset(t)
	admin := adminClient(t)
	fix := createCatalog(t, admin, "WEEKLY")
	customerID := createCustomer(t, admin, "Quoted Corp")

	// Create a template from a plan line.
	var tmpl struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ValidityDays int    `json:"validity_days"`
		IsActive     bool   `json:"is_active"`
		Lines        []struct {
			PlanID   string `json:"plan_id"`
			Quantity int    `json:"quantity"`
		} `json:"lines"`
	}
	admin.mustOK(http.MethodPost, "/api/v1/quotations/templates", map[string]any{
		"name": "Water weekly", "validity_days": 14,
		"lines": []map[string]any{{"plan_id": fix.PlanID, "quantity": 3}},
		"notes": "proposal",
	}, &tmpl)

	if tmpl.Name != "Water weekly" || tmpl.ValidityDays != 14 || !tmpl.IsActive {
		t.Fatalf("template fields wrong: %+v", tmpl)
	}
	if len(tmpl.Lines) != 1 || tmpl.Lines[0].PlanID != fix.PlanID || tmpl.Lines[0].Quantity != 3 {
		t.Fatalf("template lines not stored: %+v", tmpl.Lines)
	}

	// List + get round-trip.
	var listed []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates", nil, &listed)
	if len(listed) != 1 || listed[0].ID != tmpl.ID {
		t.Fatalf("template list = %+v, want the one just created", listed)
	}
	var got struct {
		ID    string `json:"id"`
		Lines []struct {
			Quantity int `json:"quantity"`
		} `json:"lines"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates/"+tmpl.ID, nil, &got)
	if got.ID != tmpl.ID || len(got.Lines) != 1 || got.Lines[0].Quantity != 3 {
		t.Fatalf("template get wrong: %+v", got)
	}

	// Update: bump validity and replace the lines.
	admin.mustOK(http.MethodPatch, "/api/v1/quotations/templates/"+tmpl.ID, map[string]any{
		"name": "Water weekly v2", "validity_days": 30,
		"lines": []map[string]any{{"plan_id": fix.PlanID, "quantity": 5}},
	}, nil)

	// Instantiate: expands the template into a QUOTATION subscription that
	// carries the template's quantity (5) and the plan's price (14900).
	var quote subscriptionDTO
	admin.mustOK(http.MethodPost, "/api/v1/quotations/templates/"+tmpl.ID+"/instantiate",
		map[string]any{"customer_id": customerID, "payment_terms_days": 15}, &quote)

	if quote.Status != "QUOTATION" {
		t.Fatalf("instantiated status = %q, want QUOTATION", quote.Status)
	}
	if quote.QuoteExpiresAt == nil {
		t.Fatal("quote_expires_at must be set from validity_days (30)")
	}
	if quote.CustomerID != customerID {
		t.Errorf("customer_id = %s, want %s", quote.CustomerID, customerID)
	}
	if quote.PaymentTermsDays != 15 {
		t.Errorf("payment_terms_days = %d, want 15 (passed through)", quote.PaymentTermsDays)
	}
	if len(quote.Lines) != 1 || quote.Lines[0].Quantity != 5 {
		t.Errorf("line quantity not carried from the template: %+v", quote.Lines)
	}

	// Confirming promotes it out of QUOTATION; the lifecycle continues.
	admin.mustOK(http.MethodPost, "/api/v1/subscriptions/"+quote.ID+"/confirm", map[string]any{}, nil)
	var confirmed subscriptionDTO
	admin.mustOK(http.MethodGet, "/api/v1/subscriptions/"+quote.ID, nil, &confirmed)
	if confirmed.Status != "CONFIRMED" {
		t.Fatalf("status after confirm = %q, want CONFIRMED", confirmed.Status)
	}

	// Deactivate a template: it stays readable but is flagged inactive.
	admin.mustOK(http.MethodDelete, "/api/v1/quotations/templates/"+tmpl.ID, nil, nil)
	var inactive struct {
		IsActive bool `json:"is_active"`
	}
	admin.mustOK(http.MethodGet, "/api/v1/quotations/templates/"+tmpl.ID, nil, &inactive)
	if inactive.IsActive {
		t.Error("deactivated template still reports is_active = true")
	}

	// Instantiating an inactive template is refused.
	admin.mustStatus(http.MethodPost, "/api/v1/quotations/templates/"+tmpl.ID+"/instantiate",
		map[string]any{"customer_id": customerID}, http.StatusConflict)

}
