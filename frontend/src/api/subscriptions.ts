import { api } from './client';
import {
  BillingRunResult,
  QuotationTemplate,
  Subscription,
  SubscriptionLine,
} from './types';

export const subscriptionsApi = {
  list: async (params?: {
    customer_id?: string;
    plan_id?: string;
    status?: string;
    search?: string;
    page?: number;
    per_page?: number;
  }) => {
    return await api.get<Subscription[]>('/subscriptions', params);
  },

  get: async (id: string): Promise<Subscription> => {
    const res = await api.get<Subscription>(`/subscriptions/${id}`);
    return res.data;
  },

  create: async (data: {
    customer_id: string;
    plan_id: string;
    status?: 'DRAFT' | 'QUOTATION' | string;
    start_date: string;
    expiry_date?: string | null;
    payment_terms_days: number;
    quantity: number;
    discount_id?: string | null;
    quote_expires_at?: string | null;
    notes?: string;
    customer_name?: string;
    plan_name?: string;
    lines?: {
      product_id: string;
      variant_id?: string | null;
      description?: string;
      quantity: number;
      unit_price_cents: number;
      tax_id?: string | null;
      product_name?: string;
      variant_name?: string | null;
    }[];
  }): Promise<Subscription> => {
    const res = await api.post<Subscription>('/subscriptions', data);
    return res.data;
  },

  update: async (id: string, data: Partial<Subscription>): Promise<Subscription> => {
    const res = await api.patch<Subscription>(`/subscriptions/${id}`, data);
    return res.data;
  },

  // State transitions
  confirm: async (id: string): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/subscriptions/${id}/confirm`);
    return res.data;
  },

  activate: async (id: string): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/subscriptions/${id}/activate`);
    return res.data;
  },

  pause: async (id: string): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/subscriptions/${id}/pause`);
    return res.data;
  },

  resume: async (id: string): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/subscriptions/${id}/resume`);
    return res.data;
  },

  close: async (id: string): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/subscriptions/${id}/close`);
    return res.data;
  },

  // Line management
  addLine: async (
    subId: string,
    line: { product_id: string; variant_id?: string; description?: string; quantity: number; unit_price_cents: number; tax_id?: string }
  ): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/subscriptions/${subId}/lines`, line);
    return res.data;
  },

  updateLine: async (
    subId: string,
    lineId: string,
    line: { product_id?: string; variant_id?: string; description?: string; quantity?: number; unit_price_cents?: number; tax_id?: string }
  ): Promise<Subscription> => {
    const res = await api.patch<Subscription>(`/subscriptions/${subId}/lines/${lineId}`, line);
    return res.data;
  },

  deleteLine: async (subId: string, lineId: string): Promise<Subscription> => {
    const res = await api.del<Subscription>(`/subscriptions/${subId}/lines/${lineId}`);
    return res.data;
  },

  // Quotation Templates
  listTemplates: async (params?: { search?: string; is_active?: boolean }) => {
    const res = await api.get<QuotationTemplate[]>('/quotations/templates', params);
    return res.data;
  },

  getTemplate: async (id: string): Promise<QuotationTemplate> => {
    const res = await api.get<QuotationTemplate>(`/quotations/templates/${id}`);
    return res.data;
  },

  createTemplate: async (data: {
    name: string;
    validity_days: number;
    notes?: string;
    lines: { plan_id: string; quantity: number }[];
  }): Promise<QuotationTemplate> => {
    const res = await api.post<QuotationTemplate>('/quotations/templates', data);
    return res.data;
  },

  updateTemplate: async (id: string, data: Partial<QuotationTemplate>): Promise<QuotationTemplate> => {
    const res = await api.patch<QuotationTemplate>(`/quotations/templates/${id}`, data);
    return res.data;
  },

  deactivateTemplate: async (id: string): Promise<void> => {
    await api.del<void>(`/quotations/templates/${id}`);
  },

  instantiateTemplate: async (
    templateId: string,
    data: {
      customer_id: string;
      start_date?: string;
      quantity?: number;
      payment_terms_days?: number;
      discount_id?: string | null;
      notes?: string;
      customer_name?: string;
    }
  ): Promise<Subscription> => {
    const res = await api.post<Subscription>(`/quotations/templates/${templateId}/instantiate`, data);
    return res.data;
  },

  // Billing Run
  runBilling: async (): Promise<BillingRunResult> => {
    const res = await api.post<BillingRunResult>('/billing/run');
    return res.data;
  },
};
