import { api } from './client';
import { Invoice, Payment, PortalMe, Subscription } from './types';

export const portalApi = {
  getMe: async (): Promise<PortalMe> => {
    const res = await api.get<PortalMe>('/portal/me');
    return res.data;
  },

  listSubscriptions: async (): Promise<Subscription[]> => {
    const res = await api.get<Subscription[]>('/portal/subscriptions');
    return res.data;
  },

  getSubscription: async (id: string): Promise<Subscription> => {
    const res = await api.get<Subscription>(`/portal/subscriptions/${id}`);
    return res.data;
  },

  listInvoices: async (): Promise<Invoice[]> => {
    const res = await api.get<Invoice[]>('/portal/invoices');
    return res.data;
  },

  getInvoice: async (id: string): Promise<Invoice> => {
    const res = await api.get<Invoice>(`/portal/invoices/${id}`);
    return res.data;
  },

  listPayments: async (): Promise<Payment[]> => {
    const res = await api.get<Payment[]>('/portal/payments');
    return res.data;
  },
};
