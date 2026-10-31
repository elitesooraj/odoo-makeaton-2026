// Package plans owns subscription plans.
//
// Status: scaffold. Experienced Member 2 implements this module.
//
// Migrations: 0100-0199.
// Permissions: authz.PlansRead / authz.PlansWrite.
//
// A plan belongs to exactly one product (ADR-0003 context, decision D14) and
// carries interval, price, minimum quantity, availability dates, and the
// behavioural flags for auto-close, early close, pause and renewal.
package plans
