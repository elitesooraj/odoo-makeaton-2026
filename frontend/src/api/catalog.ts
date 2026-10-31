import { api, isOfflineError } from './client';
import { Discount, Plan, Product, ProductVariant, Tax } from './types';

// Initial seed products matching the Brightloop business scene
const INITIAL_PRODUCTS: Product[] = [
  {
    id: 'prod-001',
    name: 'BrightERP Core Platform',
    type: 'SERVICE',
    sales_price_cents: 120000, // $1,200.00
    cost_price_cents: 35000,   // $350.00
    notes: 'Primary business software suite for mid-market operations',
    is_active: true,
    variants: [
      { id: 'var-001-std', product_id: 'prod-001', name: 'Standard Edition', price_delta_cents: 0, is_active: true },
      { id: 'var-001-odoo', product_id: 'prod-001', name: 'Odoo Connected Brand', price_delta_cents: 56000, is_active: true }, // +$560.00
      { id: 'var-001-ent', product_id: 'prod-001', name: 'Enterprise Multi-site', price_delta_cents: 95000, is_active: true },  // +$950.00
    ],
    created_at: new Date(Date.now() - 30 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 30 * 86400000).toISOString(),
  },
  {
    id: 'prod-002',
    name: 'Accounting & Payroll Add-on',
    type: 'SERVICE',
    sales_price_cents: 45000, // $450.00
    cost_price_cents: 12000,  // $120.00
    notes: 'Automated tax tables and localized pay stubs',
    is_active: true,
    variants: [
      { id: 'var-002-std', product_id: 'prod-002', name: 'Standard', price_delta_cents: 0, is_active: true },
      { id: 'var-002-multi', product_id: 'prod-002', name: 'Multi-Currency Pack', price_delta_cents: 18000, is_active: true },
    ],
    created_at: new Date(Date.now() - 20 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 20 * 86400000).toISOString(),
  },
  {
    id: 'prod-003',
    name: 'Dedicated Hardware Security Key',
    type: 'GOODS',
    sales_price_cents: 7500, // $75.00
    cost_price_cents: 2500,  // $25.00
    notes: 'FIDO2 hardware token for secure back-office access',
    is_active: true,
    variants: [],
    created_at: new Date(Date.now() - 15 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 15 * 86400000).toISOString(),
  },
];

const INITIAL_TAXES: Tax[] = [
  {
    id: 'tax-vat18',
    name: 'Standard VAT 18%',
    rate_bp: 1800,
    is_inclusive: false,
    country_code: 'US',
    is_active: true,
    created_at: new Date(Date.now() - 40 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 40 * 86400000).toISOString(),
  },
  {
    id: 'tax-gst05',
    name: 'GST 5% Inclusive',
    rate_bp: 500,
    is_inclusive: true,
    country_code: 'IN',
    is_active: true,
    created_at: new Date(Date.now() - 35 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 35 * 86400000).toISOString(),
  },
];

const INITIAL_DISCOUNTS: Discount[] = [
  {
    id: 'disc-early10',
    name: 'Early Adopter 10% Off',
    kind: 'PERCENT',
    value_cents: 0,
    value_bp: 1000, // 10%
    scope: 'ORDER',
    product_id: null,
    min_purchase_cents: 50000,
    min_quantity: 1,
    starts_at: '2026-01-01T00:00:00Z',
    ends_at: '2026-12-31T23:59:59Z',
    max_uses: 100,
    used_count: 14,
    is_active: true,
    created_at: new Date(Date.now() - 30 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 30 * 86400000).toISOString(),
  },
  {
    id: 'disc-flat100',
    name: 'Enterprise $100 Credit',
    kind: 'FIXED',
    value_cents: 10000, // $100.00
    value_bp: 0,
    scope: 'ORDER',
    product_id: null,
    min_purchase_cents: 100000,
    min_quantity: 1,
    starts_at: '2026-01-01T00:00:00Z',
    ends_at: null,
    max_uses: null,
    used_count: 5,
    is_active: true,
    created_at: new Date(Date.now() - 25 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - 25 * 86400000).toISOString(),
  },
];

const INITIAL_PLANS: Plan[] = [
  {
    id: 'plan-001',
    product_id: 'prod-001',
    product_name: 'BrightERP Core Platform',
    tax_id: 'tax-vat18',
    tax_name: 'Standard VAT 18%',
    name: 'Monthly Growth Plan',
    interval: 'MONTHLY',
    billing_interval: 'MONTHLY',
    price_cents: 120000,
    minimum_quantity: 1,
    min_quantity: 1,
    available_from: '2026-01-01',
    available_to: null,
    available_until: null,
    auto_close: false,
    auto_renew: true,
    allow_renewal: true,
    allow_early_close: true,
    allow_pause: true,
    is_active: true,
    status: 'ACTIVE',
    created_at: new Date(Date.now() - 25 * 86400000).toISOString(),
  },
  {
    id: 'plan-002',
    product_id: 'prod-001',
    product_name: 'BrightERP Core Platform',
    tax_id: 'tax-vat18',
    tax_name: 'Standard VAT 18%',
    name: 'Annual Enterprise Plan',
    interval: 'YEARLY',
    billing_interval: 'YEARLY',
    price_cents: 1200000, // $12,000 / yr
    minimum_quantity: 5,
    min_quantity: 5,
    available_from: '2026-01-01',
    available_to: null,
    available_until: null,
    auto_close: false,
    auto_renew: true,
    allow_renewal: true,
    allow_early_close: true,
    allow_pause: false,
    is_active: true,
    status: 'ACTIVE',
    created_at: new Date(Date.now() - 20 * 86400000).toISOString(),
  },
  {
    id: 'plan-003',
    product_id: 'prod-002',
    product_name: 'Accounting & Payroll Add-on',
    tax_id: 'tax-vat18',
    tax_name: 'Standard VAT 18%',
    name: 'Addon Monthly Recurring',
    interval: 'MONTHLY',
    billing_interval: 'MONTHLY',
    price_cents: 45000,
    minimum_quantity: 1,
    min_quantity: 1,
    available_from: '2026-02-01',
    available_to: null,
    available_until: null,
    auto_close: true,
    auto_renew: true,
    allow_renewal: true,
    allow_early_close: true,
    allow_pause: true,
    is_active: true,
    status: 'ACTIVE',
    created_at: new Date(Date.now() - 10 * 86400000).toISOString(),
  },
];

let mockProducts = [...INITIAL_PRODUCTS];
let mockPlans = [...INITIAL_PLANS];
let mockTaxes = [...INITIAL_TAXES];
let mockDiscounts = [...INITIAL_DISCOUNTS];

export const catalogApi = {
  // Products
  listProducts: async (params?: { search?: string; is_active?: boolean; page?: number; per_page?: number }) => {
    try {
      return await api.get<Product[]>('/products', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockProducts];
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter((p) => p.name.toLowerCase().includes(q) || p.type.toLowerCase().includes(q));
        }
        if (params?.is_active !== undefined) {
          filtered = filtered.filter((p) => p.is_active === params.is_active);
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
      if (isOfflineError(err)) {
        const found = mockProducts.find((p) => p.id === id);
        if (found) return found;
      }
      throw err;
    }
  },

  createProduct: async (data: {
    name: string;
    type: 'SERVICE' | 'GOODS' | string;
    sales_price_cents: number;
    cost_price_cents: number;
    notes?: string;
    variants?: { name: string; price_delta_cents: number }[];
  }): Promise<Product> => {
    try {
      const res = await api.post<Product>('/products', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const newId = `prod-${Date.now().toString(36)}`;
        const product: Product = {
          id: newId,
          name: data.name,
          type: data.type || 'SERVICE',
          sales_price_cents: data.sales_price_cents,
          cost_price_cents: data.cost_price_cents,
          notes: data.notes || null,
          is_active: true,
          variants: (data.variants || []).map((v, i) => ({
            id: `var-${newId}-${i}`,
            product_id: newId,
            name: v.name,
            price_delta_cents: v.price_delta_cents,
            is_active: true,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
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
      if (isOfflineError(err)) {
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

  deactivateProduct: async (id: string): Promise<Product> => {
    try {
      const res = await api.del<Product>(`/products/${id}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const p = mockProducts.find((item) => item.id === id);
        if (p) {
          p.is_active = false;
          return p;
        }
      }
      throw err;
    }
  },

  // Product Variants
  addVariant: async (productId: string, data: { name: string; price_delta_cents: number }): Promise<ProductVariant> => {
    try {
      const res = await api.post<ProductVariant>(`/products/${productId}/variants`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const p = mockProducts.find((item) => item.id === productId);
        const variant: ProductVariant = {
          id: `var-${Date.now().toString(36)}`,
          product_id: productId,
          name: data.name,
          price_delta_cents: data.price_delta_cents,
          is_active: true,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        if (p) {
          p.variants = [...(p.variants || []), variant];
        }
        return variant;
      }
      throw err;
    }
  },

  updateVariant: async (
    productId: string,
    variantId: string,
    data: { name?: string; price_delta_cents?: number; is_active?: boolean }
  ): Promise<ProductVariant> => {
    try {
      const res = await api.patch<ProductVariant>(`/products/${productId}/variants/${variantId}`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const p = mockProducts.find((item) => item.id === productId);
        if (p) {
          const v = p.variants.find((item) => item.id === variantId);
          if (v) {
            Object.assign(v, data, { updated_at: new Date().toISOString() });
            return v;
          }
        }
      }
      throw err;
    }
  },

  deactivateVariant: async (productId: string, variantId: string): Promise<ProductVariant> => {
    try {
      const res = await api.del<ProductVariant>(`/products/${productId}/variants/${variantId}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const p = mockProducts.find((item) => item.id === productId);
        if (p) {
          const v = p.variants.find((item) => item.id === variantId);
          if (v) {
            v.is_active = false;
            return v;
          }
        }
      }
      throw err;
    }
  },

  // Plans
  listPlans: async (params?: { product_id?: string; search?: string; is_active?: boolean; page?: number; per_page?: number }) => {
    try {
      return await api.get<Plan[]>('/plans', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockPlans];
        if (params?.product_id) {
          filtered = filtered.filter((p) => p.product_id === params.product_id);
        }
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter((p) => p.name.toLowerCase().includes(q));
        }
        if (params?.is_active !== undefined) {
          filtered = filtered.filter((p) => p.is_active === params.is_active);
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

  createPlan: async (data: {
    product_id: string;
    tax_id?: string | null;
    name: string;
    interval: string;
    price_cents: number;
    minimum_quantity?: number;
    available_from?: string | null;
    available_to?: string | null;
    auto_close?: boolean;
    auto_renew?: boolean;
    allow_pause?: boolean;
    allow_early_close?: boolean;
  }): Promise<Plan> => {
    try {
      const res = await api.post<Plan>('/plans', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const parentProd = mockProducts.find((p) => p.id === data.product_id);
        const parentTax = mockTaxes.find((t) => t.id === data.tax_id);
        const plan: Plan = {
          ...data,
          id: `plan-${Date.now().toString(36)}`,
          interval: data.interval as Plan['interval'],
          billing_interval: data.interval as Plan['interval'],
          minimum_quantity: data.minimum_quantity || 1,
          min_quantity: data.minimum_quantity || 1,
          product_name: parentProd?.name || 'Product',
          tax_name: parentTax?.name,
          available_from: data.available_from || null,
          available_to: data.available_to || null,
          available_until: data.available_to || null,
          auto_close: !!data.auto_close,
          auto_renew: data.auto_renew !== false,
          allow_renewal: data.auto_renew !== false,
          allow_pause: data.allow_pause !== false,
          allow_early_close: data.allow_early_close !== false,
          is_active: true,
          status: 'ACTIVE',
          created_at: new Date().toISOString(),
        };
        mockPlans.unshift(plan);
        return plan;
      }
      throw err;
    }
  },

  updatePlan: async (id: string, data: Partial<Plan>): Promise<Plan> => {
    try {
      const res = await api.patch<Plan>(`/plans/${id}`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const idx = mockPlans.findIndex((p) => p.id === id);
        if (idx !== -1) {
          mockPlans[idx] = {
            ...mockPlans[idx],
            ...data,
            updated_at: new Date().toISOString(),
          };
          return mockPlans[idx];
        }
      }
      throw err;
    }
  },

  deactivatePlan: async (id: string): Promise<Plan> => {
    try {
      const res = await api.del<Plan>(`/plans/${id}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const plan = mockPlans.find((p) => p.id === id);
        if (plan) {
          plan.is_active = false;
          return plan;
        }
      }
      throw err;
    }
  },

  // Taxes
  listTaxes: async (params?: { is_active?: boolean; search?: string; page?: number; per_page?: number }) => {
    try {
      return await api.get<Tax[]>('/taxes', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockTaxes];
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter((t) => t.name.toLowerCase().includes(q));
        }
        if (params?.is_active !== undefined) {
          filtered = filtered.filter((t) => t.is_active === params.is_active);
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

  createTax: async (data: {
    name: string;
    rate_bp: number;
    is_inclusive?: boolean;
    country_code?: string;
  }): Promise<Tax> => {
    try {
      const res = await api.post<Tax>('/taxes', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const tax: Tax = {
          id: `tax-${Date.now().toString(36)}`,
          name: data.name,
          rate_bp: data.rate_bp,
          is_inclusive: !!data.is_inclusive,
          country_code: data.country_code || null,
          is_active: true,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockTaxes.unshift(tax);
        return tax;
      }
      throw err;
    }
  },

  updateTax: async (id: string, data: Partial<Tax>): Promise<Tax> => {
    try {
      const res = await api.patch<Tax>(`/taxes/${id}`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const tax = mockTaxes.find((t) => t.id === id);
        if (tax) {
          Object.assign(tax, data, { updated_at: new Date().toISOString() });
          return tax;
        }
      }
      throw err;
    }
  },

  deactivateTax: async (id: string): Promise<Tax> => {
    try {
      const res = await api.del<Tax>(`/taxes/${id}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const tax = mockTaxes.find((t) => t.id === id);
        if (tax) {
          tax.is_active = false;
          return tax;
        }
      }
      throw err;
    }
  },

  // Discounts
  listDiscounts: async (params?: { is_active?: boolean; search?: string; page?: number; per_page?: number }) => {
    try {
      return await api.get<Discount[]>('/discounts', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockDiscounts];
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter((d) => d.name.toLowerCase().includes(q));
        }
        if (params?.is_active !== undefined) {
          filtered = filtered.filter((d) => d.is_active === params.is_active);
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

  createDiscount: async (data: {
    name: string;
    kind: 'PERCENT' | 'FIXED';
    value_cents?: number;
    value_bp?: number;
    scope?: 'ORDER' | 'PRODUCT';
    product_id?: string | null;
    min_purchase_cents?: number;
    min_quantity?: number;
    starts_at?: string | null;
    ends_at?: string | null;
    max_uses?: number | null;
  }): Promise<Discount> => {
    try {
      const res = await api.post<Discount>('/discounts', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const disc: Discount = {
          id: `disc-${Date.now().toString(36)}`,
          name: data.name,
          kind: data.kind,
          value_cents: data.value_cents || 0,
          value_bp: data.value_bp || 0,
          scope: data.scope || 'ORDER',
          product_id: data.product_id || null,
          min_purchase_cents: data.min_purchase_cents || 0,
          min_quantity: data.min_quantity || 1,
          starts_at: data.starts_at || null,
          ends_at: data.ends_at || null,
          max_uses: data.max_uses ?? null,
          used_count: 0,
          is_active: true,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockDiscounts.unshift(disc);
        return disc;
      }
      throw err;
    }
  },

  updateDiscount: async (id: string, data: Partial<Discount>): Promise<Discount> => {
    try {
      const res = await api.patch<Discount>(`/discounts/${id}`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const disc = mockDiscounts.find((d) => d.id === id);
        if (disc) {
          Object.assign(disc, data, { updated_at: new Date().toISOString() });
          return disc;
        }
      }
      throw err;
    }
  },

  deactivateDiscount: async (id: string): Promise<Discount> => {
    try {
      const res = await api.del<Discount>(`/discounts/${id}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const disc = mockDiscounts.find((d) => d.id === id);
        if (disc) {
          disc.is_active = false;
          return disc;
        }
      }
      throw err;
    }
  },
};
