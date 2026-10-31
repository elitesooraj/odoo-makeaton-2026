import { api, isOfflineError } from './client';
import { Invoice, Payment, PortalMe, Subscription } from './types';

export const portalApi = {
  getMe: async (): Promise<PortalMe> => {
    try {
      const res = await api.get<PortalMe>('/portal/me');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return {
          customer_id: 'cust-demo-1',
          email: 'client@apexretail.test',
          full_name: 'Apex Retail Admin',
          subscriptions: 2,
          active_status: 1,
          invoices: 2,
          unpaid_invoices: 1,
        };
      }
      throw err;
    }
  },

  listSubscriptions: async (): Promise<Subscription[]> => {
    try {
      const res = await api.get<Subscription[]>('/portal/subscriptions');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return [
          {
            id: 'sub-001',
            number: 'SUB-2026-0001',
            subscription_number: 'SUB-2026-0001',
            customer_id: 'cust-demo-1',
            customer_name: 'Apex Retailers Ltd',
            plan_id: 'plan-001',
            plan_name: 'Monthly Growth Plan',
            status: 'ACTIVE',
            start_date: '2026-03-01T00:00:00Z',
            expiry_date: null,
            payment_terms_days: 30,
            quantity: 1,
            next_billing_date: '2026-05-01T00:00:00Z',
            subtotal_cents: 176000,
            tax_cents: 31680,
            total_cents: 207680,
            lines: [
              {
                id: 'subline-1',
                subscription_id: 'sub-001',
                product_id: 'prod-001',
                product_name: 'BrightERP Core Platform',
                variant_name: 'Odoo Connected Brand (+560.00)',
                description: 'Core system with connector',
                quantity: 1,
                unit_price_cents: 176000,
                tax_rate_bp: 1800,
                tax_cents: 31680,
                discount_cents: 0,
                total_cents: 207680,
              },
            ],
            created_at: '2026-03-01T00:00:00Z',
            updated_at: '2026-03-01T00:00:00Z',
          },
        ];
      }
      throw err;
    }
  },

  listInvoices: async (): Promise<Invoice[]> => {
    try {
      const res = await api.get<Invoice[]>('/portal/invoices');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return [
          {
            id: 'inv-001',
            number: 'INV-2026-0001',
            customer_id: 'cust-demo-1',
            status: 'PAID',
            issue_date: '2026-03-01T00:00:00Z',
            due_date: '2026-03-31T00:00:00Z',
            subtotal_cents: 176000,
            discount_cents: 0,
            tax_cents: 31680,
            total_cents: 207680,
            balance_cents: 0,
            paid_at: '2026-03-10T14:30:00Z',
            lines: [],
            created_at: '2026-03-01T00:00:00Z',
            updated_at: '2026-03-10T14:30:00Z',
          },
          {
            id: 'inv-002',
            number: 'INV-2026-0002',
            customer_id: 'cust-demo-1',
            status: 'CONFIRMED',
            issue_date: '2026-04-01T00:00:00Z',
            due_date: '2026-05-01T00:00:00Z',
            subtotal_cents: 176000,
            discount_cents: 0,
            tax_cents: 31680,
            total_cents: 207680,
            balance_cents: 207680,
            lines: [],
            created_at: '2026-04-01T00:00:00Z',
            updated_at: '2026-04-01T00:00:00Z',
          },
        ];
      }
      throw err;
    }
  },

  listPayments: async (): Promise<Payment[]> => {
    try {
      const res = await api.get<Payment[]>('/portal/payments');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        return [
          {
            id: 'pay-001',
            invoice_id: 'inv-001',
            invoice_number: 'INV-2026-0001',
            method: 'CARD',
            amount_cents: 207680,
            paid_at: '2026-03-10T14:30:00Z',
            note: 'Stripe Corporate Card #4242',
            created_at: '2026-03-10T14:30:00Z',
          },
        ];
      }
      throw err;
    }
  },
};
