# AGENTS.md

## Project Overview

Build a unified Subscription Management System for Brightloop Software.

The system manages the full subscription lifecycle, including products, plans, customers, quotations, subscriptions, invoices, payments, taxes, discounts, and financial reporting.

The system has three primary user roles:

- **Admin** — full system access and can manage staff, products, plans, taxes, and discount rules.
- **Staff** — handles day-to-day customer and subscription operations with limited permissions.
- **Customer** — can access only their own subscriptions and invoices.

## Technology Stack

### Frontend

- React
- TypeScript
- Vite
- shadcn/ui

### Backend

- Go
- Gin

### Database

- PostgreSQL
- pgx for PostgreSQL connectivity

### Deployment

- Docker Compose

## Architecture Principles

1. Keep the application as a modular monolith.
2. Keep frontend and backend as separate applications.
3. Organize backend code by business domain.
4. Keep HTTP handlers thin; business logic belongs in services.
5. Keep database access isolated from business logic.
6. Use database transactions for critical financial operations.
7. Do not introduce microservices unless there is a concrete requirement.
8. Prefer simple, explicit solutions over unnecessary abstractions.

## Backend Structure

```text
backend/
├── cmd/
│   └── api/
├── internal/
│   ├── auth/
│   ├── users/
│   ├── products/
│   ├── plans/
│   ├── subscriptions/
│   ├── invoices/
│   ├── payments/
│   ├── taxes/
│   ├── discounts/
│   └── reports/
├── migrations/
└── Dockerfile
```

Each domain should generally separate:

- HTTP handlers
- Business logic/services
- Database repositories
- Domain models

## Frontend Structure

```text
frontend/
├── src/
│   ├── components/
│   ├── pages/
│   ├── features/
│   ├── lib/
│   └── api/
└── Dockerfile
```

Use shadcn/ui components where appropriate. Keep API communication separate from UI components.

## Business Rules

### Authentication

- Each email belongs to one account.
- Passwords must be more than eight characters and include uppercase, lowercase, and special characters.
- Users must be able to reset their passwords.
- Only admins can add new staff members.

### Subscription Lifecycle

Use explicit subscription states:

```text
Draft → Quotation → Confirmed → Active → Closed
```

State transitions must be validated by backend business logic.

### Billing

- Generate invoices from subscriptions automatically.
- Do not generate duplicate invoices for the same subscription and billing period.
- Calculate applicable taxes automatically.
- Apply valid discounts automatically.
- Use database transactions for invoice generation and related financial records.

### Invoice Lifecycle

```text
Draft → Confirmed → Paid
```

Invoices may also be cancelled according to the business requirements.

### Reporting

Support reporting for:

- Active subscriptions
- Revenue
- Payments
- Outstanding invoices
- Overdue invoices

## Development Rules

- Define API contracts before frontend integration.
- Add database migrations for schema changes.
- Do not modify existing migrations after they have been applied.
- Validate input at API boundaries.
- Enforce authorization on the backend, not only in the frontend.
- Do not trust client-provided prices or financial totals.
- Keep financial calculations deterministic and testable.
- Add tests for critical subscription, billing, invoice, discount, and payment logic.

## Team Ownership

### Experienced Member 1

Owns:

- Authentication and authorization
- Backend foundation
- Database setup
- Docker Compose and deployment
- Shared backend conventions

### Experienced Member 2

Owns:

- Subscription lifecycle
- Billing logic
- Invoice generation
- Payments and financial transactions

### Frontend Member 1

Owns:

- Authentication UI
- Product and plan management
- Customer and subscription screens

### Frontend Member 2

Owns:

- Invoice and payment screens
- Tax and discount screens
- Reporting dashboard

All members are responsible for integration testing of their features.
