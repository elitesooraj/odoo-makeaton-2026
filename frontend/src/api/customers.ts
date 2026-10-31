import { api } from './client';
import { Customer, CustomerCreateInput, CustomerUpdateInput } from './types';

export interface CustomerListParams extends Record<string, string | number | boolean | undefined | null> {
  search?: string;
  status?: string;
  page?: number;
  per_page?: number;
}

export const customersApi = {
  list: async (params?: CustomerListParams) => {
    return await api.get<Customer[]>('/customers', params);
  },

  get: async (id: string): Promise<Customer> => {
    const res = await api.get<Customer>(`/customers/${id}`);
    return res.data;
  },

  create: async (data: CustomerCreateInput): Promise<Customer> => {
    const res = await api.post<Customer>('/customers', data);
    return res.data;
  },

  update: async (id: string, data: CustomerUpdateInput): Promise<Customer> => {
    const res = await api.patch<Customer>(`/customers/${id}`, data);
    return res.data;
  },

  deactivate: async (id: string): Promise<void> => {
    await api.del<void>(`/customers/${id}`);
  },
};
