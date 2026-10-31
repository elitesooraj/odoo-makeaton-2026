package subscriptions

import (
	"errors"
	"testing"
)

func TestStatusValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"draft", StatusDraft, true},
		{"quotation", StatusQuotation, true},
		{"confirmed", StatusConfirmed, true},
		{"active", StatusActive, true},
		{"closed", StatusClosed, true},
		{"empty", Status(""), false},
		{"unknown", Status("PAUSED"), false},
		{"lowercase", Status("draft"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.Valid(); got != tt.want {
				t.Fatalf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateTransitionAllowed(t *testing.T) {
	tests := []struct {
		from Status
		to   Status
	}{
		{StatusDraft, StatusQuotation},
		{StatusQuotation, StatusConfirmed},
		{StatusConfirmed, StatusActive},
		{StatusActive, StatusClosed},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err != nil {
				t.Fatalf("ValidateTransition(%q, %q) = %v, want nil", tt.from, tt.to, err)
			}
			if !CanTransition(tt.from, tt.to) {
				t.Fatalf("CanTransition(%q, %q) = false, want true", tt.from, tt.to)
			}
		})
	}
}

func TestValidateTransitionRejected(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
	}{
		{"draft_to_active", StatusDraft, StatusActive},
		{"draft_to_closed", StatusDraft, StatusClosed},
		{"quotation_to_active", StatusQuotation, StatusActive},
		{"quotation_to_draft", StatusQuotation, StatusDraft},
		{"confirmed_to_closed", StatusConfirmed, StatusClosed},
		{"confirmed_to_quotation", StatusConfirmed, StatusQuotation},
		{"active_to_draft", StatusActive, StatusDraft},
		{"active_to_confirmed", StatusActive, StatusConfirmed},
		{"closed_to_active", StatusClosed, StatusActive},
		{"closed_to_draft", StatusClosed, StatusDraft},
		{"closed_to_closed", StatusClosed, StatusClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if err == nil {
				t.Fatalf("ValidateTransition(%q, %q) = nil, want an error", tt.from, tt.to)
			}

			var invalid *InvalidTransitionError
			if !errors.As(err, &invalid) {
				t.Fatalf("error type = %T, want *InvalidTransitionError", err)
			}
			if invalid.From != tt.from || invalid.To != tt.to {
				t.Fatalf("error = %+v, want from=%q to=%q", invalid, tt.from, tt.to)
			}
			if CanTransition(tt.from, tt.to) {
				t.Fatalf("CanTransition(%q, %q) = true, want false", tt.from, tt.to)
			}
		})
	}
}

func TestValidateTransitionRejectsSameStatus(t *testing.T) {
	statuses := []Status{
		StatusDraft,
		StatusQuotation,
		StatusConfirmed,
		StatusActive,
		StatusClosed,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			err := ValidateTransition(status, status)
			if err == nil {
				t.Fatalf("ValidateTransition(%q, %q) = nil, want an error", status, status)
			}

			var invalid *InvalidTransitionError
			if !errors.As(err, &invalid) {
				t.Fatalf("error type = %T, want *InvalidTransitionError", err)
			}
		})
	}
}

func TestValidateTransitionRejectsUnknownStatus(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
	}{
		{"unknown_current", Status("PAUSED"), StatusDraft},
		{"unknown_requested", StatusDraft, Status("PAUSED")},
		{"empty_current", Status(""), StatusDraft},
		{"empty_requested", StatusDraft, Status("")},
		{"unknown_both", Status(""), Status("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if err == nil {
				t.Fatalf("ValidateTransition(%q, %q) = nil, want an error", tt.from, tt.to)
			}

			var unknown *UnknownStatusError
			if !errors.As(err, &unknown) {
				t.Fatalf("error type = %T, want *UnknownStatusError", err)
			}
			if CanTransition(tt.from, tt.to) {
				t.Fatalf("CanTransition(%q, %q) = true, want false", tt.from, tt.to)
			}
		})
	}
}
