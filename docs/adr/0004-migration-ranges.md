# ADR 0004: Reserved migration ranges, forward-only

## Status

Accepted.

## Context

Four people work on one database. Sequential migration numbers collide, and
editing an applied migration corrupts every environment that already ran it.

## Decision

Numbers are reserved per owner:

| Range | Owner |
| --- | --- |
| 0001-0099 | Member 1: customers, users, auth, RBAC, audit |
| 0100-0199 | Member 2: products, variants, plans, taxes, discounts |
| 0200-0299 | Member 2: subscriptions, quotation templates |
| 0300-0399 | Member 2: invoices, invoice lines, payments |

Every migration has a real down file. Applied migrations are immutable; fixes
are new migrations. Migrations run through golang-migrate in the `migrate`
Compose service before the API starts. The API never migrates itself.

## Consequences

- No merge conflicts on file numbers.
- A failed production migration is fixed forward, never by editing history.
- The `migrate` service must finish successfully before `api` starts; Compose
  enforces this with `service_completed_successfully`.
