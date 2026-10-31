-- Redemption ledger. Enforces max_uses and proves which invoice used a rule.

CREATE TABLE discount_applications (
    id              uuid PRIMARY KEY,
    discount_id     uuid NOT NULL REFERENCES discounts (id) ON DELETE RESTRICT,
    invoice_id      uuid NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    subscription_id uuid REFERENCES subscriptions (id) ON DELETE SET NULL,
    amount_cents    bigint NOT NULL CHECK (amount_cents >= 0),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (discount_id, invoice_id)
);

CREATE INDEX discount_applications_discount_idx
    ON discount_applications (discount_id);
