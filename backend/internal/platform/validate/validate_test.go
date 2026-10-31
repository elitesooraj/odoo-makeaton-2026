package validate

import "testing"

func TestPasswordPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantOK   bool
		wantKey  string
	}{
		{"valid", "Bright!123", true, ""},
		{"too short", "Br!ght12", false, "password_min_length"},
		{"exactly eight", "Bright!1", false, "password_min_length"},
		{"no uppercase", "bright!123", false, "password_uppercase"},
		{"no lowercase", "BRIGHT!123", false, "password_lowercase"},
		{"no special", "Bright1234", false, "password_special"},
		{"empty", "", false, "password"},
		{"too long", "Aa!" + string(make([]byte, 80)), false, "password_max_length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := Password(tt.password)
			if tt.wantOK {
				if len(details) != 0 {
					t.Fatalf("expected valid password, got %v", details)
				}
				return
			}
			if _, ok := details[tt.wantKey]; !ok {
				t.Fatalf("expected detail %q, got %v", tt.wantKey, details)
			}
		})
	}
}

func TestPasswordAcceptsSymbols(t *testing.T) {
	for _, pw := range []string{"Brightloop€1", "Brightloop+1", "Brightloop=1"} {
		if details := Password(pw); len(details) != 0 {
			t.Errorf("%q should be accepted, got %v", pw, details)
		}
	}
}

func TestEmail(t *testing.T) {
	valid := []string{"a@b.co", "first.last+tag@example.com"}
	invalid := []string{"", "nope", "a@b", "a b@c.com", "a@b .com"}

	for _, v := range valid {
		if !Email(v) {
			t.Errorf("%q should be valid", v)
		}
	}
	for _, v := range invalid {
		if Email(v) {
			t.Errorf("%q should be invalid", v)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Admin@Brightloop.TEST "); got != "admin@brightloop.test" {
		t.Fatalf("unexpected normalization: %q", got)
	}
}
