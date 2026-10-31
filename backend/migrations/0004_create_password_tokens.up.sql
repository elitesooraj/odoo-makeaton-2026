-- Single-use tokens for password reset and email verification. Tokens are
-- hashed at rest and invalidated when a new one is issued for the same purpose.

CREATE TABLE password_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('RESET', 'VERIFY')),
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX password_tokens_user_idx ON password_tokens (user_id, purpose);
CREATE INDEX password_tokens_expires_idx ON password_tokens (expires_at);
