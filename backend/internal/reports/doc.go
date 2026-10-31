// Package reports owns revenue, subscription, payment and overdue reporting.
//
// Status: scaffold. Experienced Member 2 implements this module.
//
// Migrations: none preferred initially; add indexes through Member 1's range or
// a new 03xx migration if a materialized view is justified.
// Permissions: authz.ReportsRead.
//
// Define "revenue" explicitly (cash collected vs invoiced vs MRR). Every report
// endpoint paginates and filters server-side and must answer in under two
// seconds with thousands of subscriptions.
package reports
