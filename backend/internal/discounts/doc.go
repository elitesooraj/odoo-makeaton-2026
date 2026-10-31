// Package discounts owns discount rules and their redemption ledger.
//
// Status: scaffold. Experienced Member 2 implements this module.
//
// Migrations: 0100-0199.
// Permissions: authz.DiscountsRead / authz.DiscountsWrite (write is admin-only
// by default).
//
// A rule needs: fixed or percentage amount, minimum purchase/quantity, start
// and end dates, and a usage limit. Enforcing the limit requires an
// application/redemption table keyed by invoice or line. See
// docs/problem-statement.md section 3, item 5.
package discounts
