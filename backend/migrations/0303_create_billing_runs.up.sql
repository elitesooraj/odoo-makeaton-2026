-- One row per billing attempt. The unique key plus the invoice unique index
-- make the worker exactly-once even if two instances run concurrently.

CREATE TABLE billing_runs (
    id              uuid PRIMARY KEY,
    subscription_id uuid NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    period_start    date NOT NULL,
    period_end      date NOT NULL,
    invoice_id      uuid REFERENCES invoices (id) ON DELETE SET NULL,
    status          text NOT NULL CHECK (status IN ('CREATED', 'SKIPPED', 'FAILED')),
    error           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (subscription_id, period_start)
);

CREATE INDEX billing_runs_created_idx ON billing_runs (created_at DESC);
