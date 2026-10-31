# Brightloop Subscription Management System

Unified platform for plans, subscriptions and recurring billing. Modular
monolith: React + TypeScript + Vite frontend, Go + Gin API, PostgreSQL, Docker
Compose.

This repository currently contains **Experienced Member 1's foundation**:
project setup, database connection, migrations, authentication, RBAC, password
reset, Docker, and the shared backend conventions. The business modules are
scaffolded for Member 2 to fill in.

## Quick start

Docker is the only prerequisite; the host does not need Go, Node or a database
client.

```bash
cp .env.example .env
make up          # builds images, migrates, starts api, seeds accounts
```

`make up` runs the `seed` job at the end, so the admin and finance users exist
on first boot. Re-run `make seed` at any time; it never overwrites a password.

| URL | What |
| --- | --- |
| `http://localhost:8080/healthz` | API liveness |
| `http://localhost:8080/api/v1` | API |
| `http://localhost:8025` | Mailpit inbox (password reset / verification) |
| `127.0.0.1:5432` | PostgreSQL (`brightloop` / `brightloop`) |

Seed credentials come from `.env`: `admin@brightloop.test` and
`finance@brightloop.test`. The finance user is a STAFF account with
`invoices.write` and `payments.write` granted explicitly.

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
  Member 2 appends new paths to the same file.
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
| Products, plans, subscriptions, quotations, billing, invoices, payments, taxes, discounts, reports | scaffold only (Member 2) |
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
    products/ plans/ subscriptions/ invoices/ payments/ taxes/ discounts/ reports/
                        Member 2 fills these in
  migrations/           0001-0099 foundation, 0100+ Member 2
frontend/               placeholder until Frontend Member 1 replaces it
```

## Development

Everything runs through Compose. The `tools` profile holds one-shot jobs;
`dev` holds Mailpit; `frontend` holds the web app.

```bash
make test      # go vet + unit tests inside the build image
make seed      # create the bootstrap accounts (idempotent)
make logs      # follow API logs
make ps        # service status
make down      # stop, keep the database volume
make clean     # stop and delete the database volume
make config    # validate the Compose files
```

`make up` starts Postgres, Mailpit, migrations and the API. The optional frontend
placeholder starts with `make frontend` and is served on `FRONTEND_PORT`.

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
| Experienced Member 2 | `products/`, `plans/`, `subscriptions/`, `invoices/`, `payments/`, `taxes/`, `discounts/`, `reports/`, migrations 0100+ |
| Frontend Member 1 | `frontend/` auth, products, plans, customers, subscriptions, portal |
| Frontend Member 2 | `frontend/` invoices, payments, taxes, discounts, reports |

Do not edit an applied migration. Add a new one in your range.
