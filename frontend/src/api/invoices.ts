import { api } from './client';
import { Invoice } from './types';

export const invoicesApi = {
  list: async (params?: {
    customer_id?: string;
    subscription_id?: string;
    status?: string;
    search?: string;
    page?: number;
    per_page?: number;
  }) => {
    return await api.get<Invoice[]>('/invoices', params);
  },

  get: async (id: string): Promise<Invoice> => {
    const res = await api.get<Invoice>(`/invoices/${id}`);
    return res.data;
  },

  create: async (data: {
    customer_id: string;
    subscription_id?: string | null;
    issue_date?: string;
    due_date?: string;
    period_start?: string | null;
    period_end?: string | null;
    discount_id?: string | null;
    notes?: string;
    customer_name?: string;
    lines?: {
      product_id?: string;
      variant_id?: string;
      description: string;
      quantity: number;
      unit_price_cents: number;
      tax_id?: string;
    }[];
  }): Promise<Invoice> => {
    const res = await api.post<Invoice>('/invoices', data);
    return res.data;
  },

  update: async (id: string, data: Partial<Invoice>): Promise<Invoice> => {
    const res = await api.patch<Invoice>(`/invoices/${id}`, data);
    return res.data;
  },

  confirm: async (id: string): Promise<Invoice> => {
    const res = await api.post<Invoice>(`/invoices/${id}/confirm`);
    return res.data;
  },

  cancel: async (id: string): Promise<Invoice> => {
    const res = await api.post<Invoice>(`/invoices/${id}/cancel`);
    return res.data;
  },

  send: async (id: string): Promise<Invoice> => {
    const res = await api.post<Invoice>(`/invoices/${id}/send`);
    return res.data;
  },
};
