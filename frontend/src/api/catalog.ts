import { api, ApiException } from './client';
import { Plan, Product } from './types';

// Initial seed products matching the Brightloop business scene
const INITIAL_PRODUCTS: Product[] = [
  {
    id: 'prod-001',
    name: 'BrightERP Core',
    type: 'SOFTWARE',
    sales_price_cents: 120000, // $1,200.00
    cost_price_cents: 35000,   // $350.00
    notes: 'Primary business suite for mid-market operations',
    is_active: true,
    variants: [
      { id: 'var-001-std', product_id: 'prod-001', name: 'Standard Edition', price_delta_cents: 0 },
      { id: 'var-001-odoo', product_id: 'prod-001', name: 'Odoo Connected Brand', price_delta_cents: 56000 }, // +$560.00
      { id: 'var-001-ent', product_id: 'prod-001', name: 'Enterprise Multi-site', price_delta_cents: 95000 },  // +$950.00
    ],
    created_at: new Date(Date.now() - 30 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 30 * 86400000).toISOString(),
  },
  {
    id: 'prod-002',
    name: 'Accounting & Payroll Add-on',
    type: 'ADDON',
    sales_price_cents: 45000, // $450.00
    cost_price_cents: 12000,  // $120.00
    notes: 'Automated tax tables and localized pay stubs',
    is_active: true,
    variants: [
      { id: 'var-002-std', product_id: 'prod-002', name: 'Standard', price_delta_cents: 0 },
      { id: 'var-002-multi', product_id: 'prod-002', name: 'Multi-Currency Pack', price_delta_cents: 18000 },
    ],
    created_at: new Date(Date.now() - 20 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 20 * 86400000).toISOString(),
  },
  {
    id: 'prod-003',
    name: '24/7 Dedicated Support SLA',
    type: 'SERVICE',
    sales_price_cents: 60000, // $600.00
    cost_price_cents: 20000,  // $200.00
    notes: 'Direct slack channel and 15-minute response SLA',
    is_active: true,
    variants: [],
    created_at: new Date(Date.now() - 15 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 15 * 86400000).toISOString(),
  },
];

const INITIAL_PLANS: Plan[] = [
  {
    id: 'plan-001',
    product_id: 'prod-001',
    product_name: 'BrightERP Core',
    name: 'Monthly Growth Plan',
    billing_interval: 'MONTHLY',
    price_cents: 120000,
    min_quantity: 1,
    available_from: '2026-01-01',
    available_until: null,
    auto_close: false,
    allow_early_close: true,
    allow_pause: true,
    allow_renewal: true,
    status: 'ACTIVE',
    created_at: new Date(Date.now() - 25 * 86400000).toISOString(),
  },
  {
    id: 'plan-002',
    product_id: 'prod-001',
    product_name: 'BrightERP Core',
    name: 'Annual Enterprise Plan',
    billing_interval: 'YEARLY',
    price_cents: 1200000, // $12,000 / yr (2 months discount)
    min_quantity: 5,
    available_from: '2026-01-01',
    available_until: null,
    auto_close: false,
    allow_early_close: true,
    allow_pause: false,
    allow_renewal: true,
    status: 'ACTIVE',
    created_at: new Date(Date.now() - 20 * 86400000).toISOString(),
  },
  {
    id: 'plan-003',
    product_id: 'prod-002',
    product_name: 'Accounting & Payroll Add-on',
    name: 'Addon Monthly Recurring',
    billing_interval: 'MONTHLY',
    price_cents: 45000,
    min_quantity: 1,
    available_from: '2026-02-01',
    available_until: null,
    auto_close: true,
    allow_early_close: true,
    allow_pause: true,
    allow_renewal: true,
    status: 'ACTIVE',
    created_at: new Date(Date.now() - 10 * 86400000).toISOString(),
  },
];

// In-memory state storage for demo / fallback
let mockProducts = [...INITIAL_PRODUCTS];
let mockPlans = [...INITIAL_PLANS];

export const catalogApi = {
  listProducts: async (params?: { search?: string; status?: string; page?: number; per_page?: number }) => {
    try {
      return await api.get<Product[]>('/products', params);
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        let filtered = [...mockProducts];
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter((p) => p.name.toLowerCase().includes(q) || p.type.toLowerCase().includes(q));
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

  getProduct: async (id: string): Promise<Product> => {
    try {
      const res = await api.get<Product>(`/products/${id}`);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const found = mockProducts.find((p) => p.id === id);
        if (found) return found;
      }
      throw err;
    }
  },

  createProduct: async (data: {
    name: string;
    type: string;
    sales_price_cents: number;
    cost_price_cents: number;
    notes?: string;
    variants: { name: string; price_delta_cents: number }[];
  }): Promise<Product> => {
    try {
      const res = await api.post<Product>('/products', data);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const newId = `prod-${Date.now().toString(36)}`;
        const product: Product = {
          id: newId,
          name: data.name,
          type: data.type,
          sales_price_cents: data.sales_price_cents,
          cost_price_cents: data.cost_price_cents,
          notes: data.notes || null,
          is_active: true,
          variants: (data.variants || []).map((v, i) => ({
            id: `var-${newId}-${i}`,
            product_id: newId,
            name: v.name,
            price_delta_cents: v.price_delta_cents,
          })),
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockProducts.unshift(product);
        return product;
      }
      throw err;
    }
  },

  updateProduct: async (id: string, data: Partial<Product>): Promise<Product> => {
    try {
      const res = await api.patch<Product>(`/products/${id}`, data);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const idx = mockProducts.findIndex((p) => p.id === id);
        if (idx !== -1) {
          mockProducts[idx] = {
            ...mockProducts[idx],
            ...data,
            updated_at: new Date().toISOString(),
          };
          return mockProducts[idx];
        }
      }
      throw err;
    }
  },

  listPlans: async (params?: { product_id?: string; interval?: string; page?: number; per_page?: number }) => {
    try {
      return await api.get<Plan[]>('/plans', params);
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        let filtered = [...mockPlans];
        if (params?.product_id) {
          filtered = filtered.filter((p) => p.product_id === params.product_id);
        }
        if (params?.interval) {
          filtered = filtered.filter((p) => p.billing_interval === params.interval);
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

  createPlan: async (data: Omit<Plan, 'id' | 'created_at'>): Promise<Plan> => {
    try {
      const res = await api.post<Plan>('/plans', data);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const parentProd = mockProducts.find((p) => p.id === data.product_id);
        const plan: Plan = {
          ...data,
          id: `plan-${Date.now().toString(36)}`,
          product_name: parentProd?.name || 'Product',
          created_at: new Date().toISOString(),
        };
        mockPlans.unshift(plan);
        return plan;
      }
      throw err;
    }
  },
};

