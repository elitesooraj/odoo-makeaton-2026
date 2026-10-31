import { api } from './client';
import { Discount, Plan, Product, ProductVariant, Tax } from './types';

export const catalogApi = {
  // Products
  listProducts: async (params?: { search?: string; is_active?: boolean; page?: number; per_page?: number }) => {
    return await api.get<Product[]>('/products', params);
  },

  getProduct: async (id: string): Promise<Product> => {
    const res = await api.get<Product>(`/products/${id}`);
    return res.data;
  },

  createProduct: async (data: {
    name: string;
    type: 'SERVICE' | 'GOODS' | string;
    sales_price_cents: number;
    cost_price_cents: number;
    notes?: string;
    variants?: { name: string; price_delta_cents: number }[];
  }): Promise<Product> => {
    const res = await api.post<Product>('/products', data);
    return res.data;
  },

  updateProduct: async (id: string, data: Partial<Product>): Promise<Product> => {
    const res = await api.patch<Product>(`/products/${id}`, data);
    return res.data;
  },

  deactivateProduct: async (id: string): Promise<Product> => {
    const res = await api.del<Product>(`/products/${id}`);
    return res.data;
  },

  // Product Variants
  addVariant: async (productId: string, data: { name: string; price_delta_cents: number }): Promise<ProductVariant> => {
    const res = await api.post<ProductVariant>(`/products/${productId}/variants`, data);
    return res.data;
  },

  updateVariant: async (
    productId: string,
    variantId: string,
    data: { name?: string; price_delta_cents?: number; is_active?: boolean }
  ): Promise<ProductVariant> => {
    const res = await api.patch<ProductVariant>(`/products/${productId}/variants/${variantId}`, data);
    return res.data;
  },

  deactivateVariant: async (productId: string, variantId: string): Promise<ProductVariant> => {
    const res = await api.del<ProductVariant>(`/products/${productId}/variants/${variantId}`);
    return res.data;
  },

  // Plans
  listPlans: async (params?: { product_id?: string; search?: string; is_active?: boolean; page?: number; per_page?: number }) => {
    return await api.get<Plan[]>('/plans', params);
  },

  getPlan: async (id: string): Promise<Plan> => {
    const res = await api.get<Plan>(`/plans/${id}`);
    return res.data;
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
    const res = await api.post<Plan>('/plans', data);
    return res.data;
  },

  updatePlan: async (id: string, data: Partial<Plan>): Promise<Plan> => {
    const res = await api.patch<Plan>(`/plans/${id}`, data);
    return res.data;
  },

  deactivatePlan: async (id: string): Promise<Plan> => {
    const res = await api.del<Plan>(`/plans/${id}`);
    return res.data;
  },

  // Taxes
  listTaxes: async (params?: { is_active?: boolean; search?: string; page?: number; per_page?: number }) => {
    return await api.get<Tax[]>('/taxes', params);
  },

  getTax: async (id: string): Promise<Tax> => {
    const res = await api.get<Tax>(`/taxes/${id}`);
    return res.data;
  },

  createTax: async (data: {
    name: string;
    rate_bp: number;
    is_inclusive?: boolean;
    country_code?: string;
  }): Promise<Tax> => {
    const res = await api.post<Tax>('/taxes', data);
    return res.data;
  },

  updateTax: async (id: string, data: Partial<Tax>): Promise<Tax> => {
    const res = await api.patch<Tax>(`/taxes/${id}`, data);
    return res.data;
  },

  deactivateTax: async (id: string): Promise<Tax> => {
    const res = await api.del<Tax>(`/taxes/${id}`);
    return res.data;
  },

  // Discounts
  listDiscounts: async (params?: { is_active?: boolean; search?: string; page?: number; per_page?: number }) => {
    return await api.get<Discount[]>('/discounts', params);
  },

  getDiscount: async (id: string): Promise<Discount> => {
    const res = await api.get<Discount>(`/discounts/${id}`);
    return res.data;
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
    const res = await api.post<Discount>('/discounts', data);
    return res.data;
  },

  updateDiscount: async (id: string, data: Partial<Discount>): Promise<Discount> => {
    const res = await api.patch<Discount>(`/discounts/${id}`, data);
    return res.data;
  },

  deactivateDiscount: async (id: string): Promise<Discount> => {
    const res = await api.del<Discount>(`/discounts/${id}`);
    return res.data;
  },
};
