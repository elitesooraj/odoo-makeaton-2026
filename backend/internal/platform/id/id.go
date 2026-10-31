// Package id generates sortable, application-owned identifiers.
//
// All primary keys are UUIDv7 generated in Go. The database never generates
// IDs, so a record's ID is known before the INSERT and can be used to build
// related rows inside the same transaction.
package id

import "github.com/google/uuid"

// New returns a UUIDv7. It falls back to UUIDv4 only if the system random
// source fails, which would already be a fatal condition elsewhere.
func New() uuid.UUID {
	v, err := uuid.NewV7()
	if err != nil {
		return uuid.New()
	}
	return v
}
