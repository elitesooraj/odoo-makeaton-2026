-- Foundation: customers are Brightloop's clients. A customer may exist before
-- they have a portal login; users.customer_id links the login later.

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE customers (
    id              uuid PRIMARY KEY,
    name            text NOT NULL,
    email           text,
    phone           text,
    billing_address jsonb NOT NULL DEFAULT '{}'::jsonb,
    tax_id          text,
    status          text NOT NULL DEFAULT 'ACTIVE'
                    CHECK (status IN ('ACTIVE', 'INACTIVE')),
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX customers_status_idx ON customers (status);
CREATE INDEX customers_name_lower_idx ON customers (lower(name));

CREATE TRIGGER customers_set_updated_at
    BEFORE UPDATE ON customers
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
