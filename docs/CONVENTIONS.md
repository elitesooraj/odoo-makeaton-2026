# Backend Conventions

Everything here is already implemented in the foundation or enforced by code
review. Member 2 must follow it; Frontend members code against it.

## 1. Ownership and package boundaries

```text
internal/platform/   shared primitives (Member 1)
internal/auth/       sessions, passwords, tokens, middleware (Member 1)
internal/authz/      permission catalog (Member 1)
internal/users/      internal user management (Member 1)
internal/customers/  customer CRUD (Member 1)
internal/audit/      audit writer (Member 1)
internal/api/        route composition (Member 1)
internal/<domain>/   business modules (Member 2)
```

Each domain exposes `handler.go`, `service.go`, `repository.go`, `model.go`.
`api/router.go` mounts it with a permission middleware per route. Do not add
business rules to `api/`.

## 2. Identifiers, time, money

| Concern | Rule |
| --- | --- |
| Primary keys | `uuid` columns, UUIDv7 generated in Go with `id.New()` |
| Timestamps | `timestamptz`, UTC, `now()` in SQL |
| Money | `BIGINT` minor units (cents). Never `float`, never `NUMERIC` arithmetic in Go |
| Currency | one currency from `CURRENCY`; do not add per-row currency without an ADR |
| Enums | `text` + `CHECK`, not Postgres `ENUM` types |
| Deletion | catalog rows deactivate via `is_active`/`status`; financial rows are never deleted |

Never trust a price or total sent by the client. The server recomputes every
financial value from stored data.

## 3. Migrations

Format: `NNNN_snake_case.up.sql` and `NNNN_snake_case.down.sql`. The down file
must fully reverse the up file.

| Range | Owner |
| --- | --- |
| `0001-0099` | Member 1 (foundation) |
| `0100-0199` | Member 2 — products, variants, plans, taxes, discounts |
| `0200-0299` | Member 2 — subscriptions, quotation templates |
| `0300-0399` | Member 2 — invoices, invoice lines, payments |

Rules:

- Applied migrations are immutable. Fix forward with a new migration.
- Every foreign key needs an index when it will be filtered or joined often.
- Every table gets `created_at`; mutable tables get `updated_at`.
- The shared `set_updated_at()` trigger already exists.

Create a pair:

```bash
make migrate-create name=product_variants
# then rename 000008_* to 0100_product_variants.*
```

## 4. HTTP contract

Base path: `/api/v1`. JSON only.

Success:

```json
{ "data": { "...": "..." }, "meta": { "request_id": "..." } }
```

List:

```json
{
  "data": [],
  "meta": {
    "page": 1,
    "per_page": 25,
    "total": 412,
    "total_pages": 17,
    "request_id": "..."
  }
}
```

Error:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "one or more fields are invalid",
    "details": { "email": "enter a valid email address" },
    "request_id": "..."
  }
}
```

| Code | Status | When |
| --- | --- | --- |
| `BAD_REQUEST` | 400 | malformed path/query |
| `VALIDATION_FAILED` | 422 | body or field rules failed |
| `UNAUTHENTICATED` | 401 | no or expired session |
| `FORBIDDEN` | 403 | authenticated but not allowed |
| `NOT_FOUND` | 404 | entity or route missing |
| `CONFLICT` | 409 | uniqueness or state conflict |
| `RATE_LIMITED` | 429 | too many attempts |
| `INTERNAL` | 500 | unexpected; details are logged, not returned |

Build errors with `httpx.Validation`, `httpx.Conflict`, etc. Never write
`c.JSON(500, ...)` directly.

## 5. Pagination, filtering, sorting

Use `pagination.Parse(c)` and `pagination.Meta(page, total)`. Default 25, max
100. Filters are query parameters and always applied server-side:
`?search=&status=&page=&per_page=`.

Large tables need an index on the filtered column. Reporting may use keyset
pagination later, but the list envelope stays the same.

## 6. Authentication

- Server-side sessions. The cookie `bl_session` is `HttpOnly`, `SameSite=Lax`,
  and `Secure` in production.
- The frontend must send `credentials: 'include'`; there is no bearer token.
- Login is rate limited per IP + email. Password reset is rate limited per IP.
- Password policy: length > 8, at least one uppercase, one lowercase, one
  special character, at most 72 bytes.
- A failed login always returns the same message, and the service performs a
  dummy bcrypt comparison so timing does not reveal registered emails.
- Password reset and email verification tokens are 256-bit, stored as SHA-256,
  single-use, and replaced when reissued.
- On password reset every session is revoked.

Endpoints:

```text
POST /auth/register          public
POST /auth/login             public
POST /auth/logout            public (clears the cookie)
POST /auth/forgot-password   public, always 202
POST /auth/reset-password    public
POST /auth/verify-email      public
GET  /auth/me                session
POST /auth/change-password   session
POST /auth/resend-verification session
```

## 7. Authorization

`role` is `ADMIN`, `STAFF` or `CUSTOMER`. Effective permissions are:

```text
role_permissions(role)  ∪  user_permissions(effect = GRANT)
                        −  user_permissions(effect = REVOKE)
```

ADMIN bypasses the tables. CUSTOMER has no back-office permissions; portal
endpoints must still scope every query by the actor's `customer_id` in the
repository, not only in the handler.

Check permissions at the route:

```go
router.GET("/invoices", auth.RequirePermission(authz.InvoicesRead), handler.List)
```

Inside services use `actor.Can(authz.InvoicesWrite)`. Never compare role strings
except with `authz.RoleCustomer` for portal routing.

Default STAFF permissions are seeded in migration `0006`. Finance writes are
granted per user; the seed command does this for the finance staff account.

## 8. Transactions and audit

Any mutation that touches more than one row, or that must be consistent with an
audit record, uses `db.WithTx`:

```go
err := db.WithTx(ctx, pool, func(tx pgx.Tx) error {
    if err := repo.Update(ctx, tx, entity); err != nil { return err }
    return audit.Record(ctx, tx, audit.Event{
        ActorID: &actor.UserID,
        ActorRole: actor.Role,
        Action: "invoice.cancel",
        EntityType: "invoice",
        EntityID: entity.ID,
        Before: before,
        After: after,
        Meta: meta,
    })
})
```

Repositories accept `db.Querier`, which is satisfied by both `*pgxpool.Pool`
and `pgx.Tx`. That is what lets the same method run inside or outside a
transaction.

Audit actions use `entity.verb`: `customer.create`, `user.update`,
`invoice.cancel`.

After an INSERT or UPDATE, re-read the row inside the same transaction and put
that value in `After`. The database fills `created_at`/`updated_at`, so the Go
struct you sent to the INSERT still has zero timestamps; recording it directly
would store `0001-01-01T00:00:00Z` in the audit snapshot. `users.CreateStaff`,
`users.Update` and the customer mutations already do this.

## 9. Financial snapshots (required by Member 2)

Invoice lines must store the values used when the invoice was generated:

- `product_name`
- `variant_name`
- `quantity`
- `unit_price_cents`
- `tax_rate_bp` (basis points, e.g. 750 = 7.5%)
- `tax_cents`
- `discount_cents`
- `line_total_cents`

Never join live product or plan prices when rendering an old invoice. A plan
price change must not rewrite history.

Billing idempotency belongs to Member 2, but the schema must support it with a
unique constraint such as:

```sql
UNIQUE (subscription_id, period_start)
```

Overdue is derived (`due_date < now() AND balance_cents > 0`), not a stored
status.

## 10. Adding a module

```go
// internal/widgets/handler.go
type Handler struct{ svc *Service }

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
    rg.GET("", func(c *gin.Context) { /* ... */ })
    rg.POST("", func(c *gin.Context) { /* ... */ })
}
```

In `api/router.go`:

```go
widgets := protected.Group("/widgets")
widgets.GET("", auth.RequirePermission(authz.WidgetsRead), deps.WidgetsHandler.List)
```

Add the permission to `authz.Catalog` **and** a new migration that inserts it
into `permissions` and `role_permissions`. The catalog is code-owned; the seed
migration is its database mirror.

## 11. Build and run

Docker Compose is the build system. Do not document or rely on a host Go or Node
toolchain.

```bash
make up        # build + start dev stack, then seed
make build     # images only
make test      # go vet + go test in the test stage
make logs      # api logs
make config    # validate Compose
```

- `backend/Dockerfile` has `deps`, `test`, `build` and `runtime` stages. The
  runtime image is non-root and ships CA certificates and `tzdata`.
- `api` and `seed` share the same image; `seed` is a one-shot under the `tools`
  profile.
- Long-running services never run migrations themselves. The `migrate` service
  must exit 0 before `api` starts.
- Production adds `docker-compose.prod.yml`. It forces `APP_ENV=production`,
  secure cookies and real SMTP, and keeps Mailpit out.
- All state is in the `postgres_data` volume. `make clean` deletes it.

## 12. Testing

- Pure unit tests for validation, money, tax, discount and state machines.
- Integration tests use the Compose Postgres; never mock the database for
  financial behaviour.
- Every authorization rule needs a negative test (a staff member denied an
  admin endpoint, a customer denied another customer's invoice).
- `make test` must pass before merging.

## 13. Frontend contract

- `fetch(..., { credentials: 'include' })` on every call.
- Bootstrap the session from `GET /auth/me`: it returns the user and the
  effective permission list. Render navigation from permissions, not role.
- `401` means redirect to login. `403` means show a forbidden state.
- `must_change_password: true` must force the change-password screen.
- `email_verified: false` shows a non-blocking banner.
- CORS only allows `CORS_ALLOWED_ORIGINS`; cookies require an exact origin.
