-- Subscription lifecycle: DRAFT -> QUOTATION -> CONFIRMED -> ACTIVE -> CLOSED.
-- PAUSED is available from ACTIVE when the plan allows it. Financial history is
-- never deleted; a subscription is closed, not removed.

CREATE SEQUENCE subscription_number_seq;

CREATE TABLE subscriptions (
    id                 uuid PRIMARY KEY,
    number             text NOT NULL UNIQUE,
    customer_id        uuid NOT NULL REFERENCES customers (id) ON DELETE RESTRICT,
    plan_id            uuid NOT NULL REFERENCES plans (id) ON DELETE RESTRICT,
    status             text NOT NULL DEFAULT 'DRAFT'
                       CHECK (status IN ('DRAFT', 'QUOTATION', 'CONFIRMED', 'ACTIVE', 'PAUSED', 'CLOSED')),
    start_date         date NOT NULL,
    expiry_date        date,
    payment_terms_days integer NOT NULL DEFAULT 30 CHECK (payment_terms_days >= 0),
    quantity           integer NOT NULL DEFAULT 1 CHECK (quantity > 0),
    discount_id        uuid REFERENCES discounts (id) ON DELETE SET NULL,
    next_billing_date  date,
    quote_expires_at   timestamptz,
    notes              text,
    created_by         uuid REFERENCES users (id) ON DELETE SET NULL,
    activated_at       timestamptz,
    paused_at          timestamptz,
    closed_at          timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (expiry_date IS NULL OR start_date <= expiry_date)
);

CREATE INDEX subscriptions_customer_idx ON subscriptions (customer_id);
CREATE INDEX subscriptions_plan_idx ON subscriptions (plan_id);
CREATE INDEX subscriptions_status_idx ON subscriptions (status);
CREATE INDEX subscriptions_next_billing_idx ON subscriptions (next_billing_date)
    WHERE status = 'ACTIVE';

CREATE TRIGGER subscriptions_set_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE subscription_lines (
    id              uuid PRIMARY KEY,
    subscription_id uuid NOT NULL REFERENCES subscriptions (id) ON DELETE CASCADE,
    product_id      uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    variant_id      uuid REFERENCES product_variants (id) ON DELETE SET NULL,
    description     text,
    quantity        integer NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price_cents bigint NOT NULL CHECK (unit_price_cents >= 0),
    tax_id          uuid REFERENCES taxes (id) ON DELETE SET NULL,
    position        integer NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX subscription_lines_subscription_idx
    ON subscription_lines (subscription_id);
