-- Tax rules are data, not code. Rate is stored in basis points so 7.5% is 750.
-- is_inclusive means the price already contains the tax.

CREATE TABLE taxes (
    id           uuid PRIMARY KEY,
    name         text NOT NULL,
    rate_bp      integer NOT NULL CHECK (rate_bp BETWEEN 0 AND 100000),
    is_inclusive boolean NOT NULL DEFAULT false,
    country_code text,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX taxes_active_idx ON taxes (is_active);

CREATE TRIGGER taxes_set_updated_at
    BEFORE UPDATE ON taxes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
