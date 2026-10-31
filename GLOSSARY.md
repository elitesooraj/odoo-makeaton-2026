# Glossary — Brightloop Subscription Management

Domain vocabulary for the platform. Terms here are the canonical names used in
code, migrations, docs and tests. When a new concept is introduced, add it
here first; the code follows the glossary, not the other way around.

## Core entities

- **Customer** — a company or person who buys a subscription. Has a
  billing address, tax id, and optional notes. Soft-deactivated, never
  deleted.
- **User** — an internal or portal login. Roles: ADMIN, STAFF, CUSTOMER.
  A CUSTOMER user is linked to exactly one Customer record.
- **Product** — a sellable thing (e.g. "Water Delivery"). Has variants.
- **Plan** — a billing cadence for a product: interval (DAILY/WEEKLY/MONTHLY/YEARLY),
  price per unit per interval, minimum quantity, auto-renew, auto-close,
  pause/early-close permissions, and an optional tax rule.
- **Subscription** — a customer's commitment to a plan. Carries its own
  quantity, discount, expiry date, payment terms, and a list of lines.
  Lifecycle: DRAFT -> QUOTATION -> CONFIRMED -> ACTIVE -> (PAUSED) -> CLOSED.
- **Line** — one item on a subscription or invoice: product, quantity,
  unit price, tax, discount share, line total. Lines are editable only
  while the subscription is DRAFT or QUOTATION.
- **Quotation** — a subscription in QUOTATION status with `quote_expires_at`
  set. A quotation template is a reusable line set; instantiating it creates
  a QUOTATION subscription.
- **Invoice** — a bill issued to a customer. DRAFT until confirmed (locked,
  numbered INV-NNNNNN). Carries lines, subtotal, discount, tax, total,
  balance. Balance = total - payments. Overdue is derived at read time.
- **Payment** — a recorded payment against a confirmed invoice. Method:
  CASH, BANK_TRANSFER, CARD, MOBILE_MONEY, OTHER.
- **Billing run** — one execution of the recurring billing cycle. Processes
  every ACTIVE subscription whose `next_billing_date` is due, creates a
  CONFIRMED invoice for the period, advances the date, and records the run.
  Idempotent via `billing_runs(subscription_id, period_start)` and
  `invoices(subscription_id, period_start)` unique keys.
- **Period** — a billing interval: [period_start, period_end). One invoice
  per subscription per period.

## Money

- **Minor units** — all money is integer cents. No floats anywhere.
- **Exclusive tax** — added on top of line totals (total grows).
- **Inclusive tax** — carved out of the line total (total unchanged,
  informational).
- **MRR** — monthly recurring revenue, approximated from the plan interval
  (yearly / 12, weekly x 52 / 12, daily x 30).

## Statuses

- **DRAFT** — editable, not yet a commitment.
- **QUOTATION** — a proposal sent to the customer, with expiry.
- **CONFIRMED** — locked, numbered, ready to bill or activate.
- **ACTIVE** — billing. Has a `next_billing_date`.
- **PAUSED** — billing suspended; `next_billing_date` cleared.
- **CLOSED** — terminal. No further edits or billing.

## Architecture terms

- **Module** — a Go package under `internal/` (e.g. subscriptions, invoices).
  Each owns its repository, service and HTTP handler.
- **Seam** — an interface that lets a module be tested or swapped without its
  real dependencies (e.g. `PlanLoader`, `TaxLoader`).
- **Adapter** — a thin struct converting one package's types to a neutral
  interface (e.g. `api.PlanAdapter`).
- **Deep module** — small interface, complex implementation (money, auth,
  subscriptions). Preferred.
- **Shallow module** — interface nearly as complex as implementation (thin
  CRUD wrappers). Acceptable for hackathon speed; refactor when they grow.
- **Aggregate** — a cluster of entities persisted together (subscription +
  lines). The service is the transaction boundary.
