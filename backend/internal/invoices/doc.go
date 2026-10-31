// Package invoices owns invoices and invoice lines.
//
// Status: scaffold. Experienced Member 2 implements this module.
//
// Migrations: 0300-0399.
// Permissions: authz.InvoicesRead / authz.InvoicesWrite.
//
// Invoice lines must snapshot product name, variant, quantity, unit price,
// tax rate in basis points, tax amount, discount amount and line total. Never
// read live catalog prices when rendering an existing invoice. Overdue is
// derived from due_date and balance, not stored.
package invoices
