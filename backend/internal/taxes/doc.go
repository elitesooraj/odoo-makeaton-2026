// Package taxes owns tax rules.
//
// Status: scaffold. Experienced Member 2 implements this module.
//
// Migrations: 0100-0199.
// Permissions: authz.TaxesRead / authz.TaxesWrite (write is admin-only by
// default).
//
// Decide and document: rate in basis points, inclusive vs exclusive, whether a
// rule is per product or per region, and whether multiple taxes can apply to
// one line. See docs/problem-statement.md section 3, item 4.
package taxes
