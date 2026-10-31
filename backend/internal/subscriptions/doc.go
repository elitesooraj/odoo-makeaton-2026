// Package subscriptions owns subscriptions, subscription lines and their state
// machine (Draft -> Quotation -> Confirmed -> Active -> Closed).
//
// Status: scaffold. Experienced Member 2 implements this module and the billing
// worker described in docs/problem-statement.md section 3.
//
// Migrations: 0200-0299.
// Permissions: authz.SubscriptionsRead / authz.SubscriptionsWrite.
//
// The billing period fields on a subscription are the key Member 2 supplies to
// the invoice generator. Duplicate prevention belongs on a unique constraint,
// not on application memory.
package subscriptions
