-- Invoices snapshot every value they need. Rendering an old invoice must never
-- join live product or plan prices. Overdue is derived from due_date and
-- balance_cents, not stored as a status.

CREATE SEQUENCE invoice_number_seq;

CREATE TABLE invoices (
    id              uuid PRIMARY KEY,
    number          text NOT NULL UNIQUE,
    customer_id     uuid NOT NULL REFERENCES customers (id) ON DELETE RESTRICT,
    subscription_id uuid REFERENCES subscriptions (id) ON DELETE SET NULL,
    status          text NOT NULL DEFAULT 'DRAFT'
                    CHECK (status IN ('DRAFT', 'CONFIRMED', 'PAID', 'CANCELLED')),
    issue_date      date NOT NULL DEFAULT current_date,
    due_date        date NOT NULL,
    period_start    date,
    period_end      date,
    subtotal_cents  bigint NOT NULL DEFAULT 0 CHECK (subtotal_cents >= 0),
    discount_cents  bigint NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    tax_cents       bigint NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
    total_cents     bigint NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    balance_cents   bigint NOT NULL DEFAULT 0 CHECK (balance_cents >= 0),
    notes           text,
    confirmed_at    timestamptz,
    sent_at         timestamptz,
    paid_at         timestamptz,
    cancelled_at    timestamptz,
    created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CHECK (period_end IS NULL OR period_start IS NULL OR period_start < period_end)
);

CREATE INDEX invoices_customer_idx ON invoices (customer_id);
CREATE INDEX invoices_status_idx ON invoices (status);
CREATE INDEX invoices_issue_date_idx ON invoices (issue_date);
CREATE INDEX invoices_due_date_idx ON invoices (due_date);
CREATE INDEX invoices_subscription_idx ON invoices (subscription_id);

-- One invoice per subscription billing period. This is the database guard that
-- makes the billing worker exactly-once.
CREATE UNIQUE INDEX invoices_subscription_period_uq
    ON invoices (subscription_id, period_start)
    WHERE subscription_id IS NOT NULL AND period_start IS NOT NULL;

CREATE TRIGGER invoices_set_updated_at
    BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE invoice_lines (
    id               uuid PRIMARY KEY,
    invoice_id       uuid NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    product_id       uuid REFERENCES products (id) ON DELETE SET NULL,
    variant_id       uuid REFERENCES product_variants (id) ON DELETE SET NULL,
    description      text NOT NULL,
    quantity         integer NOT NULL CHECK (quantity > 0),
    unit_price_cents bigint NOT NULL CHECK (unit_price_cents >= 0),
    tax_rate_bp      integer NOT NULL DEFAULT 0 CHECK (tax_rate_bp BETWEEN 0 AND 100000),
    tax_cents        bigint NOT NULL DEFAULT 0 CHECK (tax_cents >= 0),
    discount_cents   bigint NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    line_total_cents bigint NOT NULL DEFAULT 0 CHECK (line_total_cents >= 0),
    position         integer NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX invoice_lines_invoice_idx ON invoice_lines (invoice_id);
