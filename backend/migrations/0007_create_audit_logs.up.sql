-- Append-only audit trail for administrative and customer mutations.
-- before/after hold the changed representation, never secrets.

CREATE TABLE audit_logs (
    id            uuid PRIMARY KEY,
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_role    text,
    action        text NOT NULL,
    entity_type   text NOT NULL,
    entity_id     uuid NOT NULL,
    before_state  jsonb,
    after_state   jsonb,
    ip            text,
    user_agent    text,
    request_id    text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_user_id);
