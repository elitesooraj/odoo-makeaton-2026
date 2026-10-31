# ADR 0003: UUIDv7, integer money, text enums

## Status

Accepted.

## Decision

- **IDs**: UUIDv7 generated in Go (`id.New()`), stored in `uuid` columns. IDs
  are known before INSERT, so parent and child rows can be created in one
  transaction without `RETURNING`.
- **Time**: `timestamptz`, always UTC. The company timezone is used only for
  display and report bucketing.
- **Money**: `BIGINT` minor units (cents). No floats anywhere. Currency is fixed
  per deployment via `CURRENCY`.
- **Enums**: `text` with `CHECK` constraints.

## Consequences

- UUIDv7 keeps B-tree insert locality close to sequential IDs while remaining
  unguessable.
- Integer cents make totals deterministic and testable; formatting belongs to
  the frontend.
- Adding an enum value is a constraint change, not an `ALTER TYPE` migration.
- Multi-currency would require an ADR and FX tables; it is out of scope.
