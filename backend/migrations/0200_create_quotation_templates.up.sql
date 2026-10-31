-- Quotation templates are reusable line sets. A quote itself is a subscription
-- in QUOTATION status with quote_expires_at set; see ADR-0006.

CREATE TABLE quotation_templates (
    id           uuid PRIMARY KEY,
    name         text NOT NULL,
    validity_days integer NOT NULL DEFAULT 30 CHECK (validity_days > 0),
    notes        text,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX quotation_templates_active_idx ON quotation_templates (is_active);

CREATE TRIGGER quotation_templates_set_updated_at
    BEFORE UPDATE ON quotation_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE quotation_template_lines (
    id          uuid PRIMARY KEY,
    template_id uuid NOT NULL REFERENCES quotation_templates (id) ON DELETE CASCADE,
    plan_id     uuid NOT NULL REFERENCES plans (id) ON DELETE RESTRICT,
    product_id  uuid REFERENCES products (id) ON DELETE SET NULL,
    variant_id  uuid REFERENCES product_variants (id) ON DELETE SET NULL,
    quantity    integer NOT NULL DEFAULT 1 CHECK (quantity > 0),
    position    integer NOT NULL DEFAULT 0
);

CREATE INDEX quotation_template_lines_template_idx
    ON quotation_template_lines (template_id);
