/**
 * finance.ts — API client for invoices, payments, taxes, discounts, and reports.
 * All money values are in integer cents (BIGINT on the backend).
 * Basis-point rates: 100 bp = 1 %.
 */

import { api, ApiException } from './client';

// ─── Shared ──────────────────────────────────────────────────────────────────

export interface PaginatedMeta {
  request_id: string;
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

// ─── Invoice types ────────────────────────────────────────────────────────────

export type InvoiceStatus = 'DRAFT' | 'CONFIRMED' | 'PAID' | 'CANCELLED';

export interface InvoiceLine {
  id: string;
  invoice_id: string;
  product_id: string | null;
  variant_id: string | null;
  description: string;
  quantity: number;
  unit_price_cents: number;
  tax_rate_bp: number;
  tax_cents: number;
  discount_cents: number;
  line_total_cents: number;
  position: number;
}

export interface Invoice {
  id: string;
  number: string;
  customer_id: string;
  customer_name?: string;
  subscription_id: string | null;
  status: InvoiceStatus;
  issue_date: string;
  due_date: string;
  period_start: string | null;
  period_end: string | null;
  subtotal_cents: number;
  discount_cents: number;
  tax_cents: number;
  total_cents: number;
  balance_cents: number;
  notes: string | null;
  confirmed_at: string | null;
  sent_at: string | null;
  paid_at: string | null;
  cancelled_at: string | null;
  created_at: string;
  updated_at: string;
  lines?: InvoiceLine[];
}

export interface InvoiceCreateInput {
  customer_id: string;
  subscription_id?: string;
  issue_date?: string;
  due_date?: string;
  period_start?: string;
  period_end?: string;
  notes?: string;
  lines: {
    product_id?: string;
    variant_id?: string;
    description: string;
    quantity: number;
    unit_price_cents: number;
    tax_rate_bp?: number;
    discount_cents?: number;
  }[];
}

export interface InvoiceUpdateInput {
  due_date?: string;
  notes?: string;
}

// ─── Payment types ────────────────────────────────────────────────────────────

export type PaymentMethod = 'CASH' | 'BANK_TRANSFER' | 'CARD' | 'MOBILE_MONEY' | 'OTHER';

export interface Payment {
  id: string;
  invoice_id: string;
  method: PaymentMethod;
  amount_cents: number;
  paid_at: string;
  note: string | null;
  recorded_by: string | null;
  created_at: string;
}

export interface PaymentRecordInput {
  invoice_id: string;
  method: PaymentMethod;
  amount_cents: number;
  paid_at?: string;
  note?: string;
}

// ─── Tax types ────────────────────────────────────────────────────────────────

export interface Tax {
  id: string;
  name: string;
  rate_bp: number;
  is_inclusive: boolean;
  country_code: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface TaxCreateInput {
  name: string;
  rate_bp: number;
  is_inclusive?: boolean;
  country_code?: string;
}

export interface TaxUpdateInput {
  name?: string;
  rate_bp?: number;
  is_inclusive?: boolean;
  country_code?: string;
  is_active?: boolean;
}

// ─── Discount types ───────────────────────────────────────────────────────────

export type DiscountKind = 'FIXED' | 'PERCENT';
export type DiscountScope = 'SUBSCRIPTION' | 'PRODUCT';

export interface Discount {
  id: string;
  name: string;
  kind: DiscountKind;
  value_cents: number;
  value_bp: number;
  scope: DiscountScope;
  product_id: string | null;
  min_purchase_cents: number;
  min_quantity: number;
  starts_at: string | null;
  ends_at: string | null;
  max_uses: number | null;
  used_count: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface DiscountCreateInput {
  name: string;
  kind: DiscountKind;
  value_cents?: number;
  value_bp?: number;
  scope?: DiscountScope;
  product_id?: string;
  min_purchase_cents?: number;
  min_quantity?: number;
  starts_at?: string;
  ends_at?: string;
  max_uses?: number;
}

export interface DiscountUpdateInput {
  name?: string;
  kind?: DiscountKind;
  value_cents?: number;
  value_bp?: number;
  scope?: DiscountScope;
  product_id?: string;
  min_purchase_cents?: number;
  min_quantity?: number;
  starts_at?: string;
  ends_at?: string;
  max_uses?: number;
  is_active?: boolean;
}

// ─── Reports types ────────────────────────────────────────────────────────────

export interface ReportSummary {
  active_subscriptions: number;
  paused_subscriptions: number;
  open_quotations: number;
  mrr_cents: number;
  outstanding_cents: number;
  overdue_invoices: number;
  overdue_cents: number;
  collected_this_month_cents: number;
}

export interface RevenuePoint {
  month: string;
  paid_cents: number;
  invoiced_cents: number;
}

export interface StatusCount {
  status: string;
  count: number;
}

export interface OverdueInvoice {
  id: string;
  number: string;
  customer_id: string;
  customer_name: string;
  due_date: string;
  days_late: number;
  balance_cents: number;
  total_cents: number;
}

// ─── Mock data (used when backend is offline) ─────────────────────────────────

const MOCK_INVOICES: Invoice[] = [
  {
    id: 'inv-001', number: 'INV-2026-0001', customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd', subscription_id: 'sub-001',
    status: 'CONFIRMED', issue_date: '2026-10-01', due_date: '2026-10-31',
    period_start: '2026-10-01', period_end: '2026-10-31',
    subtotal_cents: 176000, discount_cents: 0, tax_cents: 13200,
    total_cents: 189200, balance_cents: 189200,
    notes: null, confirmed_at: '2026-10-02T08:00:00Z',
    sent_at: '2026-10-02T08:01:00Z', paid_at: null, cancelled_at: null,
    created_at: '2026-10-01T07:00:00Z', updated_at: '2026-10-02T08:01:00Z',
  },
  {
    id: 'inv-002', number: 'INV-2026-0002', customer_id: 'cust-demo-2',
    customer_name: 'Acme Logistics Corp', subscription_id: 'sub-002',
    status: 'PAID', issue_date: '2026-09-01', due_date: '2026-09-30',
    period_start: '2026-09-01', period_end: '2026-09-30',
    subtotal_cents: 6000000, discount_cents: 300000, tax_cents: 427500,
    total_cents: 6127500, balance_cents: 0,
    notes: 'Early payment discount applied', confirmed_at: '2026-09-02T08:00:00Z',
    sent_at: '2026-09-02T08:05:00Z', paid_at: '2026-09-10T14:22:00Z', cancelled_at: null,
    created_at: '2026-09-01T07:00:00Z', updated_at: '2026-09-10T14:22:00Z',
  },
  {
    id: 'inv-003', number: 'INV-2026-0003', customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd', subscription_id: null,
    status: 'DRAFT', issue_date: '2026-10-20', due_date: '2026-11-19',
    period_start: null, period_end: null,
    subtotal_cents: 45000, discount_cents: 0, tax_cents: 3375,
    total_cents: 48375, balance_cents: 48375,
    notes: null, confirmed_at: null, sent_at: null, paid_at: null, cancelled_at: null,
    created_at: '2026-10-20T10:00:00Z', updated_at: '2026-10-20T10:00:00Z',
  },
  {
    id: 'inv-004', number: 'INV-2026-0004', customer_id: 'cust-demo-3',
    customer_name: 'Summit Ventures', subscription_id: 'sub-003',
    status: 'CONFIRMED', issue_date: '2026-08-01', due_date: '2026-08-31',
    period_start: '2026-08-01', period_end: '2026-08-31',
    subtotal_cents: 350000, discount_cents: 0, tax_cents: 26250,
    total_cents: 376250, balance_cents: 376250,
    notes: null, confirmed_at: '2026-08-02T09:00:00Z',
    sent_at: '2026-08-02T09:05:00Z', paid_at: null, cancelled_at: null,
    created_at: '2026-08-01T07:00:00Z', updated_at: '2026-08-02T09:05:00Z',
  },
];

const MOCK_PAYMENTS: Payment[] = [
  {
    id: 'pay-001', invoice_id: 'inv-002', method: 'BANK_TRANSFER',
    amount_cents: 6127500, paid_at: '2026-09-10T14:22:00Z',
    note: 'Wire transfer ref #TXN-99281', recorded_by: null,
    created_at: '2026-09-10T14:22:00Z',
  },
  {
    id: 'pay-002', invoice_id: 'inv-001', method: 'CARD',
    amount_cents: 50000, paid_at: '2026-10-15T09:00:00Z',
    note: 'Partial payment', recorded_by: null,
    created_at: '2026-10-15T09:00:00Z',
  },
];

const MOCK_TAXES: Tax[] = [
  { id: 'tax-001', name: 'Standard GST (India)', rate_bp: 1800, is_inclusive: false, country_code: 'IN', is_active: true, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'tax-002', name: 'US Sales Tax (CA)', rate_bp: 725, is_inclusive: false, country_code: 'US', is_active: true, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'tax-003', name: 'UK VAT', rate_bp: 2000, is_inclusive: true, country_code: 'GB', is_active: true, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'tax-004', name: 'EU VAT (Standard)', rate_bp: 2100, is_inclusive: false, country_code: 'DE', is_active: false, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'tax-005', name: 'Zero Rate', rate_bp: 0, is_inclusive: false, country_code: '', is_active: true, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
];

const MOCK_DISCOUNTS: Discount[] = [
  { id: 'disc-001', name: 'Early Bird 15%', kind: 'PERCENT', value_cents: 0, value_bp: 1500, scope: 'SUBSCRIPTION', product_id: null, min_purchase_cents: 0, min_quantity: 0, starts_at: '2026-01-01T00:00:00Z', ends_at: '2026-12-31T23:59:59Z', max_uses: 100, used_count: 23, is_active: true, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'disc-002', name: 'Flat $500 Off', kind: 'FIXED', value_cents: 50000, value_bp: 0, scope: 'SUBSCRIPTION', product_id: null, min_purchase_cents: 200000, min_quantity: 0, starts_at: null, ends_at: null, max_uses: null, used_count: 7, is_active: true, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 'disc-003', name: 'BrightERP Promo 5%', kind: 'PERCENT', value_cents: 0, value_bp: 500, scope: 'PRODUCT', product_id: 'prod-001', min_purchase_cents: 0, min_quantity: 1, starts_at: '2026-10-01T00:00:00Z', ends_at: '2026-11-30T23:59:59Z', max_uses: 50, used_count: 50, is_active: false, created_at: '2026-06-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z' },
];

const MOCK_SUMMARY: ReportSummary = {
  active_subscriptions: 42,
  paused_subscriptions: 6,
  open_quotations: 11,
  mrr_cents: 8_420_000,
  outstanding_cents: 3_180_000,
  overdue_invoices: 8,
  overdue_cents: 1_250_000,
  collected_this_month_cents: 5_960_000,
};

const MOCK_REVENUE: RevenuePoint[] = [
  { month: '2026-05', paid_cents: 4_200_000, invoiced_cents: 5_100_000 },
  { month: '2026-06', paid_cents: 5_800_000, invoiced_cents: 6_200_000 },
  { month: '2026-07', paid_cents: 6_100_000, invoiced_cents: 6_400_000 },
  { month: '2026-08', paid_cents: 4_900_000, invoiced_cents: 7_100_000 },
  { month: '2026-09', paid_cents: 7_300_000, invoiced_cents: 7_700_000 },
  { month: '2026-10', paid_cents: 5_960_000, invoiced_cents: 8_420_000 },
];

const MOCK_STATUS_COUNTS: StatusCount[] = [
  { status: 'ACTIVE', count: 42 },
  { status: 'PAUSED', count: 6 },
  { status: 'QUOTATION', count: 11 },
  { status: 'CONFIRMED', count: 5 },
  { status: 'DRAFT', count: 4 },
  { status: 'CLOSED', count: 18 },
];

const MOCK_OVERDUE: OverdueInvoice[] = [
  { id: 'inv-004', number: 'INV-2026-0004', customer_id: 'cust-demo-3', customer_name: 'Summit Ventures', due_date: '2026-08-31', days_late: 61, balance_cents: 376250, total_cents: 376250 },
  { id: 'inv-005', number: 'INV-2026-0005', customer_id: 'cust-demo-4', customer_name: 'Zephyr Tech Labs', due_date: '2026-09-15', days_late: 46, balance_cents: 89000, total_cents: 89000 },
  { id: 'inv-006', number: 'INV-2026-0006', customer_id: 'cust-demo-5', customer_name: 'Orion Media Group', due_date: '2026-10-10', days_late: 21, balance_cents: 212500, total_cents: 212500 },
];

// ─── Helper ───────────────────────────────────────────────────────────────────

function isFallbackError(err: unknown): boolean {
  if (err instanceof ApiException) {
    return err.status === 404 || err.status === 501 || err.status === 502 || err.status === 503 || err.code === 'BACKEND_OFFLINE';
  }
  if (err instanceof TypeError) return true;
  return false;
}

// ─── Invoices API ─────────────────────────────────────────────────────────────

let mockInvoices = [...MOCK_INVOICES];

export const invoicesApi = {
  list: async (params?: { status?: string; search?: string; customer_id?: string; overdue?: boolean; page?: number; per_page?: number }) => {
    try {
      return await api.get<Invoice[]>('/invoices', params as Record<string, string>);
    } catch (err) {
      if (isFallbackError(err)) {
        let result = [...mockInvoices];
        if (params?.status) result = result.filter((i) => i.status === params.status);
        if (params?.overdue) result = result.filter((i) => i.status === 'CONFIRMED' && i.balance_cents > 0);
        if (params?.search) {
          const q = params.search.toLowerCase();
          result = result.filter((i) => i.number.toLowerCase().includes(q) || (i.customer_name ?? '').toLowerCase().includes(q));
        }
        if (params?.customer_id) result = result.filter((i) => i.customer_id === params.customer_id);
        return { data: result, meta: { request_id: 'mock', page: 1, per_page: 25, total: result.length, total_pages: 1 } };
      }
      throw err;
    }
  },

  get: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.get<Invoice>(`/invoices/${id}`);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const found = mockInvoices.find((i) => i.id === id);
        if (found) return { ...found };
      }
      throw err;
    }
  },

  create: async (input: InvoiceCreateInput): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>('/invoices', input);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const newInv: Invoice = {
          id: `inv-${Date.now().toString(36)}`,
          number: `INV-2026-${Math.floor(1000 + Math.random() * 9000)}`,
          customer_id: input.customer_id,
          subscription_id: input.subscription_id ?? null,
          status: 'DRAFT',
          issue_date: input.issue_date ?? new Date().toISOString().slice(0, 10),
          due_date: input.due_date ?? '',
          period_start: input.period_start ?? null,
          period_end: input.period_end ?? null,
          subtotal_cents: input.lines.reduce((a, l) => a + l.quantity * l.unit_price_cents, 0),
          discount_cents: input.lines.reduce((a, l) => a + (l.discount_cents ?? 0), 0),
          tax_cents: input.lines.reduce((a, l) => a + Math.round(l.quantity * l.unit_price_cents * (l.tax_rate_bp ?? 750) / 10000), 0),
          total_cents: 0,
          balance_cents: 0,
          notes: input.notes ?? null,
          confirmed_at: null, sent_at: null, paid_at: null, cancelled_at: null,
          created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
        };
        newInv.total_cents = newInv.subtotal_cents + newInv.tax_cents - newInv.discount_cents;
        newInv.balance_cents = newInv.total_cents;
        mockInvoices.unshift(newInv);
        return newInv;
      }
      throw err;
    }
  },

  confirm: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>(`/invoices/${id}/confirm`);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) { inv.status = 'CONFIRMED'; inv.confirmed_at = new Date().toISOString(); return { ...inv }; }
      }
      throw err;
    }
  },

  cancel: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>(`/invoices/${id}/cancel`);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) { inv.status = 'CANCELLED'; inv.cancelled_at = new Date().toISOString(); return { ...inv }; }
      }
      throw err;
    }
  },

  send: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>(`/invoices/${id}/send`);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) { inv.sent_at = new Date().toISOString(); return { ...inv }; }
      }
      throw err;
    }
  },
};

// ─── Payments API ─────────────────────────────────────────────────────────────

let mockPayments = [...MOCK_PAYMENTS];

export const paymentsApi = {
  list: async (params?: { invoice_id?: string; page?: number; per_page?: number }) => {
    try {
      return await api.get<Payment[]>('/payments', params as Record<string, string>);
    } catch (err) {
      if (isFallbackError(err)) {
        let result = [...mockPayments];
        if (params?.invoice_id) result = result.filter((p) => p.invoice_id === params.invoice_id);
        return { data: result, meta: { request_id: 'mock', page: 1, per_page: 25, total: result.length, total_pages: 1 } };
      }
      throw err;
    }
  },

  record: async (input: PaymentRecordInput): Promise<{ payment_recorded: boolean; invoice: Invoice }> => {
    try {
      const res = await api.post<{ payment_recorded: boolean; invoice: Invoice }>('/payments', input);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const p: Payment = {
          id: `pay-${Date.now().toString(36)}`,
          invoice_id: input.invoice_id,
          method: input.method,
          amount_cents: input.amount_cents,
          paid_at: input.paid_at ?? new Date().toISOString(),
          note: input.note ?? null,
          recorded_by: null,
          created_at: new Date().toISOString(),
        };
        mockPayments.unshift(p);
        const inv = mockInvoices.find((i) => i.id === input.invoice_id);
        if (inv) {
          inv.balance_cents = Math.max(0, inv.balance_cents - input.amount_cents);
          if (inv.balance_cents === 0) { inv.status = 'PAID'; inv.paid_at = new Date().toISOString(); }
        }
        return { payment_recorded: true, invoice: inv ? { ...inv } : {} as Invoice };
      }
      throw err;
    }
  },
};

// ─── Taxes API ────────────────────────────────────────────────────────────────

let mockTaxes = [...MOCK_TAXES];

export const taxesApi = {
  list: async (params?: { search?: string; active?: boolean; page?: number; per_page?: number }) => {
    try {
      return await api.get<Tax[]>('/taxes', params as Record<string, string>);
    } catch (err) {
      if (isFallbackError(err)) {
        let result = [...mockTaxes];
        if (params?.search) { const q = params.search.toLowerCase(); result = result.filter((t) => t.name.toLowerCase().includes(q) || t.country_code.toLowerCase().includes(q)); }
        if (params?.active) result = result.filter((t) => t.is_active);
        return { data: result, meta: { request_id: 'mock', page: 1, per_page: 25, total: result.length, total_pages: 1 } };
      }
      throw err;
    }
  },

  create: async (input: TaxCreateInput): Promise<Tax> => {
    try {
      const res = await api.post<Tax>('/taxes', input);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const t: Tax = { id: `tax-${Date.now().toString(36)}`, name: input.name, rate_bp: input.rate_bp, is_inclusive: input.is_inclusive ?? false, country_code: input.country_code ?? '', is_active: true, created_at: new Date().toISOString(), updated_at: new Date().toISOString() };
        mockTaxes.unshift(t);
        return t;
      }
      throw err;
    }
  },

  update: async (id: string, input: TaxUpdateInput): Promise<Tax> => {
    try {
      const res = await api.patch<Tax>(`/taxes/${id}`, input);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const t = mockTaxes.find((x) => x.id === id);
        if (t) { Object.assign(t, input); t.updated_at = new Date().toISOString(); return { ...t }; }
      }
      throw err;
    }
  },

  deactivate: async (id: string): Promise<Tax> => {
    try {
      const res = await api.del<Tax>(`/taxes/${id}`);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const t = mockTaxes.find((x) => x.id === id);
        if (t) { t.is_active = false; t.updated_at = new Date().toISOString(); return { ...t }; }
      }
      throw err;
    }
  },
};

// ─── Discounts API ────────────────────────────────────────────────────────────

let mockDiscounts = [...MOCK_DISCOUNTS];

export const discountsApi = {
  list: async (params?: { search?: string; active?: boolean; page?: number; per_page?: number }) => {
    try {
      return await api.get<Discount[]>('/discounts', params as Record<string, string>);
    } catch (err) {
      if (isFallbackError(err)) {
        let result = [...mockDiscounts];
        if (params?.search) { const q = params.search.toLowerCase(); result = result.filter((d) => d.name.toLowerCase().includes(q)); }
        if (params?.active) result = result.filter((d) => d.is_active);
        return { data: result, meta: { request_id: 'mock', page: 1, per_page: 25, total: result.length, total_pages: 1 } };
      }
      throw err;
    }
  },

  create: async (input: DiscountCreateInput): Promise<Discount> => {
    try {
      const res = await api.post<Discount>('/discounts', input);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const d: Discount = { id: `disc-${Date.now().toString(36)}`, name: input.name, kind: input.kind, value_cents: input.value_cents ?? 0, value_bp: input.value_bp ?? 0, scope: input.scope ?? 'SUBSCRIPTION', product_id: input.product_id ?? null, min_purchase_cents: input.min_purchase_cents ?? 0, min_quantity: input.min_quantity ?? 0, starts_at: input.starts_at ?? null, ends_at: input.ends_at ?? null, max_uses: input.max_uses ?? null, used_count: 0, is_active: true, created_at: new Date().toISOString(), updated_at: new Date().toISOString() };
        mockDiscounts.unshift(d);
        return d;
      }
      throw err;
    }
  },

  update: async (id: string, input: DiscountUpdateInput): Promise<Discount> => {
    try {
      const res = await api.patch<Discount>(`/discounts/${id}`, input);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const d = mockDiscounts.find((x) => x.id === id);
        if (d) { Object.assign(d, input); d.updated_at = new Date().toISOString(); return { ...d }; }
      }
      throw err;
    }
  },

  deactivate: async (id: string): Promise<Discount> => {
    try {
      const res = await api.del<Discount>(`/discounts/${id}`);
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) {
        const d = mockDiscounts.find((x) => x.id === id);
        if (d) { d.is_active = false; d.updated_at = new Date().toISOString(); return { ...d }; }
      }
      throw err;
    }
  },
};

// ─── Reports API ──────────────────────────────────────────────────────────────

export const reportsApi = {
  summary: async (): Promise<ReportSummary> => {
    try {
      const res = await api.get<ReportSummary>('/reports/summary');
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) return { ...MOCK_SUMMARY };
      throw err;
    }
  },

  revenue: async (months = 6): Promise<RevenuePoint[]> => {
    try {
      const res = await api.get<RevenuePoint[]>('/reports/revenue', { months: String(months) });
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) return [...MOCK_REVENUE];
      throw err;
    }
  },

  subscriptionsByStatus: async (): Promise<StatusCount[]> => {
    try {
      const res = await api.get<StatusCount[]>('/reports/subscriptions');
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) return [...MOCK_STATUS_COUNTS];
      throw err;
    }
  },

  overdue: async (): Promise<OverdueInvoice[]> => {
    try {
      const res = await api.get<OverdueInvoice[]>('/reports/overdue');
      return res.data;
    } catch (err) {
      if (isFallbackError(err)) return [...MOCK_OVERDUE];
      throw err;
    }
  },
};
