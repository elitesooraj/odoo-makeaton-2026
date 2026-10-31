// Package quotations owns quotation templates and quotes that become
// subscriptions.
//
// Status: scaffold. Experienced Member 2 decides whether a quote is a separate
// entity or a subscription status, and records the decision in an ADR.
//
// Migrations: 0200-0299.
// Permissions: authz.SubscriptionsRead / authz.SubscriptionsWrite.
//
// A template carries plan and product lines plus a validity period in days.
// See the open conflict in docs/problem-statement.md section 3, item 3.
package quotations
