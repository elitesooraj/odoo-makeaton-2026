-- RBAC: a seeded catalog of permissions, role defaults, and per-user
-- GRANT/REVOKE overrides. Effective access is role defaults UNION grants
-- MINUS revokes; ADMIN bypasses the tables entirely.

CREATE TABLE permissions (
    key         text PRIMARY KEY,
    category    text NOT NULL,
    description text NOT NULL
);

CREATE TABLE role_permissions (
    role           text NOT NULL CHECK (role IN ('ADMIN', 'STAFF', 'CUSTOMER')),
    permission_key text NOT NULL REFERENCES permissions (key) ON DELETE CASCADE,
    PRIMARY KEY (role, permission_key)
);

CREATE TABLE user_permissions (
    user_id        uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    permission_key text NOT NULL REFERENCES permissions (key) ON DELETE CASCADE,
    effect         text NOT NULL CHECK (effect IN ('GRANT', 'REVOKE')),
    granted_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, permission_key)
);

CREATE INDEX user_permissions_user_idx ON user_permissions (user_id);
