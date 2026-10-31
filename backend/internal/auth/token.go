package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// newOpaqueToken returns a 256-bit URL-safe token and the SHA-256 hash that is
// stored in the database. The raw token is only ever sent to the browser or
// embedded in an email link.
func newOpaqueToken() (string, []byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashOpaqueToken(raw), nil
}

func hashOpaqueToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
