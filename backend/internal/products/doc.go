// Package products owns products and their named variants.
//
// Status: scaffold. Experienced Member 2 implements handler.go, service.go,
// repository.go and model.go, using internal/customers as the reference module.
//
// Migrations: 0100-0199.
// Permissions: authz.ProductsRead / authz.ProductsWrite.
// Money: BIGINT minor units (cost_price_cents, sales_price_cents); a variant
// stores price_delta_cents, never an absolute price.
package products
