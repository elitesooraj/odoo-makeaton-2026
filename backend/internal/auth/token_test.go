package auth

import (
	"bytes"
	"testing"
)

func TestOpaqueTokensAreUniqueAndHashed(t *testing.T) {
	seen := make(map[string]struct{}, 100)

	for i := 0; i < 100; i++ {
		raw, hash, err := newOpaqueToken()
		if err != nil {
			t.Fatalf("newOpaqueToken: %v", err)
		}
		if raw == "" {
			t.Fatal("empty token")
		}
		if len(hash) != 32 {
			t.Fatalf("expected 32-byte SHA-256 hash, got %d", len(hash))
		}
		if !bytes.Equal(hash, hashOpaqueToken(raw)) {
			t.Fatal("hash is not stable for the same token")
		}
		if bytes.Contains(hash, []byte(raw)) {
			t.Fatal("hash must not contain the raw token")
		}
		if _, dup := seen[raw]; dup {
			t.Fatal("duplicate token generated")
		}
		seen[raw] = struct{}{}
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("Bright!123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "Bright!123" {
		t.Fatal("password was not hashed")
	}
	if !VerifyPassword(hash, "Bright!123") {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword(hash, "Bright!124") {
		t.Fatal("wrong password accepted")
	}
}
