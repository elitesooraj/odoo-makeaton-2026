import { api } from './client';
import { Payment, PaymentMethod } from './types';

export const paymentsApi = {
  list: async (params?: {
    invoice_id?: string;
    customer_id?: string;
    method?: string;
    page?: number;
    per_page?: number;
  }) => {
    return await api.get<Payment[]>('/payments', params);
  },

  get: async (id: string): Promise<Payment> => {
    const res = await api.get<Payment>(`/payments/${id}`);
    return res.data;
  },

  record: async (data: {
    invoice_id: string;
    method: PaymentMethod;
    amount_cents: number;
    paid_at?: string;
    note?: string;
  }): Promise<Payment> => {
    const res = await api.post<Payment>('/payments', {
      invoice_id: data.invoice_id,
      method: data.method,
      amount_cents: data.amount_cents,
      paid_at: data.paid_at || new Date().toISOString(),
      note: data.note || null,
    });
    return res.data;
  },
};
