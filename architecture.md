# ARCHITECTURE.md

# Subscription Management System Architecture

## Overview

The Subscription Management System is designed as a modular monolith.

The architecture consists of:

- A React frontend
- A Go backend API
- A PostgreSQL database
- Docker Compose for local and production deployment

```text
                        ┌─────────────────────┐
                        │  React + TypeScript │
                        │       + Vite        │
                        │      shadcn/ui      │
                        └──────────┬──────────┘
                                   │
                                HTTP/JSON
                                   │
                        ┌──────────▼──────────┐
                        │      Go Backend     │
                        │        Gin          │
                        │                     │
                        │  Modular Monolith   │
                        ├─────────────────────┤
                        │ Auth & Users        │
                        │ Products & Plans    │
                        │ Customers           │
                        │ Subscriptions       │
                        │ Invoices            │
                        │ Payments            │
                        │ Taxes & Discounts   │
                        │ Reports             │
                        └──────────┬──────────┘
                                   │
                                  pgx
                                   │
                        ┌──────────▼──────────┐
                        │     PostgreSQL      │
                        └─────────────────────┘
```

## Frontend Architecture

The frontend is a single-page React application built with TypeScript and Vite.

Primary areas:

- Authentication
- Admin dashboard
- Product and plan management
- Customer management
- Subscription management
- Invoice and payment management
- Tax and discount management
- Reporting dashboard
- Customer portal

Use shadcn/ui as the component library.

The frontend should not contain critical authorization or financial business rules. Those rules must be enforced by the backend.

## Backend Architecture

The Go backend uses Gin to expose a JSON HTTP API.

The backend is organized into domain modules rather than technical layers at the top level.

Example:

```text
internal/
├── auth/
├── users/
├── products/
├── plans/
├── customers/
├── subscriptions/
├── invoices/
├── payments/
├── taxes/
├── discounts/
└── reports/
```

Within each domain:

```text
domain/
├── handler.go
├── service.go
├── repository.go
└── model.go
```

### Request Flow

```text
HTTP Request
     │
     ▼
Gin Router
     │
     ▼
Handler
     │
     ▼
Service / Business Logic
     │
     ▼
Repository
     │
     ▼
PostgreSQL
```

## Database Architecture

PostgreSQL is the primary data store.

Core entities include:

```text
Users
Customers
Products
ProductVariants
Plans
Subscriptions
SubscriptionLines
Invoices
InvoiceLines
Payments
Taxes
Discounts
```

Relationships should preserve historical financial information. For example, invoice lines should store the price and tax values used when the invoice was generated rather than relying only on the current product price.

## Key Business Flows

### Customer Registration

```text
Customer
   ↓
Register Account
   ↓
Validate Email and Password
   ↓
Create User
   ↓
Assign CUSTOMER Role
```

### Subscription Lifecycle

```text
Draft
  ↓
Quotation
  ↓
Confirmed
  ↓
Active
  ↓
Closed
```

The backend validates every transition.

### Invoice Generation

```text
Active Subscription
        ↓
Check Billing Date
        ↓
Create Invoice
        ↓
Copy Subscription Lines
        ↓
Apply Taxes and Discounts
        ↓
Calculate Total
        ↓
Save Invoice Transactionally
```

The system must prevent duplicate invoices for the same subscription and billing period.

### Payment Flow

```text
Payment Recorded
       ↓
Validate Invoice
       ↓
Save Payment
       ↓
Update Outstanding Balance
       ↓
Mark Invoice Paid When Fully Settled
```

## API Design

Use REST-style endpoints.

Example resource groups:

```text
/auth
/users
/customers
/products
/plans
/subscriptions
/invoices
/payments
/taxes
/discounts
/reports
```

All protected endpoints must verify both authentication and authorization.

## Deployment

Docker Compose manages the application services:

```text
docker-compose.yml
│
├── frontend
├── backend
└── postgres
```

Typical request flow:

```text
Browser
   │
   ▼
Frontend Container
   │
   ▼
Backend Container
   │
   ▼
PostgreSQL Container
```

## Performance

Reporting and list pages must remain responsive with thousands of subscriptions.

Use:

- Database indexes on frequently filtered fields
- Pagination for large lists
- Efficient SQL queries
- Server-side filtering and sorting

## Architecture Decisions

### Why a Modular Monolith?

The team has four members and the system is a single business application. A modular monolith:

- Reduces deployment complexity
- Simplifies database transactions
- Makes development faster
- Keeps domain boundaries clear
- Can be split later if scaling requirements demand it

Microservices are not currently justified.
