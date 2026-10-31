# Brightloop Subscription Management System

Unified platform for plans, subscriptions and recurring billing. Modular
monolith: React + TypeScript + Vite frontend, Go + Gin API, PostgreSQL, Docker
Compose.

This repository has **Member 1's foundation** (project setup, database,
migrations, authentication, RBAC, password reset, Docker) and **Member 2's
business modules**: products, plans, taxes, discounts, subscriptions,
quotations, invoices, payments, billing runs, reports and the customer portal —
all routes implemented and smoke-tested, documented in
[`docs/API.md`](docs/API.md).

## Quick start

Docker is the only prerequisite; the host does not need Go, Node or a database
client. One command does everything — Postgres, migrations, seed, API, Mailpit:

```bash
cp .env.example .env
docker compose up -d --build
```

`docker compose up` also runs the `migrate` and `seed` one-shots before the API
starts (they exit when done; `docker compose ps` shows them as exited, which is
expected). The seed is idempotent: first boot on a fresh volume creates the
admin and finance users from `.env`, later boots see them and leave them
untouched. It never overwrites a password.

The `make` targets are thin wrappers around the same Compose commands:
`make up` = the two steps above, `make seed` re-runs the seed manually.

| URL | What |
| --- | --- |
| `http://localhost:8080/healthz` | API liveness |
| `http://localhost:8080/api/v1` | API |
| `http://localhost:8025` | Mailpit inbox (password reset / verification) |
| `127.0.0.1:5432` | PostgreSQL (`brightloop` / `brightloop`) |

Seed credentials come from `.env`: `admin@brightloop.test` and
`finance@brightloop.test`. The finance user is a STAFF account with
`invoices.write` and `payments.write` granted explicitly.

**Why seed at all?** `/auth/register` only ever creates CUSTOMER (portal)
accounts — self-service sign-up can never mint back-office roles, or anyone
could sign up and become admin. Only an admin can create staff, so the *first*
admin must come from outside the request path: that is the seed job's one job.
Once the frontend is ready, customers self-register via the UI and staff are
created from the admin area — the seed still runs on every boot, but after the
first it changes nothing.

```bash
curl -i -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@brightloop.test","password":"ChangeMe!Now1"}'

curl -b cookies.txt http://localhost:8080/api/v1/auth/me
```

## Documentation

- [`docs/problem-statement.md`](docs/problem-statement.md) — requirements,
  decisions taken, and unresolved conflicts.
- [`docs/CONVENTIONS.md`](docs/CONVENTIONS.md) — the contract every module
  follows: envelopes, errors, pagination, money, IDs, migrations, audit, RBAC.
- [`docs/api/openapi.yaml`](docs/api/openapi.yaml) — implemented endpoints.
- [`docs/API.md`](docs/API.md) — business-module route reference: permissions,
  payload shapes, lifecycle rules, portal scoping.
- [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) — production deployment, TLS,
  backups and the hardening checklist.
- [`docs/adr/`](docs/adr/) — architecture decisions.

## What exists

| Area | Status |
| --- | --- |
| Go module, config, structured logging, graceful shutdown | done |
| PostgreSQL pool, transaction helper, health/readiness | done |
| Numbered migrations with up/down | done |
| Customer self-registration + company record | done |
| Login/logout/me, server-side sessions | done |
| Password policy, reset link by email | done |
| Soft email verification | done |
| RBAC: catalog, role defaults, per-user grant/revoke | done |
| Staff creation with one-time temporary password | done |
| Customer CRUD with soft deactivate | done |
| Audit log for user/customer mutations | done |
| Docker Compose + Makefile | done |
| Products, plans, taxes, discounts, subscriptions, quotations, invoices, payments, billing, reports, portal | routes implemented (thin) |
| React application | placeholder (Frontend 1 and 2) |

## Layout

```text
backend/
  cmd/api/              HTTP entrypoint
  cmd/seed/             bootstrap admin + finance staff
  internal/
    platform/           config, db, httpx, pagination, validate, mailer, ratelimit, id
    auth/               sessions, password reset, verification, auth middleware
    authz/              permission catalog
    users/              admin management of internal users
    customers/          customer CRUD
    audit/              append-only audit writer
    api/                route composition
    products/ plans/ taxes/ discounts/ quotations/ billing/ reports/ portal/
                        business modules (Member 2, routes thin)
    subscriptions/ invoices/ payments/
  migrations/           0001-0099 foundation, 0100+ Member 2
frontend/               placeholder until Frontend Member 1 replaces it
```

## Development

Everything runs through Compose. Plain `docker compose up` starts postgres,
migrations, seed, api and mailpit; the `test` job sits behind the `tools`
profile and the frontend behind `frontend`.

```bash
make test      # go vet + unit tests inside the build image
make seed      # re-run the idempotent seed job manually
make logs      # follow API logs
make ps        # service status
make down      # stop, keep the database volume
make clean     # stop and delete the database volume
make config    # validate the Compose files
```

`make up` is `docker compose up -d --build`. The optional frontend placeholder
starts with `make frontend` and is served on `FRONTEND_PORT`.

Production uses the same build system with an override:

```bash
make up-prod        # hardened settings, no Mailpit, no automatic seed
make seed-prod      # once
```

See [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).

## Ownership

| Owner | Scope |
| --- | --- |
| Experienced Member 1 | `platform/`, `auth/`, `authz/`, `users/`, `customers/`, `audit/`, `api/`, migrations 0001-0099, Docker |
| Experienced Member 2 | `products/`, `plans/`, `taxes/`, `discounts/`, `subscriptions/`, `quotations/`, `invoices/`, `payments/`, `billing/`, `reports/`, `portal/`, `platform/money/`, `platform/number/`, migrations 0100+ |
| Frontend Member 1 | `frontend/` auth, products, plans, customers, subscriptions, portal |
| Frontend Member 2 | `frontend/` invoices, payments, taxes, discounts, reports |

Do not edit an applied migration. Add a new one in your range.
