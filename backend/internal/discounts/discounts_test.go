package discounts

import (
	"testing"
)

func TestValidateValueBPBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		kind        string
		valueBP     int
		wantError   bool
		errorField  string
	}{
		{"1 bp (0.01%) PERCENT", "PERCENT", 1, false, ""},
		{"9999 bp (99.99%) PERCENT", "PERCENT", 9999, false, ""},
		{"10000 bp (100%) PERCENT", "PERCENT", 10000, false, ""},
		{"10001 bp PERCENT exceeds max", "PERCENT", 10001, true, "value_bp"},
		{"0 bp PERCENT invalid", "PERCENT", 0, true, "value_bp"},
		{"negative bp PERCENT", "PERCENT", -1, true, "value_bp"},
		{"FIXED with value_cents positive", "FIXED", 0, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{Name: "test", Kind: tc.kind, ValueBP: tc.valueBP, ValueCents: 1000}
			_, details := validate("", in, nil)
			hasError := len(details) > 0
			if tc.wantError != hasError {
				t.Fatalf("Validate: wantError=%v, got %v, details=%v", tc.wantError, hasError, details)
			}
			if tc.wantError && details[tc.errorField] == "" {
				t.Fatalf("Expected error on field %q, got details %v", tc.errorField, details)
			}
		})
	}
}

func TestValidateValueCentsBounds(t *testing.T) {
	cases := []struct {
		name       string
		kind       string
		valueCents int64
		wantError  bool
	}{
		{"FIXED 1 cent", "FIXED", 1, false},
		{"FIXED 10 billion", "FIXED", 1_000_000_000_000, false},
		{"FIXED exceeds max", "FIXED", 1_000_000_000_001, true},
		{"FIXED zero invalid", "FIXED", 0, true},
		{"FIXED negative invalid", "FIXED", -100, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{Name: "test", Kind: tc.kind, ValueCents: tc.valueCents}
			_, details := validate("", in, nil)
			hasError := len(details) > 0
			if tc.wantError != hasError {
				t.Fatalf("Validate: wantError=%v, got %v, details=%v", tc.wantError, hasError, details)
			}
		})
	}
}