# API Route Reference — Business Modules

Status: **all routes implemented and smoke-tested**. Deep business rules stay
thin by hackathon design: basic validation, working CRUD and lifecycle, no
esoteric edge cases. This document is the contract for the frontend team.

Conventions shared with the foundation (`docs/CONVENTIONS.md`):

- Base URL: `http://localhost:8080/api/v1`
- Auth: server-side session in the `bl_session` cookie (`HttpOnly; SameSite=Lax`).
  Browsers: `credentials: 'include'`. Curl: `curl -c/-b cookies.txt`.
- Every response is `{ data, meta }` or `{ error }`; list endpoints carry
  `meta.page`, `meta.per_page`, `meta.total`, `meta.total_pages`.
- List filter params: `page`, `per_page` (default `1` / `25`, max `100`).
- Money is integer minor units (`..._cents`). All timestamps UTC.
- Errors: `VALIDATION_FAILED` (422), `NOT_FOUND` (404), `CONFLICT` (409),
  `UNAUTHENTICATED` (401), `FORBIDDEN` (403), `INTERNAL` (500).

Roles: `ADMIN` (back office, bypasses permission checks), `STAFF` (back office,
collective permissions), `CUSTOMER` (portal only, read-only, identity-scoped).

## Quick route map

| Group | Path prefix | Permission pattern | Notes |
| --- | --- | --- | --- |
| Catalog | `/products`, `/plans`, `/taxes`, `/discounts` | `*.read` / `*.write` | Admin back office |
| Subscriptions | `/subscriptions` | `subscriptions.read` / `subscriptions.write` | + lifecycle actions, `lines` |
| Quotations | `/quotations/templates` | `subscriptions.read` / `subscriptions.write` | template CRUD + `instantiate` |
| Invoices | `/invoices` | `invoices.read` / `invoices.write` | + confirm/cancel/send |
| Payments | `/payments` | `payments.read` / `payments.write` | record against confirmed invoices |
| Billing | `/billing/run` | `subscriptions.write` | manual run trigger |
| Reports | `/reports` | `reports.read` | read-only aggregates |
| Portal | `/portal` | CUSTOMER role only | identity-scoped, read-only |

## Catalog

### Products

- `POST /products` — create. `{"name":"Water Delivery","description":"...", "is_active":true}`
- `GET /products` — list. Filters: `search`, `is_active`, `page`, `per_page`.
- `GET /products/:id`
- `PATCH /products/:id` — partial update.
- `DELETE /products/:id` — soft-deactivate (keeps history).
- `POST /products/:id/variants` — `{"name":"10L can","sku":"WTR-10L"}`
- `PATCH /products/:id/variants/:variant_id`
- `DELETE /products/:id/variants/:variant_id` — soft-deactivate.

### Plans

- `POST /plans` — `{"product_id":"<uuid>","name":"Weekly 10L","interval":"WEEKLY","price_cents":14900,"minimum_quantity":1,"formula":"FLAT","auto_renew":true,"auto_close":false,"allow_pause":true,"allow_early_close":true,"tax_id":"<uuid>","is_active":true}`
  - `interval`: `DAILY | WEEKLY | MONTHLY | YEARLY` (drives billing math).
  - `price_cents` is per interval per unit; tax is a `taxes` record id or `null`.
- `GET /plans` — list; filters: `product_id`, `is_active`, `search`.
- `GET /plans/:id`, `PATCH /plans/:id`, `DELETE /plans/:id` (soft-deactivate).

### Taxes

- `POST /taxes` — `{"name":"VAT 18","rate_bp":1800,"is_inclusive":false,"is_active":true}`
  - `rate_bp` is basis points: `1800` = 18%.
  - Exclusive taxes are added on top of line totals; inclusive taxes are carved
    out of the line total and shown informationally.
- `GET /taxes`, `GET /taxes/:id`, `PATCH /taxes/:id`, `DELETE /taxes/:id`.

### Discounts

- `POST /discounts` — `{"name":"Launch 10","kind":"PERCENT","value_bp":1000,"is_active":true}`
  - `kind`: `PERCENT` (`value_bp` = 10% as `1000`) or `FIXED` (`amount_cents`).
- `GET /discounts`, `GET /discounts/:id`, `PATCH /discounts/:id`, `DELETE /discounts/:id`.

## Subscriptions

Lifecycle: `DRAFT → QUOTATION → CONFIRMED → ACTIVE → (PAUSED ↔ ACTIVE) → CLOSED`.
Lines and fields like expiry date are editable **only** in `DRAFT`/`QUOTATION`.

- `POST /subscriptions` — create as DRAFT:
  ```json
  {"customer_id":"<uuid>","plan_id":"<uuid>","status":"DRAFT",
   "start_date":"2026-11-01","expiry_date":null,"payment_terms_days":30,
   "quantity":1,"notes":"first order"}
  ```
  `status` may be `DRAFT` or `QUOTATION` (quotation also sets `quote_expires_at`,
  default 30 days out).
- `GET /subscriptions` — list. Filters: `customer_id`, `plan_id`, `status`, `search`, `page`, `per_page`.
- `GET /subscriptions/:id` — includes `lines` and the computed amounts.
- `PATCH /subscriptions/:id` — edit while `DRAFT`/`QUOTATION`.
- `POST /subscriptions/:id/confirm` — `QUOTATION → CONFIRMED`.
- `POST /subscriptions/:id/activate` — sets `next_billing_date` (start date) and goes ACTIVE.
- `POST /subscriptions/:id/pause` — clears `next_billing_date` (plan must `allow_pause`).
- `POST /subscriptions/:id/resume` — recomputes `next_billing_date`.
- `POST /subscriptions/:id/close` — close (early or natural end).
- Lines (editable only while `DRAFT`/`QUOTATION`):
  ```json
  POST   /subscriptions/:id/lines
  {"product_id":"<uuid>","quantity":2,"unit_price_cents":14900,
   "tax_id":"<uuid>","description":"Weekly 10L"}

  PATCH  /subscriptions/:id/lines/:line_id
  DELETE /subscriptions/:id/lines/:line_id
  ```
  The response of every subscription mutation is the full subscription record.

## Quotations (templates)

A quotation *is* a subscription in `QUOTATION` status. Templates are reusable
line sets; expanding one creates the quote.

- `POST /quotations/templates` —
  ```json
  {"name":"Water weekly","validity_days":30,
   "lines":[{"plan_id":"<uuid>","quantity":3}], "notes":"optional"}
  ```
- `GET /quotations/templates`, `GET /quotations/templates/:id`
- `PATCH /quotations/templates/:id` — `{"name":"...","lines":[...],"is_active":true}`
- `DELETE /quotations/templates/:id` — soft-deactivate.
- `POST /quotations/templates/:id/instantiate` — creates the QUOTATION subscription:
  ```json
  {"customer_id":"<uuid>","start_date":"2026-11-01","payment_terms_days":30,
   "quantity":1,"notes":"optional"}
  ```
  Returns the new subscription in `QUOTATION` status with `quote_expires_at`.

## Invoices

Money model: `line_total = unit × qty − discount share`; exclusive tax is added
on top; inclusive tax is carved out. `total = Σ line_total + exclusive tax`,
`balance = total − payments`; overdue is derived at read time (`due_date <
today` and balance > 0).

- `POST /invoices` — create a DRAFT invoice, either snapshot a subscription
  (one source only):
  ```json
  {"customer_id":"<uuid>","subscription_id":"<uuid>"}
  {"customer_id":"<uuid>","lines":[{"description":"Setup fee","quantity":1,"unit_price_cents":50000}]}
  ```
  Optional: `issue_date`, `due_date`, `period_start`, `period_end`,
  `discount_id`, `notes`.
- `GET /invoices` — filters: `customer_id`, `subscription_id`, `status`, `search`.
- `GET /invoices/:id` — includes `lines`, totals, `balance_cents`.
- `PATCH /invoices/:id` — edit while DRAFT.
- `POST /invoices/:id/confirm` — DRAFT → CONFIRMED; locks totals; gets a number `INV-NNNNNN`.
- `POST /invoices/:id/cancel` — only if unpaid.
- `POST /invoices/:id/send` — queues the customer email (stub log when SMTP off).

## Payments

- `POST /payments` — record a payment against a confirmed invoice; returns the
  updated invoice (balance, PAID transition):
  ```json
  {"invoice_id":"<uuid>","amount_cents":30000,"method":"CARD",
   "paid_at":"2026-10-31T10:00:00Z","reference":"optional"}
  ```
  `method`: `CASH | CARD | BANK | UPI | OTHER`.
- `GET /payments` — filters: `invoice_id`, `customer_id`, `method`, `from`, `to`.
- `GET /payments/:id`

## Billing

`POST /billing/run` (permission `subscriptions.write`) processes every ACTIVE
subscription whose `next_billing_date` is due, in one run:

- Creates a CONFIRMED invoice for the period, advances `next_billing_date` by
  the plan interval, records the run.
- Idempotent: `billing_runs(subscription_id, period_start)` and
  `invoices(subscription_id, period_start)` are unique; replays report
  `SKIPPED` with a reason instead of double-billing.
- Non-renewing subscriptions are closed after their final period (`CLOSED`).
- Concurrency: a second overlapping run gets `409 CONFLICT`.
- A subscription whose first period is already invoiced ad hoc is skipped.

Response:
```json
{"as_of":"...","billed":1,"skipped":3,"closed":0,"failed":0,"invoices":["<uuid>"],
 "results":[{"subscription_id":"...","period_start":"2026-10-31T00:00:00Z",
             "period_end":"2026-11-07T00:00:00Z","invoice_id":"...","status":"CREATED"}]}
```

## Reports

All read-only; permission `reports.read`.

- `GET /reports/summary` — dashboard: counts by status, MRR (approximate: year
  plans / 12, weekly plans × 52/12, daily × 30), outstanding and overdue
  amounts, collected this month.
- `GET /reports/revenue?months=12` — paid vs invoiced per month (1–24 months).
- `GET /reports/subscriptions` — counts by subscription status.
- `GET /reports/overdue` — confirmed invoices past due with balance, oldest first.

## Customer portal

Role `CUSTOMER` only (a STAFF/ADMIN account without `customer_id` gets 403 on
the portal, and a CUSTOMER account gets 403 on the back-office routes). Every
portal route is scoped by the actor's `customer_id`; foreign resources return
`404` to avoid leaking existence. **Read-only.**

- `GET /portal/me` — landing counts: `subscriptions`, `active_status`,
  `invoices`, `unpaid_invoices`.
- `GET /portal/subscriptions` — own subscriptions, paginated.
- `GET /portal/subscriptions/:id` — own subscription with lines.
- `GET /portal/invoices`, `GET /portal/invoices/:id` — own invoices.
- `GET /portal/payments` — own payment history.

## Access matrix

| Route | ADMIN | STAFF (default perms) | STAFF (finance) | CUSTOMER |
| --- | --- | --- | --- | --- |
| Auth, `/auth/me`, change-password | ✅ | ✅ | ✅ | ✅ |
| Users, permissions | ✅ | ❌ | ❌ | ❌ |
| Customers CRUD | ✅ | ✅ (`customers.*`) | ❌ | ❌ |
| Products/plans/taxes/discounts | ✅ | ✅ read+write | read ❌ / write ❌ | ❌ |
| Subscriptions + lifecycle + lines | ✅ | ✅ (`subscriptions.*`) | ❌ | ❌ |
| Quotation templates + instantiate | ✅ | ✅ (`subscriptions.*`) | ❌ | ❌ |
| Invoices | ✅ | ✅ (`invoices.*`) | ✅ (explicitly granted) | ❌ |
| Payments | ✅ | ✅ (`payments.*`) | ✅ (explicitly granted) | ❌ |
| Billing run | ✅ | `subscriptions.write` required | ❌ | ❌ |
| Reports | ✅ | `reports.read` required | ❌ | ❌ |
| `/portal/*` | ❌ | ❌ | ❌ | ✅ |

## Seeded accounts & curl quickstart

```bash
# admin
curl -s -c admin.jar -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@brightloop.test","password":"<ADMIN_PASSWORD>"}'

# portal customer (self-service)
curl -s -c cust.jar -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"c@example.com","password":"Cust0mer!Pass",
       "full_name":"Jane Doe","company_name":"Acme Corp"}'
curl -s -b cust.jar http://localhost:8080/api/v1/portal/me
```

Full route list mirrors `backend/internal/api/router.go`; OpenAPI source of
truth: `docs/api/openapi.yaml`.
