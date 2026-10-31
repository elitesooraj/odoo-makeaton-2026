package users

import (
	"testing"

	"github.com/brightloop/subscription-platform/backend/internal/platform/validate"
)

func TestGenerateTempPasswordSatisfiesPolicy(t *testing.T) {
	seen := make(map[string]struct{}, 50)

	for i := 0; i < 50; i++ {
		password, err := GenerateTempPassword()
		if err != nil {
			t.Fatalf("GenerateTempPassword: %v", err)
		}
		if len(password) != 16 {
			t.Fatalf("expected 16 characters, got %d", len(password))
		}
		if details := validate.Password(password); len(details) != 0 {
			t.Fatalf("generated password %q fails policy: %v", password, details)
		}
		if _, dup := seen[password]; dup {
			t.Fatal("duplicate temporary password generated")
		}
		seen[password] = struct{}{}
	}
}
