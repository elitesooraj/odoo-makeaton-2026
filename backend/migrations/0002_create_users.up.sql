-- One login per person. A CUSTOMER login always points at exactly one
-- customer; ADMIN and STAFF logins never do.

CREATE TABLE users (
    id                   uuid PRIMARY KEY,
    email                text NOT NULL,
    password_hash        text NOT NULL,
    full_name            text NOT NULL DEFAULT '',
    role                 text NOT NULL
                         CHECK (role IN ('ADMIN', 'STAFF', 'CUSTOMER')),
    status               text NOT NULL DEFAULT 'ACTIVE'
                         CHECK (status IN ('ACTIVE', 'DISABLED')),
    customer_id          uuid REFERENCES customers (id) ON DELETE RESTRICT,
    email_verified_at    timestamptz,
    must_change_password boolean NOT NULL DEFAULT false,
    last_login_at        timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_customer_role_chk
        CHECK ((role = 'CUSTOMER') = (customer_id IS NOT NULL))
);

CREATE UNIQUE INDEX users_email_lower_uq ON users (lower(email));
CREATE UNIQUE INDEX users_customer_uq ON users (customer_id)
    WHERE customer_id IS NOT NULL;
CREATE INDEX users_role_idx ON users (role);
CREATE INDEX users_status_idx ON users (status);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
