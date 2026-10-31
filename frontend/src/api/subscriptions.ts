import { api, ApiException } from './client';
import { QuotationTemplate, Subscription, SubscriptionStatus } from './types';

const INITIAL_TEMPLATES: QuotationTemplate[] = [
  {
    id: 'tmpl-001',
    name: 'Standard Mid-Market Onboarding (Monthly)',
    plan_id: 'plan-001',
    plan_name: 'Monthly Growth Plan',
    validity_days: 30,
    lines: [
      { product_id: 'prod-001', variant_id: 'var-001-std', quantity: 1 },
      { product_id: 'prod-002', variant_id: 'var-002-std', quantity: 1 },
    ],
  },
  {
    id: 'tmpl-002',
    name: 'Enterprise Full Stack Suite (Yearly)',
    plan_id: 'plan-002',
    plan_name: 'Annual Enterprise Plan',
    validity_days: 14,
    lines: [
      { product_id: 'prod-001', variant_id: 'var-001-odoo', quantity: 5 },
      { product_id: 'prod-002', variant_id: 'var-002-multi', quantity: 5 },
      { product_id: 'prod-003', quantity: 1 },
    ],
  },
];

const INITIAL_SUBSCRIPTIONS: Subscription[] = [
  {
    id: 'sub-001',
    subscription_number: 'SUB-2026-0001',
    customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd',
    plan_id: 'plan-001',
    plan_name: 'Monthly Growth Plan',
    status: 'ACTIVE',
    start_date: '2026-03-01T00:00:00Z',
    expiry_date: null,
    payment_terms: 'NET_30',
    subtotal_cents: 176000, // $1,760.00
    tax_cents: 13200,      // $132.00 (7.5%)
    total_cents: 189200,   // $1,892.00
    lines: [
      {
        id: 'subline-1',
        product_id: 'prod-001',
        product_name: 'BrightERP Core',
        variant_id: 'var-001-odoo',
        variant_name: 'Odoo Connected Brand (+560.00)',
        quantity: 1,
        unit_price_cents: 176000,
        tax_rate_bp: 750,
        tax_cents: 13200,
        discount_cents: 0,
        total_cents: 189200,
      },
    ],
    created_at: '2026-03-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
  },
  {
    id: 'sub-002',
    subscription_number: 'SUB-2026-0002',
    customer_id: 'cust-demo-2',
    customer_name: 'Acme Logistics Corp',
    plan_id: 'plan-002',
    plan_name: 'Annual Enterprise Plan',
    status: 'QUOTATION',
    start_date: '2026-04-01T00:00:00Z',
    expiry_date: '2026-04-15T00:00:00Z',
    payment_terms: 'IMMEDIATE',
    subtotal_cents: 6000000, // $60,000.00
    tax_cents: 450000,
    total_cents: 6450000,
    lines: [
      {
        id: 'subline-2',
        product_id: 'prod-001',
        product_name: 'BrightERP Core',
        variant_id: 'var-001-std',
        variant_name: 'Standard Edition',
        quantity: 5,
        unit_price_cents: 1200000,
        tax_rate_bp: 750,
        tax_cents: 450000,
        discount_cents: 0,
        total_cents: 6450000,
      },
    ],
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
  },
  {
    id: 'sub-003',
    subscription_number: 'SUB-2026-0003',
    customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd',
    plan_id: 'plan-003',
    plan_name: 'Addon Monthly Recurring',
    status: 'DRAFT',
    start_date: '2026-04-01T00:00:00Z',
    expiry_date: null,
    payment_terms: 'NET_15',
    subtotal_cents: 45000,
    tax_cents: 3375,
    total_cents: 48375,
    lines: [
      {
        id: 'subline-3',
        product_id: 'prod-002',
        product_name: 'Accounting & Payroll Add-on',
        quantity: 1,
        unit_price_cents: 45000,
        tax_rate_bp: 750,
        tax_cents: 3375,
        discount_cents: 0,
        total_cents: 48375,
      },
    ],
    created_at: '2026-03-20T00:00:00Z',
    updated_at: '2026-03-20T00:00:00Z',
  },
];

let mockSubscriptions = [...INITIAL_SUBSCRIPTIONS];
let mockTemplates = [...INITIAL_TEMPLATES];

export const subscriptionsApi = {
  list: async (params?: { customer_id?: string; status?: string; search?: string; page?: number; per_page?: number }) => {
    try {
      return await api.get<Subscription[]>('/subscriptions', params);
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        let filtered = [...mockSubscriptions];
        if (params?.customer_id) {
          filtered = filtered.filter((s) => s.customer_id === params.customer_id);
        }
        if (params?.status) {
          filtered = filtered.filter((s) => s.status === params.status);
        }
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter(
            (s) =>
              s.subscription_number.toLowerCase().includes(q) ||
              (s.customer_name && s.customer_name.toLowerCase().includes(q))
          );
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

  get: async (id: string): Promise<Subscription> => {
    try {
      const res = await api.get<Subscription>(`/subscriptions/${id}`);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const found = mockSubscriptions.find((s) => s.id === id);
        if (found) return found;
      }
      throw err;
    }
  },

  create: async (data: {
    customer_id: string;
    customer_name?: string;
    plan_id: string;
    plan_name?: string;
    start_date: string;
    expiry_date?: string | null;
    payment_terms: string;
    lines: {
      product_id: string;
      product_name: string;
      variant_id?: string | null;
      variant_name?: string | null;
      quantity: number;
      unit_price_cents: number;
      tax_rate_bp?: number;
    }[];
  }): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>('/subscriptions', data);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const id = `sub-${Date.now().toString(36)}`;
        const subLines = data.lines.map((l, idx) => {
          const subtotal = l.quantity * l.unit_price_cents;
          const tax = Math.round((subtotal * (l.tax_rate_bp || 750)) / 10000);
          return {
            id: `line-${id}-${idx}`,
            product_id: l.product_id,
            product_name: l.product_name,
            variant_id: l.variant_id,
            variant_name: l.variant_name,
            quantity: l.quantity,
            unit_price_cents: l.unit_price_cents,
            tax_rate_bp: l.tax_rate_bp || 750,
            tax_cents: tax,
            discount_cents: 0,
            total_cents: subtotal + tax,
          };
        });

        const subtotal = subLines.reduce((acc, l) => acc + l.quantity * l.unit_price_cents, 0);
        const taxTotal = subLines.reduce((acc, l) => acc + l.tax_cents, 0);

        const sub: Subscription = {
          id,
          subscription_number: `SUB-2026-${Math.floor(1000 + Math.random() * 9000)}`,
          customer_id: data.customer_id,
          customer_name: data.customer_name || 'Customer',
          plan_id: data.plan_id,
          plan_name: data.plan_name || 'Selected Plan',
          status: 'DRAFT',
          start_date: data.start_date,
          expiry_date: data.expiry_date || null,
          payment_terms: data.payment_terms,
          subtotal_cents: subtotal,
          tax_cents: taxTotal,
          total_cents: subtotal + taxTotal,
          lines: subLines,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockSubscriptions.unshift(sub);
        return sub;
      }
      throw err;
    }
  },

  transition: async (
    id: string,
    targetStatus: SubscriptionStatus
  ): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${id}/transition`, {
        target_status: targetStatus,
      });
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          sub.status = targetStatus;
          sub.updated_at = new Date().toISOString();
          return { ...sub };
        }
      }
      throw err;
    }
  },

  listTemplates: async () => {
    try {
      const res = await api.get<QuotationTemplate[]>('/quotation-templates');
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        return mockTemplates;
      }
      throw err;
    }
  },

  createTemplate: async (data: Omit<QuotationTemplate, 'id'>) => {
    try {
      const res = await api.post<QuotationTemplate>('/quotation-templates', data);
      return res.data;
    } catch (err) {
      if (err instanceof ApiException && (err.status === 404 || err.status === 501)) {
        const tmpl: QuotationTemplate = {
          ...data,
          id: `tmpl-${Date.now().toString(36)}`,
        };
        mockTemplates.unshift(tmpl);
        return tmpl;
      }
      throw err;
    }
  },
};
