import { api, isOfflineError } from './client';
import {
  BillingRunResult,
  QuotationTemplate,
  Subscription,
  SubscriptionLine,
  SubscriptionStatus,
} from './types';

const INITIAL_TEMPLATES: QuotationTemplate[] = [
  {
    id: 'tmpl-001',
    name: 'Standard Mid-Market Onboarding (Monthly)',
    validity_days: 30,
    notes: 'Default package for new mid-market software contracts',
    is_active: true,
    lines: [
      { id: 'tmpl-line-1', plan_id: 'plan-001', plan_name: 'Monthly Growth Plan', quantity: 1, position: 1 },
      { id: 'tmpl-line-2', plan_id: 'plan-003', plan_name: 'Addon Monthly Recurring', quantity: 1, position: 2 },
    ],
  },
  {
    id: 'tmpl-002',
    name: 'Enterprise Full Stack Suite (Yearly)',
    validity_days: 14,
    notes: 'High-volume annual subscription with bundled multi-site support',
    is_active: true,
    lines: [
      { id: 'tmpl-line-3', plan_id: 'plan-002', plan_name: 'Annual Enterprise Plan', quantity: 5, position: 1 },
    ],
  },
];

const INITIAL_SUBSCRIPTIONS: Subscription[] = [
  {
    id: 'sub-001',
    number: 'SUB-2026-0001',
    subscription_number: 'SUB-2026-0001',
    customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd',
    plan_id: 'plan-001',
    plan_name: 'Monthly Growth Plan',
    status: 'ACTIVE',
    start_date: '2026-03-01T00:00:00Z',
    expiry_date: null,
    payment_terms_days: 30,
    payment_terms: 'NET_30',
    quantity: 1,
    discount_id: null,
    next_billing_date: '2026-04-01T00:00:00Z',
    subtotal_cents: 176000, // $1,760.00
    tax_cents: 31680,      // 18%
    total_cents: 207680,   // $2,076.80
    lines: [
      {
        id: 'subline-1',
        subscription_id: 'sub-001',
        product_id: 'prod-001',
        product_name: 'BrightERP Core Platform',
        variant_id: 'var-001-odoo',
        variant_name: 'Odoo Connected Brand (+560.00)',
        description: 'BrightERP Core with Odoo brand add-on',
        quantity: 1,
        unit_price_cents: 176000,
        tax_id: 'tax-vat18',
        tax_rate_bp: 1800,
        tax_cents: 31680,
        discount_cents: 0,
        total_cents: 207680,
        position: 1,
      },
    ],
    created_at: '2026-03-01T00:00:00Z',
    updated_at: '2026-03-01T00:00:00Z',
  },
  {
    id: 'sub-002',
    number: 'SUB-2026-0002',
    subscription_number: 'SUB-2026-0002',
    customer_id: 'cust-demo-2',
    customer_name: 'Acme Logistics Corp',
    plan_id: 'plan-002',
    plan_name: 'Annual Enterprise Plan',
    status: 'QUOTATION',
    start_date: '2026-04-01T00:00:00Z',
    expiry_date: '2026-04-15T00:00:00Z',
    payment_terms_days: 15,
    payment_terms: 'NET_15',
    quantity: 5,
    discount_id: 'disc-early10',
    quote_expires_at: '2026-04-15T00:00:00Z',
    subtotal_cents: 6000000, // $60,000.00
    tax_cents: 1080000,
    total_cents: 7080000,
    lines: [
      {
        id: 'subline-2',
        subscription_id: 'sub-002',
        product_id: 'prod-001',
        product_name: 'BrightERP Core Platform',
        variant_id: 'var-001-std',
        variant_name: 'Standard Edition',
        description: 'BrightERP Enterprise Annual License',
        quantity: 5,
        unit_price_cents: 1200000,
        tax_id: 'tax-vat18',
        tax_rate_bp: 1800,
        tax_cents: 1080000,
        discount_cents: 0,
        total_cents: 7080000,
        position: 1,
      },
    ],
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
  },
  {
    id: 'sub-003',
    number: 'SUB-2026-0003',
    subscription_number: 'SUB-2026-0003',
    customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd',
    plan_id: 'plan-003',
    plan_name: 'Addon Monthly Recurring',
    status: 'DRAFT',
    start_date: '2026-04-01T00:00:00Z',
    expiry_date: null,
    payment_terms_days: 30,
    payment_terms: 'NET_30',
    quantity: 1,
    discount_id: null,
    subtotal_cents: 45000,
    tax_cents: 8100,
    total_cents: 53100,
    lines: [
      {
        id: 'subline-3',
        subscription_id: 'sub-003',
        product_id: 'prod-002',
        product_name: 'Accounting & Payroll Add-on',
        description: 'Payroll automation recurring line',
        quantity: 1,
        unit_price_cents: 45000,
        tax_id: 'tax-vat18',
        tax_rate_bp: 1800,
        tax_cents: 8100,
        discount_cents: 0,
        total_cents: 53100,
        position: 1,
      },
    ],
    created_at: '2026-03-20T00:00:00Z',
    updated_at: '2026-03-20T00:00:00Z',
  },
];

let mockSubscriptions = [...INITIAL_SUBSCRIPTIONS];
let mockTemplates = [...INITIAL_TEMPLATES];

export const subscriptionsApi = {
  list: async (params?: {
    customer_id?: string;
    plan_id?: string;
    status?: string;
    search?: string;
    page?: number;
    per_page?: number;
  }) => {
    try {
      return await api.get<Subscription[]>('/subscriptions', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockSubscriptions];
        if (params?.customer_id) {
          filtered = filtered.filter((s) => s.customer_id === params.customer_id);
        }
        if (params?.plan_id) {
          filtered = filtered.filter((s) => s.plan_id === params.plan_id);
        }
        if (params?.status) {
          filtered = filtered.filter((s) => s.status === params.status);
        }
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter(
            (s) =>
              (s.number && s.number.toLowerCase().includes(q)) ||
              (s.subscription_number && s.subscription_number.toLowerCase().includes(q)) ||
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
      if (isOfflineError(err)) {
        const found = mockSubscriptions.find((s) => s.id === id);
        if (found) return found;
      }
      throw err;
    }
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
    try {
      const res = await api.post<Subscription>('/subscriptions', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const id = `sub-${Date.now().toString(36)}`;
        const subLines: SubscriptionLine[] = (data.lines || []).map((l, idx) => {
          const subtotal = l.quantity * l.unit_price_cents;
          const taxRate = 1800; // 18% default fallback
          const tax = Math.round((subtotal * taxRate) / 10000);
          return {
            id: `line-${id}-${idx}`,
            subscription_id: id,
            product_id: l.product_id,
            product_name: l.product_name || 'Product Item',
            variant_id: l.variant_id,
            variant_name: l.variant_name,
            description: l.description || 'Subscription Line',
            quantity: l.quantity,
            unit_price_cents: l.unit_price_cents,
            tax_id: l.tax_id || null,
            tax_rate_bp: taxRate,
            tax_cents: tax,
            discount_cents: 0,
            total_cents: subtotal + tax,
            position: idx + 1,
          };
        });

        const subtotal = subLines.reduce((acc, l) => acc + l.quantity * l.unit_price_cents, 0);
        const taxTotal = subLines.reduce((acc, l) => acc + (l.tax_cents || 0), 0);
        const num = `SUB-2026-${Math.floor(1000 + Math.random() * 9000)}`;

        const sub: Subscription = {
          id,
          number: num,
          subscription_number: num,
          customer_id: data.customer_id,
          customer_name: data.customer_name || 'Customer Profile',
          plan_id: data.plan_id,
          plan_name: data.plan_name || 'Selected Plan',
          status: (data.status as SubscriptionStatus) || 'DRAFT',
          start_date: data.start_date,
          expiry_date: data.expiry_date || null,
          payment_terms_days: data.payment_terms_days || 30,
          payment_terms: `NET_${data.payment_terms_days || 30}`,
          quantity: data.quantity || 1,
          discount_id: data.discount_id || null,
          quote_expires_at: data.quote_expires_at || null,
          notes: data.notes || null,
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

  update: async (id: string, data: Partial<Subscription>): Promise<Subscription> => {
    try {
      const res = await api.patch<Subscription>(`/subscriptions/${id}`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          Object.assign(sub, data, { updated_at: new Date().toISOString() });
          return { ...sub };
        }
      }
      throw err;
    }
  },

  // State transitions
  confirm: async (id: string): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${id}/confirm`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          sub.status = 'CONFIRMED';
          sub.updated_at = new Date().toISOString();
          return { ...sub };
        }
      }
      throw err;
    }
  },

  activate: async (id: string): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${id}/activate`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          sub.status = 'ACTIVE';
          sub.activated_at = new Date().toISOString();
          sub.next_billing_date = sub.start_date;
          sub.updated_at = new Date().toISOString();
          return { ...sub };
        }
      }
      throw err;
    }
  },

  pause: async (id: string): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${id}/pause`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          sub.status = 'PAUSED';
          sub.paused_at = new Date().toISOString();
          sub.next_billing_date = null;
          sub.updated_at = new Date().toISOString();
          return { ...sub };
        }
      }
      throw err;
    }
  },

  resume: async (id: string): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${id}/resume`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          sub.status = 'ACTIVE';
          sub.next_billing_date = new Date().toISOString();
          sub.updated_at = new Date().toISOString();
          return { ...sub };
        }
      }
      throw err;
    }
  },

  close: async (id: string): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${id}/close`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === id);
        if (sub) {
          sub.status = 'CLOSED';
          sub.closed_at = new Date().toISOString();
          sub.next_billing_date = null;
          sub.updated_at = new Date().toISOString();
          return { ...sub };
        }
      }
      throw err;
    }
  },

  // Line management
  addLine: async (
    subId: string,
    line: { product_id: string; variant_id?: string; description?: string; quantity: number; unit_price_cents: number; tax_id?: string }
  ): Promise<Subscription> => {
    try {
      const res = await api.post<Subscription>(`/subscriptions/${subId}/lines`, line);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === subId);
        if (sub) {
          const newLine: SubscriptionLine = {
            id: `line-${Date.now().toString(36)}`,
            subscription_id: subId,
            product_id: line.product_id,
            variant_id: line.variant_id || null,
            description: line.description || '',
            quantity: line.quantity,
            unit_price_cents: line.unit_price_cents,
            tax_id: line.tax_id || null,
            tax_rate_bp: 1800,
            tax_cents: Math.round((line.quantity * line.unit_price_cents * 1800) / 10000),
            discount_cents: 0,
            total_cents: line.quantity * line.unit_price_cents,
            position: sub.lines.length + 1,
          };
          sub.lines.push(newLine);
          return { ...sub };
        }
      }
      throw err;
    }
  },

  deleteLine: async (subId: string, lineId: string): Promise<Subscription> => {
    try {
      const res = await api.del<Subscription>(`/subscriptions/${subId}/lines/${lineId}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const sub = mockSubscriptions.find((s) => s.id === subId);
        if (sub) {
          sub.lines = sub.lines.filter((l) => l.id !== lineId);
          return { ...sub };
        }
      }
      throw err;
    }
  },

  // Quotation Templates
  listTemplates: async (params?: { search?: string; is_active?: boolean }) => {
    try {
      const res = await api.get<QuotationTemplate[]>('/quotations/templates', params);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockTemplates];
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter((t) => t.name.toLowerCase().includes(q));
        }
        if (params?.is_active !== undefined) {
          filtered = filtered.filter((t) => t.is_active === params.is_active);
        }
        return filtered;
      }
      throw err;
    }
  },

  createTemplate: async (data: {
    name: string;
    validity_days: number;
    notes?: string;
    lines: { plan_id: string; quantity: number }[];
  }): Promise<QuotationTemplate> => {
    try {
      const res = await api.post<QuotationTemplate>('/quotations/templates', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const tmpl: QuotationTemplate = {
          id: `tmpl-${Date.now().toString(36)}`,
          name: data.name,
          validity_days: data.validity_days,
          notes: data.notes || null,
          is_active: true,
          lines: data.lines.map((l, i) => ({
            id: `tl-${i}`,
            plan_id: l.plan_id,
            quantity: l.quantity,
            position: i + 1,
          })),
          created_at: new Date().toISOString(),
        };
        mockTemplates.unshift(tmpl);
        return tmpl;
      }
      throw err;
    }
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
    try {
      const res = await api.post<Subscription>(`/quotations/templates/${templateId}/instantiate`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const tmpl = mockTemplates.find((t) => t.id === templateId);
        const subNum = `QUOTE-2026-${Math.floor(1000 + Math.random() * 9000)}`;
        const expiry = new Date(Date.now() + (tmpl?.validity_days || 30) * 86400000).toISOString();
        const sub: Subscription = {
          id: `sub-${Date.now().toString(36)}`,
          number: subNum,
          subscription_number: subNum,
          customer_id: data.customer_id,
          customer_name: data.customer_name || 'Customer',
          plan_id: tmpl?.lines[0]?.plan_id || 'plan-001',
          plan_name: tmpl?.lines[0]?.plan_name || 'Plan from Template',
          status: 'QUOTATION',
          start_date: data.start_date || new Date().toISOString(),
          expiry_date: expiry,
          quote_expires_at: expiry,
          payment_terms_days: data.payment_terms_days || 30,
          payment_terms: `NET_${data.payment_terms_days || 30}`,
          quantity: data.quantity || 1,
          discount_id: data.discount_id || null,
          notes: data.notes || tmpl?.notes || null,
          subtotal_cents: 120000,
          tax_cents: 21600,
          total_cents: 141600,
          lines: [
            {
              id: `line-${Date.now().toString(36)}`,
              product_id: 'prod-001',
              product_name: 'BrightERP Core Platform',
              description: tmpl?.name || 'Quotation item',
              quantity: data.quantity || 1,
              unit_price_cents: 120000,
              tax_rate_bp: 1800,
              tax_cents: 21600,
              discount_cents: 0,
              total_cents: 141600,
              position: 1,
            },
          ],
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockSubscriptions.unshift(sub);
        return sub;
      }
      throw err;
    }
  },

  // Billing Run
  runBilling: async (): Promise<BillingRunResult> => {
    try {
      const res = await api.post<BillingRunResult>('/billing/run');
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const activeSubs = mockSubscriptions.filter((s) => s.status === 'ACTIVE');
        return {
          as_of: new Date().toISOString(),
          billed: activeSubs.length,
          skipped: 0,
          closed: 0,
          failed: 0,
          invoices: activeSubs.map((s) => `inv-${s.id}`),
          results: activeSubs.map((s) => ({
            subscription_id: s.id,
            period_start: new Date().toISOString(),
            period_end: new Date(Date.now() + 30 * 86400000).toISOString(),
            invoice_id: `inv-${s.id}`,
            status: 'CREATED',
          })),
        };
      }
      throw err;
    }
  },
};
