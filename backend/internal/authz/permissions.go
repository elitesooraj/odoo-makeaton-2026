// Package authz owns the permission catalog and role names.
//
// Authorization is intentionally two-layered:
//   - role_permissions holds the defaults for each role,
//   - user_permissions holds per-user GRANT/REVOKE overrides.
//
// Every check goes through auth.Actor.Can, which applies the admin bypass and
// the effective set resolved at authentication time. Handlers must never test
// role strings directly except for "is this the customer portal" checks.
package authz

const (
	RoleAdmin    = "ADMIN"
	RoleStaff    = "STAFF"
	RoleCustomer = "CUSTOMER"
)

const (
	UsersRead  = "users.read"
	UsersWrite = "users.write"
	AuditRead  = "audit.read"

	CustomersRead  = "customers.read"
	CustomersWrite = "customers.write"

	ProductsRead  = "products.read"
	ProductsWrite = "products.write"

	PlansRead  = "plans.read"
	PlansWrite = "plans.write"

	TaxesRead  = "taxes.read"
	TaxesWrite = "taxes.write"

	DiscountsRead  = "discounts.read"
	DiscountsWrite = "discounts.write"

	SubscriptionsRead  = "subscriptions.read"
	SubscriptionsWrite = "subscriptions.write"

	InvoicesRead  = "invoices.read"
	InvoicesWrite = "invoices.write"

	PaymentsRead  = "payments.read"
	PaymentsWrite = "payments.write"

	ReportsRead = "reports.read"
)

type Permission struct {
	Key         string `json:"key"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// Catalog is the source of truth for permission names. It must match the rows
// seeded by migration 0006; adding a permission means adding it here and in a
// new migration.
var Catalog = []Permission{
	{UsersRead, "users", "View internal users"},
	{UsersWrite, "users", "Create and manage internal users"},
	{AuditRead, "audit", "View the audit log"},

	{CustomersRead, "customers", "View customers"},
	{CustomersWrite, "customers", "Create and manage customers"},

	{ProductsRead, "catalog", "View products and variants"},
	{ProductsWrite, "catalog", "Create and manage products and variants"},

	{PlansRead, "catalog", "View plans"},
	{PlansWrite, "catalog", "Create and manage plans"},

	{TaxesRead, "finance", "View tax rules"},
	{TaxesWrite, "finance", "Create and manage tax rules"},

	{DiscountsRead, "finance", "View discount rules"},
	{DiscountsWrite, "finance", "Create and manage discount rules"},

	{SubscriptionsRead, "subscriptions", "View subscriptions"},
	{SubscriptionsWrite, "subscriptions", "Create and manage subscriptions"},

	{InvoicesRead, "finance", "View invoices"},
	{InvoicesWrite, "finance", "Issue, cancel and send invoices"},

	{PaymentsRead, "finance", "View payments"},
	{PaymentsWrite, "finance", "Record payments"},

	{ReportsRead, "reports", "View revenue and subscription reports"},
}

// Keys returns the catalog keys, used to validate admin-supplied overrides.
func Keys() map[string]struct{} {
	keys := make(map[string]struct{}, len(Catalog))
	for _, p := range Catalog {
		keys[p.Key] = struct{}{}
	}
	return keys
}
