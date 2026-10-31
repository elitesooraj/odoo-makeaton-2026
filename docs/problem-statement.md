# Problem Statement and Decisions

This file preserves the original brief and records every design decision the
team has taken against it. When a decision contradicts the brief, the conflict
is listed in section 3 and must be resolved explicitly.

## 1. Original brief (verbatim)

### Subscription Management System

Build one system to run plans, subscriptions and recurring billing in one place.

### 1) The Challenge

Your task is to build one unified software platform that runs the full life of a subscription
business, from setting up products and plans to sending invoices, collecting payments
and reporting revenue. Think of it as the digital backbone of a company that earns its
money every month, not just once. Read the scenes that follow as things that actually
happen in an ordinary billing month, and design the system that makes each of them easy.

### 2) Meet Brightloop Software

Brightloop Software sells everything on subscription: business software billed by the
month or the year, add-ons customers switch on and off, and a few products that come in
different variants. Customers range from small shops on a monthly plan to large
companies on yearly contracts. An admin sets up products, plans, taxes and discounts,
a small internal team handles customers day to day, and customers want to see and
manage their own subscriptions without having to email anyone.

How Things Run Today

- Customer subscriptions: Tracked in a spreadsheet
- Renewals: Remembered by whoever notices first
- Invoices: Typed up by hand every month
- Payments: Matched to invoices by scrolling through bank statements
- Revenue reports: Built manually at the end of each quarter

Renewals get missed, invoices go out wrong, and nobody can say how much recurring
revenue the company actually has.

### 3) A Month at Brightloop

The scenes below follow one billing month at Brightloop Software, from a new customer
creating an account to the finance lead reviewing the numbers. Every one of them is a
problem someone at the company faces today.

#### A new customer signs up

A small business finds Brightloop online and creates an account with their email and a
password. Each email can belong to only one account, and the password must be more
than eight characters, with an uppercase letter, a lowercase letter and a special
character. If they forget it later, a reset link should arrive at their registered email.
Not everyone who logs in should see the same things. The admin controls the whole
system, internal staff handle day-to-day work with limited access, and customers see
only their own subscriptions and invoices through a portal. Only the admin can add a
new member of staff.

#### Launching a new plan

Brightloop is launching a new product. The admin adds it with its name, type, sales price,
cost price and notes, and offers it in variants, for example by brand, where choosing the
Odoo brand adds 560 to the price.

Then comes the plan: what it costs and how often it bills (daily, weekly, monthly or
yearly), the minimum quantity, and the dates it is available. Some plans should close on
their own when they end, and others can be closed early, paused or renewed. The
admin decides which, plan by plan.

#### Turning a quote into a subscription

A sales rep is preparing an offer for a large customer. Instead of building it from scratch,
they start from a quotation template that already has the right plan and product lines, and
stays valid for a set number of days.

Each subscription gets its own number, a customer, a plan, a start date, an expiry date
and payment terms, with lines showing the product, quantity, unit price, taxes and
amount. It moves from Draft to Quotation to Confirmed to Active and finally to Closed,
and anyone on the team should be able to see where it stands.

#### Billing day

It is billing day, and hundreds of subscriptions are due at once. Nobody should be typing
invoices by hand. Each one should be created from its subscription, with the customer
details, product lines, taxes and totals already filled in.

Taxes are calculated automatically from rules the admin has set. The team can confirm,
cancel, send or print any invoice, and each invoice moves from Draft to Confirmed to
Paid. Nothing should be billed twice or skipped.

#### A customer asks for a better price

A customer wants a discount for taking more seats. Only the admin can create discount
rules: a fixed amount or a percentage, with a minimum purchase or quantity, start and
end dates, and a limit on how many times it can be used.

A discount can apply to specific products or to whole subscriptions. Once it applies, it
should appear on the next invoice on its own, without anyone having to remember it, and
stop when it expires or runs out.

#### The money comes in

Payments arrive by different methods and on different days. Each one needs to be
recorded with its method, amount, date and a note, against the right invoice.

At any moment, the team should be able to see which invoices are paid and which are
still outstanding, so overdue ones can be chased before they turn into lost revenue.

#### The finance lead, at the end of the month

How many subscriptions are active? How much revenue came in, and how much is still
owed? Which invoices are overdue? Today, answering those questions takes days.

The finance lead wants one place to see active subscriptions, revenue, payments and
overdue invoices, with filters and a summary view. With thousands of subscriptions on
the books, every page should still respond in under two seconds.

## 2. Decisions taken

| # | Decision | Rationale |
| --- | --- | --- |
| D1 | Single tenant. ADMIN and STAFF are Brightloop employees; CUSTOMER is a client. | The brief describes one company, not a SaaS reselling the platform. No `tenant_id`. |
| D2 | One login per client. `users.customer_id` is unique when present. | "Each email can belong to only one account" plus "customers see only their own". |
| D3 | A customer record may exist before the client has a portal login. | Sales must be able to quote a prospect; the login is linked later. |
| D4 | Server-side sessions in an httpOnly cookie, not JWT. | Instant revocation when staff is disabled, simpler RBAC, one deployment. |
| D5 | Hybrid RBAC: seeded role defaults plus per-user GRANT/REVOKE overrides. | Staff needs sensible defaults, but an admin must be able to widen or narrow one person. |
| D6 | STAFF defaults: customers and subscriptions read/write; catalog, tax and discount read-only; invoices and payments read-only; reports read. | Matches "only the admin creates rules" and keeps money writes opt-in. |
| D7 | Finance staff get `invoices.write` and `payments.write` as explicit grants; the seed command creates that user. | The brief says the team sends invoices and records payments; a read-only default would break the demo. |
| D8 | Soft email verification: login is immediate, `email_verified` is exposed and the UI shows a banner. | Verification is not required by the brief; the account should be usable at once. |
| D9 | Email is sent only for password reset (required) and verification (soft). Staff onboarding returns a one-time temporary password instead of an invite email. | "A reset link should arrive at their registered email" is the only explicit email requirement. |
| D10 | Password policy: longer than 8 characters, upper, lower, special, at most 72 bytes. | Verbatim from the brief plus bcrypt's 72-byte limit. |
| D11 | UUIDv7 primary keys generated in Go; `TIMESTAMPTZ` in UTC; `BIGINT` minor units; `text` + `CHECK` instead of Postgres enums. | Sortable IDs, exact money, no enum migration pain. |
| D12 | Customers are soft-deactivated, never deleted. Financial rows are immutable. | Invoices and subscriptions reference customers forever. |
| D13 | Product variants are named options with an additive price delta (`+560`). | Matches the brand example without a full attribute matrix. |
| D14 | A plan belongs to exactly one product. | "The admin adds [a product] ... then comes the plan". Member 2 owns the schema. |
| D15 | Billing, subscriptions, invoices, payments, taxes and discounts are Member 2's modules. Member 1 provides the schema conventions and the `customers`/`users` tables they join to. | Work split. |
| D16 | Migration number ranges are reserved per owner; applied migrations are never edited. | Four people on one database. |
| D17 | Automatic billing runs in-process with database locking and a unique `(subscription_id, period_start)` guard. | Exactly-once without extra infrastructure. Member 2 implements it. |

## 3. Open conflicts and gaps

These are unresolved on purpose. They must be settled by the owner before the
affected module ships.

1. **Staff read-only finance vs. the brief.** The brief says "the team can
   confirm, cancel, send or print any invoice" and that payments are recorded by
   the team. Our STAFF default is read-only, resolved by explicit grants (D7).
   Consequence: the demo must use the seeded finance user or an admin for invoice
   and payment actions. Owner: Member 1 + Member 2.
2. **Invoice lifecycle is incomplete.** The brief says Draft → Confirmed → Paid
   and "cancel according to requirements". Partially paid, cancelled, overdue and
   refunds/credit notes are undefined. Recommendation: add `CANCELLED`, derive
   paid/partially-paid/overdue from the balance and due date, and model refunds
   as negative payments or credit notes. Owner: Member 2.
3. **Quotations have no entity in `architecture.md`.** The brief needs quotation
   templates with validity days and a quote that becomes a subscription. The docs
   only have a `Quotation` subscription status. Recommendation: `quotation_templates`
   plus a `quotes` table, or a documented status-only model. Owner: Member 2.
4. **Tax rules are undefined.** "Rules the admin has set" does not say whether tax
   is per product, per customer region, inclusive or exclusive, compound, or
   multiple per line. Recommendation: `taxes(rate_bp, is_inclusive, country_code)`
   attached to products, snapshotted onto invoice lines. Owner: Member 2.
5. **Discount redemption is not modelled.** A usage limit cannot be enforced
   without a ledger. Recommendation: `discount_applications(discount_id, invoice_id,
   subscription_id, amount_cents)` with a unique key per invoice/line. Owner:
   Member 2.
6. **No scheduler exists in the architecture diagram.** Recommendation: a Go
   worker in `cmd/api` (or a separate `cmd/worker`) using `SELECT ... FOR UPDATE
   SKIP LOCKED`, writing a `billing_runs` row per attempt. Owner: Member 2.
7. **Plan close/pause/renew flags are behavioural, not just fields.** The brief
   says some plans close automatically, others can be closed early, paused or
   renewed, decided per plan. The state machine needs these flags and explicit
   transitions. Owner: Member 2.
8. **Product cost price is mentioned but no margin report is specified.** Keep
   `cost_price_cents` on products so margin reporting is possible later. Owner:
   Member 2.
9. **"Revenue" is ambiguous.** Cash collected, invoiced revenue and MRR/ARR are
   different numbers. The report endpoint must state which one it returns. Owner:
   Member 2 + Frontend Member 2.
10. **Two-second budget with thousands of subscriptions.** List endpoints must
    paginate and filter server-side, and the reporting queries need indexes.
    Owner: Member 2 with Member 1's conventions.

## 4. Scope of this foundation

Implemented: project setup, configuration, PostgreSQL pool, migrations, auth,
sessions, password reset, soft verification, RBAC, staff management, customer
CRUD, audit, Docker, seed, conventions, OpenAPI stub.

Not implemented: products, plans, subscriptions, quotations, billing, invoices,
payments, taxes, discounts, reports, and the React application.
