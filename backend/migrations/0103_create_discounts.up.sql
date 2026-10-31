-- Discount rules. A rule is either a fixed amount or a percentage, can target a
-- product or the whole subscription, and can limit uses. Redemptions are
-- recorded in discount_applications (migration 0303).

CREATE TABLE discounts (
    id                  uuid PRIMARY KEY,
    name                text NOT NULL,
    kind                text NOT NULL CHECK (kind IN ('FIXED', 'PERCENT')),
    value_cents         bigint NOT NULL DEFAULT 0 CHECK (value_cents >= 0),
    value_bp            integer NOT NULL DEFAULT 0 CHECK (value_bp BETWEEN 0 AND 10000),
    scope               text NOT NULL DEFAULT 'SUBSCRIPTION'
                        CHECK (scope IN ('SUBSCRIPTION', 'PRODUCT')),
    product_id          uuid REFERENCES products (id) ON DELETE SET NULL,
    min_purchase_cents  bigint NOT NULL DEFAULT 0 CHECK (min_purchase_cents >= 0),
    min_quantity        integer NOT NULL DEFAULT 0 CHECK (min_quantity >= 0),
    starts_at           timestamptz,
    ends_at             timestamptz,
    max_uses            integer CHECK (max_uses IS NULL OR max_uses > 0),
    used_count          integer NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    is_active           boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at < ends_at),
    CHECK (scope <> 'PRODUCT' OR product_id IS NOT NULL)
);

CREATE INDEX discounts_active_idx ON discounts (is_active);
CREATE INDEX discounts_product_idx ON discounts (product_id);

CREATE TRIGGER discounts_set_updated_at
    BEFORE UPDATE ON discounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
