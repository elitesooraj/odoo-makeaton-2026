-- A plan belongs to exactly one product. The four flags below decide what may
-- happen to a subscription on this plan: close at expiry, renew, pause, or
-- close before expiry.

CREATE TABLE plans (
    id                 uuid PRIMARY KEY,
    product_id         uuid NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    tax_id             uuid REFERENCES taxes (id) ON DELETE SET NULL,
    name               text NOT NULL,
    interval           text NOT NULL
                       CHECK (interval IN ('DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY')),
    price_cents        bigint NOT NULL CHECK (price_cents >= 0),
    minimum_quantity   integer NOT NULL DEFAULT 1 CHECK (minimum_quantity >= 1),
    available_from     date,
    available_to       date,
    auto_close         boolean NOT NULL DEFAULT false,
    auto_renew         boolean NOT NULL DEFAULT true,
    allow_pause        boolean NOT NULL DEFAULT false,
    allow_early_close  boolean NOT NULL DEFAULT false,
    is_active          boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CHECK (available_from IS NULL OR available_to IS NULL OR available_from <= available_to)
);

CREATE INDEX plans_product_idx ON plans (product_id);
CREATE INDEX plans_active_idx ON plans (is_active);

CREATE TRIGGER plans_set_updated_at
    BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
