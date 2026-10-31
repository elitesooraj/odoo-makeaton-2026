// Optional-argument conversions shared by repositories. pgx encodes a nil
// unspecified value as NULL and keeps time.Time/uuid.UUID as is, so these
// helpers only need to turn Go optionals into database optionals.
package id

import (
	"time"

	"github.com/google/uuid"
)

// UUIDArg converts an optional UUID pointer into NULL or the value.
func UUIDArg(v *uuid.UUID) any {
	if v == nil {
		return nil
	}
	return *v
}

// StringArg converts an optional string pointer into NULL or the value.
func StringArg(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

// DateArg converts an optional date into NULL or the value.
func DateArg(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}
