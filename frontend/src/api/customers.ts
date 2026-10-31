import { api, ApiException, isOfflineError } from './client';
import { Customer, CustomerCreateInput, CustomerUpdateInput } from './types';

export interface CustomerListParams extends Record<string, string | number | boolean | undefined | null> {
  search?: string;
  status?: string;
  page?: number;
  per_page?: number;
}

const INITIAL_CUSTOMERS: Customer[] = [
  {
    id: 'cust-demo-1',
    name: 'Apex Retailers Ltd',
    email: 'billing@apexretail.test',
    phone: '+1 (555) 234-5678',
    tax_id: 'US-84920194',
    status: 'ACTIVE',
    billing_address: {
      street: '742 Evergreen Terrace',
      city: 'Springfield',
      state: 'IL',
      postal_code: '62704',
      country: 'USA',
    },
    notes: 'Key enterprise account on annual renewal',
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-01-15T10:00:00Z',
  },
  {
    id: 'cust-demo-2',
    name: 'Acme Logistics Corp',
    email: 'finance@acmelogistics.test',
    phone: '+1 (555) 876-5432',
    tax_id: 'US-91823746',
    status: 'ACTIVE',
    billing_address: {
      street: '100 Industrial Parkway',
      city: 'Chicago',
      state: 'IL',
      postal_code: '60607',
      country: 'USA',
    },
    notes: 'Multi-seat subscription with Odoo brand variant',
    created_at: '2026-02-01T14:30:00Z',
    updated_at: '2026-02-01T14:30:00Z',
  },
  {
    id: 'cust-demo-3',
    name: 'Northwind Traders',
    email: 'orders@northwind.test',
    phone: '+1 (555) 345-6789',
    tax_id: 'US-10293847',
    status: 'INACTIVE',
    billing_address: {
      street: '123 Harbor View Road',
      city: 'Seattle',
      state: 'WA',
      postal_code: '98101',
      country: 'USA',
    },
    notes: 'Deactivated seasonal contract',
    created_at: '2026-02-10T09:15:00Z',
    updated_at: '2026-02-28T16:00:00Z',
  },
];

let mockCustomers = [...INITIAL_CUSTOMERS];

export const customersApi = {
  list: async (params?: CustomerListParams) => {
    try {
      return await api.get<Customer[]>('/customers', params);
    } catch (err: any) {
      if (isOfflineError(err) || (err instanceof ApiException && (err.status === 404 || err.status === 501))) {
        let filtered = [...mockCustomers];
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter(
            (c) =>
              c.name.toLowerCase().includes(q) ||
              (c.email && c.email.toLowerCase().includes(q)) ||
              (c.tax_id && c.tax_id.toLowerCase().includes(q))
          );
        }
        if (params?.status) {
          filtered = filtered.filter((c) => c.status === params.status);
        }
        return {
          data: filtered,
          meta: {
            request_id: 'mock-req',
            page: params?.page || 1,
            per_page: params?.per_page || 10,
            total: filtered.length,
            total_pages: Math.ceil(filtered.length / (params?.per_page || 10)) || 1,
          },
        };
      }
      throw err;
    }
  },

  get: async (id: string): Promise<Customer> => {
    try {
      const res = await api.get<Customer>(`/customers/${id}`);
      return res.data;
    } catch (err: any) {
      if (isOfflineError(err) || (err instanceof ApiException && (err.status === 404 || err.status === 501))) {
        const found = mockCustomers.find((c) => c.id === id);
        if (found) return found;
      }
      throw err;
    }
  },

  create: async (data: CustomerCreateInput): Promise<Customer> => {
    try {
      const res = await api.post<Customer>('/customers', data);
      return res.data;
    } catch (err: any) {
      if (isOfflineError(err) || (err instanceof ApiException && (err.status === 404 || err.status === 501))) {
        const newCustomer: Customer = {
          id: `cust-${Date.now().toString(36)}`,
          name: data.name,
          email: data.email || null,
          phone: data.phone || null,
          billing_address: data.billing_address || {},
          tax_id: data.tax_id || null,
          status: 'ACTIVE',
          notes: data.notes || null,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockCustomers.unshift(newCustomer);
        return newCustomer;
      }
      throw err;
    }
  },

  update: async (id: string, data: CustomerUpdateInput): Promise<Customer> => {
    try {
      const res = await api.patch<Customer>(`/customers/${id}`, data);
      return res.data;
    } catch (err: any) {
      if (isOfflineError(err) || (err instanceof ApiException && (err.status === 404 || err.status === 501))) {
        const idx = mockCustomers.findIndex((c) => c.id === id);
        if (idx !== -1) {
          mockCustomers[idx] = {
            ...mockCustomers[idx],
            ...data,
            billing_address: data.billing_address || mockCustomers[idx].billing_address,
            updated_at: new Date().toISOString(),
          };
          return mockCustomers[idx];
        }
      }
      throw err;
    }
  },

  deactivate: async (id: string): Promise<void> => {
    try {
      await api.del<void>(`/customers/${id}`);
    } catch (err: any) {
      if (isOfflineError(err) || (err instanceof ApiException && (err.status === 404 || err.status === 501))) {
        const cust = mockCustomers.find((c) => c.id === id);
        if (cust) {
          cust.status = 'INACTIVE';
          cust.updated_at = new Date().toISOString();
        }
        return;
      }
      throw err;
    }
  },
};
