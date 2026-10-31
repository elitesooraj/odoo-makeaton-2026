// Package number generates human-readable document numbers from a Postgres
// sequence. Sequence names are compile-time constants chosen by the caller.
package number

import (
	"context"
	"fmt"

	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
)

// Next returns PREFIX-000001 style numbers. The sequence name must be a trusted
// constant (it is interpolated into SQL, never user input).
func Next(ctx context.Context, q db.Querier, sequence, prefix string) (string, error) {
	var n int64
	if err := q.QueryRow(ctx, "SELECT nextval('"+sequence+"')").Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%06d", prefix, n), nil
}
