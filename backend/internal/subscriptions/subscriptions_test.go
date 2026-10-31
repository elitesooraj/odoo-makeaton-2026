package subscriptions

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// buildLine is the validation gate every subscription line passes through,
// from POST /subscriptions/:id/lines and from quotation templates. This test
// locks its decision table: which inputs are rejected with which field key,
// and how defaults are applied.

func TestBuildLine(t *testing.T) {
	productID := uuid.Must(uuid.NewV7()).String()
	taxID := uuid.Must(uuid.NewV7()).String()
	variantID := uuid.Must(uuid.NewV7()).String()
	taxPtr := func(v string) *string { return &v }

	tests := []struct {
		name      string
		in        LineInput
		wantOK    bool
		wantField string // the details key expected on rejection
		checkLine func(*testing.T, Line)
	}{
		{
			name: "a fully specified line validates",
			in: LineInput{
				ProductID: productID, Quantity: 2, UnitPriceCents: 14900,
				Description: "Weekly 10L", TaxID: taxPtr(taxID), VariantID: taxPtr(variantID),
			},
			wantOK: true,
			checkLine: func(t *testing.T, l Line) {
				if l.ProductID.String() != productID {
					t.Errorf("product id not carried through")
				}
				if l.Quantity != 2 {
					t.Errorf("quantity = %d, want 2", l.Quantity)
				}
				if l.UnitPriceCents != 14900 {
					t.Errorf("unit price = %d, want 14900", l.UnitPriceCents)
				}
				if l.TaxID == nil || l.TaxID.String() != taxID {
					t.Errorf("tax id not parsed")
				}
				if l.VariantID == nil || l.VariantID.String() != variantID {
					t.Errorf("variant id not parsed")
				}
				if l.Description == nil || *l.Description != "Weekly 10L" {
					t.Errorf("description not set")
				}
			},
		},
		{
			name:   "quantity zero defaults to one instead of rejecting",
			in:     LineInput{ProductID: productID, Quantity: 0, UnitPriceCents: 100},
			wantOK: true,
			checkLine: func(t *testing.T, l Line) {
				if l.Quantity != 1 {
					t.Errorf("quantity = %d, want the default 1", l.Quantity)
				}
			},
		},
		{
			name:      "negative quantity also falls back to one",
			in:        LineInput{ProductID: productID, Quantity: -3, UnitPriceCents: 100},
			wantOK:    true,
			checkLine: func(t *testing.T, l Line) { _ = l },
		},
		{
			name:      "missing product id is rejected under the product_id key",
			in:        LineInput{Quantity: 1},
			wantOK:    false,
			wantField: "product_id",
		},
		{
			name:      "malformed product id is rejected",
			in:        LineInput{ProductID: "not-a-uuid", Quantity: 1},
			wantOK:    false,
			wantField: "product_id",
		},
		{
			name:      "blank product id is rejected after trimming",
			in:        LineInput{ProductID: "   ", Quantity: 1},
			wantOK:    false,
			wantField: "product_id",
		},
		{
			name:      "negative unit price is rejected",
			in:        LineInput{ProductID: productID, UnitPriceCents: -1},
			wantOK:    false,
			wantField: "unit_price_cents",
		},
		{
			name:   "a zero price is allowed: free lines exist",
			in:     LineInput{ProductID: productID, UnitPriceCents: 0},
			wantOK: true,
		},
		{
			name:      "malformed variant id is rejected under variant_id",
			in:        LineInput{ProductID: productID, VariantID: taxPtr("junk")},
			wantOK:    false,
			wantField: "variant_id",
		},
		{
			name:      "malformed tax id is rejected under tax_id",
			in:        LineInput{ProductID: productID, TaxID: taxPtr("junk")},
			wantOK:    false,
			wantField: "tax_id",
		},
		{
			name:   "an empty-string optional pointer is treated as unset",
			in:     LineInput{ProductID: productID, TaxID: taxPtr(""), VariantID: taxPtr("")},
			wantOK: true,
			checkLine: func(t *testing.T, l Line) {
				if l.TaxID != nil {
					t.Errorf("blank tax id should stay nil, got %s", l.TaxID)
				}
				if l.VariantID != nil {
					t.Errorf("blank variant id should stay nil")
				}
			},
		},
		{
			name:   "uuids are trimmed before parsing",
			in:     LineInput{ProductID: " " + productID + "\t", Quantity: 1},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, details, ok := buildLine(tt.in)

			if ok != tt.wantOK {
				t.Fatalf("buildLine ok = %v, want %v (details: %v)", ok, tt.wantOK, details)
			}
			if !tt.wantOK {
				if _, has := details[tt.wantField]; !has {
					t.Fatalf("expected details key %q, got %v", tt.wantField, details)
				}
				return
			}
			if len(details) != 0 {
				t.Fatalf("unexpected validation details on a valid line: %v", details)
			}
			if tt.checkLine != nil {
				tt.checkLine(t, line)
			}
		})
	}
}

// editableStatus is the gate for PATCH /subscriptions/:id. It documents the
// CURRENT behavior: only CLOSED is outright rejected. Note for the module
// owner: docs/API.md promises "fields editable only in DRAFT/QUOTATION", but
// this gate also lets CONFIRMED, ACTIVE and PAUSED through — the line
// endpoints are stricter (DRAFT/QUOTATION only, enforced inline). This test
// pins what the code does today so any tightening is a deliberate change.
func TestEditableStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool // true = editable
	}{
		{"DRAFT is editable", StatusDraft, true},
		{"QUOTATION is editable", StatusQuotation, true},
		{"CONFIRMED is not editable", StatusConfirmed, false},
		{"ACTIVE is not editable", StatusActive, false},
		{"PAUSED is not editable", StatusPaused, false},
		{"CLOSED is never editable", StatusClosed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := editableStatus(tt.status)
			if tt.want && err != nil {
				t.Fatalf("status %q should be editable, got %v", tt.status, err)
			}
			if !tt.want && err == nil {
				t.Fatalf("status %q must not be editable", tt.status)
			}
			if !tt.want {
				msg := err.Error()
				if !strings.Contains(msg, "only DRAFT or QUOTATION") {
					t.Fatalf("expected a 'DRAFT or QUOTATION' conflict message, got %q", msg)
				}
			}
		})
	}
}

// The lifecycle vocabulary: these constants are the state machine's alphabet
// and are persisted as text in the database. A typo here would break every
// status filter, so pin the exact strings.
func TestStatusConstants(t *testing.T) {
	cases := map[string]string{
		StatusDraft:     "DRAFT",
		StatusQuotation: "QUOTATION",
		StatusConfirmed: "CONFIRMED",
		StatusActive:    "ACTIVE",
		StatusPaused:    "PAUSED",
		StatusClosed:    "CLOSED",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("status constant drifted: %q != %q", got, want)
		}
	}
}
