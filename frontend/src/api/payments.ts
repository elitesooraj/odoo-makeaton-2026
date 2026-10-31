import { api, isOfflineError } from './client';
import { invoicesApi } from './invoices';
import { Payment, PaymentMethod } from './types';

const INITIAL_PAYMENTS: Payment[] = [
  {
    id: 'pay-001',
    invoice_id: 'inv-001',
    invoice_number: 'INV-2026-0001',
    customer_name: 'Apex Retailers Ltd',
    method: 'CARD',
    amount_cents: 207680, // $2,076.80
    paid_at: '2026-03-10T14:30:00Z',
    note: 'Stripe Corporate Card #4242',
    recorded_by: 'usr-admin-1',
    created_at: '2026-03-10T14:30:00Z',
  },
];

let mockPayments = [...INITIAL_PAYMENTS];

export const paymentsApi = {
  list: async (params?: {
    invoice_id?: string;
    customer_id?: string;
    method?: string;
    page?: number;
    per_page?: number;
  }) => {
    try {
      return await api.get<Payment[]>('/payments', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockPayments];
        if (params?.invoice_id) {
          filtered = filtered.filter((p) => p.invoice_id === params.invoice_id);
        }
        if (params?.method) {
          filtered = filtered.filter((p) => p.method === params.method);
        }
        return {
          data: filtered,
          meta: {
            request_id: 'mock-req',
            page: params?.page || 1,
            per_page: params?.per_page || 25,
            total: filtered.length,
            total_pages: 1,
          },
        };
      }
      throw err;
    }
  },

  get: async (id: string): Promise<Payment> => {
    try {
      const res = await api.get<Payment>(`/payments/${id}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const found = mockPayments.find((p) => p.id === id);
        if (found) return found;
      }
      throw err;
    }
  },

  record: async (data: {
    invoice_id: string;
    method: PaymentMethod;
    amount_cents: number;
    paid_at?: string;
    note?: string;
    invoice_number?: string;
    customer_name?: string;
  }): Promise<Payment> => {
    try {
      const res = await api.post<Payment>('/payments', {
        invoice_id: data.invoice_id,
        method: data.method,
        amount_cents: data.amount_cents,
        paid_at: data.paid_at || new Date().toISOString(),
        note: data.note || null,
      });
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const payment: Payment = {
          id: `pay-${Date.now().toString(36)}`,
          invoice_id: data.invoice_id,
          invoice_number: data.invoice_number || 'INV-2026-XXXX',
          customer_name: data.customer_name || 'Customer Account',
          method: data.method,
          amount_cents: data.amount_cents,
          paid_at: data.paid_at || new Date().toISOString(),
          note: data.note || null,
          created_at: new Date().toISOString(),
        };
        mockPayments.unshift(payment);
        // Automatically deduct from invoice in mock store
        invoicesApi.applyPaymentReduction(data.invoice_id, data.amount_cents);
        return payment;
      }
      throw err;
    }
  },
};
