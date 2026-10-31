# ADR 0002: Hybrid RBAC with role defaults and per-user overrides

## Status

Accepted.

## Context

Three roles are enough to describe the product, but "staff has limited access"
is not precise enough for finance: some staff send invoices and record payments,
most do not. Fixed role checks in code would force a code change per person.
A full permission builder from day one would slow the team down.

## Decision

Role defaults live in `role_permissions`, seeded by migration. Individual
exceptions live in `user_permissions` with `GRANT` or `REVOKE`. Effective access
is:

```text
role_permissions(role) ∪ grants − revokes
```

ADMIN bypasses the tables. CUSTOMER has no back-office permissions; portal
access is identity-scoped. All checks go through `auth.Actor.Can` and the
`RequirePermission` middleware.

## Consequences

- Staff ships with useful defaults and no setup.
- An admin can give one finance user `invoices.write` without changing code.
- Permission resolution is one SQL query per request, cached in the request
  context.
- Every new permission requires a catalog entry and a migration.
