// Package validate holds input rules shared by every module.
package validate

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// PasswordMinLength is "more than eight characters" from the requirements.
const PasswordMinLength = 9

// bcrypt silently truncates at 72 bytes, so longer passwords are rejected
// instead of being accepted and partially ignored.
const passwordMaxBytes = 72

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func Email(value string) bool {
	return emailPattern.MatchString(strings.TrimSpace(value))
}

// NormalizeEmail is the canonical form used for uniqueness and lookups.
func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Password returns field errors keyed by rule. An empty map means the password
// is acceptable.
func Password(password string) map[string]string {
	details := map[string]string{}

	if password == "" {
		details["password"] = "password is required"
		return details
	}
	if len(password) > passwordMaxBytes {
		details["password_max_length"] = "password must be at most 72 bytes"
	}
	if utf8.RuneCountInString(password) < PasswordMinLength {
		details["password_min_length"] = "password must be longer than 8 characters"
	}

	var hasUpper, hasLower, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if !hasUpper {
		details["password_uppercase"] = "password must contain an uppercase letter"
	}
	if !hasLower {
		details["password_lowercase"] = "password must contain a lowercase letter"
	}
	if !hasSpecial {
		details["password_special"] = "password must contain a special character"
	}
	return details
}
