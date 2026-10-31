-- Catalog: products and their named variants. A variant is an additive price
-- delta (for example "+560 for the Odoo brand"), not an absolute price.

CREATE TABLE products (
    id                uuid PRIMARY KEY,
    name              text NOT NULL,
    type              text NOT NULL DEFAULT 'SERVICE'
                      CHECK (type IN ('SERVICE', 'GOODS')),
    sales_price_cents bigint NOT NULL DEFAULT 0 CHECK (sales_price_cents >= 0),
    cost_price_cents  bigint NOT NULL DEFAULT 0 CHECK (cost_price_cents >= 0),
    notes             text,
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX products_active_idx ON products (is_active);
CREATE INDEX products_name_lower_idx ON products (lower(name));

CREATE TRIGGER products_set_updated_at
    BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE product_variants (
    id                uuid PRIMARY KEY,
    product_id        uuid NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    name              text NOT NULL,
    price_delta_cents bigint NOT NULL DEFAULT 0,
    is_active         boolean NOT NULL DEFAULT true,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (product_id, name)
);

CREATE INDEX product_variants_product_idx ON product_variants (product_id);

CREATE TRIGGER product_variants_set_updated_at
    BEFORE UPDATE ON product_variants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
