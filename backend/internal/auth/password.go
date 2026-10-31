package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost 12 keeps login well under the 2s page budget while making offline
// cracking expensive. Changing this value does not invalidate existing hashes.
const bcryptCost = 12

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
