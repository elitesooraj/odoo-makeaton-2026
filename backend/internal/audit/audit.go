// Package audit writes the append-only trail of administrative mutations.
//
// Records are written in the same transaction as the change they describe, so
// a committed customer or user change always has a matching audit row.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/brightloop/subscription-platform/backend/internal/platform/db"
	"github.com/brightloop/subscription-platform/backend/internal/platform/id"
)

type Meta struct {
	IP        string
	UserAgent string
	RequestID string
}

type Event struct {
	ActorID    *uuid.UUID
	ActorRole  string
	Action     string
	EntityType string
	EntityID   uuid.UUID
	Before     any
	After      any
	Meta       Meta
}

type Recorder struct {
	pool *pgxpool.Pool
}

func NewRecorder(pool *pgxpool.Pool) *Recorder {
	return &Recorder{pool: pool}
}

func (r *Recorder) Record(ctx context.Context, q db.Querier, e Event) error {
	before, err := jsonValue(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonValue(e.After)
	if err != nil {
		return err
	}

	_, err = q.Exec(ctx, `INSERT INTO audit_logs
		(id, actor_user_id, actor_role, action, entity_type, entity_id,
		 before_state, after_state, ip, user_agent, request_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		id.New(), actorArg(e.ActorID), e.ActorRole, e.Action, e.EntityType, e.EntityID,
		before, after, emptyToNil(e.Meta.IP), emptyToNil(e.Meta.UserAgent), emptyToNil(e.Meta.RequestID),
	)
	return err
}

func jsonValue(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func actorArg(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func emptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
