import { api, isOfflineError } from './client';
import { Invoice, ReportSummary, RevenuePoint } from './types';

export const reportsApi = {
  getSummary: async (): Promise<ReportSummary> => {
    try {
      const res = await api.get<ReportSummary>('/reports/summary');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return {
          active_subscriptions: 14,
          paused_subscriptions: 2,
          open_quotations: 3,
          mrr_cents: 1425000,           // $14,250.00 MRR
          outstanding_cents: 844880,     // $8,448.80
          overdue_invoices: 1,
          overdue_cents: 207680,         // $2,076.80
          collected_this_month_cents: 1245000, // $12,450.00
        };
      }
      throw err;
    }
  },

  getRevenue: async (months = 12): Promise<RevenuePoint[]> => {
    try {
      const res = await api.get<RevenuePoint[]>('/reports/revenue', { months });
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return [
          { month: 'Nov 25', paid_cents: 850000, invoiced_cents: 920000 },
          { month: 'Dec 25', paid_cents: 910000, invoiced_cents: 990000 },
          { month: 'Jan 26', paid_cents: 1120000, invoiced_cents: 1250000 },
          { month: 'Feb 26', paid_cents: 1190000, invoiced_cents: 1300000 },
          { month: 'Mar 26', paid_cents: 1340000, invoiced_cents: 1420000 },
          { month: 'Apr 26', paid_cents: 1245000, invoiced_cents: 1510000 },
        ];
      }
      throw err;
    }
  },

  getOverdue: async (): Promise<Invoice[]> => {
    try {
      const res = await api.get<Invoice[]>('/reports/overdue');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return [
          {
            id: 'inv-overdue-1',
            number: 'INV-2026-0002',
            customer_id: 'cust-demo-1',
            customer_name: 'Apex Retailers Ltd',
            status: 'CONFIRMED',
            issue_date: '2026-03-01T00:00:00Z',
            due_date: '2026-03-31T00:00:00Z',
            subtotal_cents: 176000,
            discount_cents: 0,
            tax_cents: 31680,
            total_cents: 207680,
            balance_cents: 207680,
            notes: 'Overdue growth subscription invoice',
            lines: [],
            created_at: '2026-03-01T00:00:00Z',
            updated_at: '2026-04-01T00:00:00Z',
          },
        ];
      }
      throw err;
    }
  },
};
