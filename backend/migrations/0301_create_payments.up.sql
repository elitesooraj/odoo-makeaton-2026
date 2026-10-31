-- Payments always belong to exactly one invoice. Recording one and updating the
-- invoice balance must happen in one transaction.

CREATE TABLE payments (
    id          uuid PRIMARY KEY,
    invoice_id  uuid NOT NULL REFERENCES invoices (id) ON DELETE RESTRICT,
    method      text NOT NULL
                CHECK (method IN ('CASH', 'BANK_TRANSFER', 'CARD', 'MOBILE_MONEY', 'OTHER')),
    amount_cents bigint NOT NULL CHECK (amount_cents > 0),
    paid_at     timestamptz NOT NULL DEFAULT now(),
    note        text,
    recorded_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX payments_invoice_idx ON payments (invoice_id);
CREATE INDEX payments_paid_at_idx ON payments (paid_at DESC);
