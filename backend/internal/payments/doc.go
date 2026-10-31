// Package payments owns payment records against invoices.
//
// Status: scaffold. Experienced Member 2 implements this module.
//
// Migrations: 0300-0399.
// Permissions: authz.PaymentsRead / authz.PaymentsWrite.
//
// A payment stores method, amount_cents, paid_at and a note, and is always
// linked to one invoice. Recording a payment and updating the invoice balance
// must happen in the same transaction.
package payments
