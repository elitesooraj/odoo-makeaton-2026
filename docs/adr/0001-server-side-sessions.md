# ADR 0001: Server-side sessions instead of JWT

## Status

Accepted.

## Context

The platform has one web client, three roles, and a hard requirement that
disabling a staff member takes effect immediately. JWT access tokens would
require a revocation list or very short lifetimes plus refresh rotation.

## Decision

Authentication uses an opaque 256-bit token stored as SHA-256 in `sessions`,
delivered in an `HttpOnly`, `SameSite=Lax` cookie. The cookie is `Secure` in
production. Expiry is absolute (default 7 days). Revocation is a single UPDATE.

CSRF is covered by `SameSite=Lax` plus an Origin allowlist middleware on every
non-GET request.

## Consequences

- Every authenticated request costs one indexed session lookup and one
  permission resolution. Both are cacheable later if needed.
- Horizontal scaling works because sessions live in Postgres; no sticky
  sessions are required.
- Mobile or third-party API clients would need a bearer-token variant. Not
  required now.
