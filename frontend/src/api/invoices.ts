import { api, isOfflineError } from './client';
import { Invoice, InvoiceStatus } from './types';

const INITIAL_INVOICES: Invoice[] = [
  {
    id: 'inv-001',
    number: 'INV-2026-0001',
    customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd',
    subscription_id: 'sub-001',
    subscription_number: 'SUB-2026-0001',
    status: 'PAID',
    issue_date: '2026-03-01T00:00:00Z',
    due_date: '2026-03-31T00:00:00Z',
    period_start: '2026-03-01T00:00:00Z',
    period_end: '2026-03-31T00:00:00Z',
    subtotal_cents: 176000,
    discount_cents: 0,
    tax_cents: 31680,
    total_cents: 207680,
    balance_cents: 0,
    notes: 'March 2026 Growth Billing',
    confirmed_at: '2026-03-01T00:05:00Z',
    sent_at: '2026-03-01T00:06:00Z',
    paid_at: '2026-03-10T14:30:00Z',
    lines: [
      {
        id: 'invline-1',
        invoice_id: 'inv-001',
        product_id: 'prod-001',
        description: 'BrightERP Core Platform with Odoo brand variant',
        quantity: 1,
        unit_price_cents: 176000,
        tax_rate_bp: 1800,
        tax_cents: 31680,
        discount_cents: 0,
        line_total_cents: 207680,
        position: 1,
      },
    ],
    created_at: '2026-03-01T00:00:00Z',
    updated_at: '2026-03-10T14:30:00Z',
  },
  {
    id: 'inv-002',
    number: 'INV-2026-0002',
    customer_id: 'cust-demo-1',
    customer_name: 'Apex Retailers Ltd',
    subscription_id: 'sub-001',
    subscription_number: 'SUB-2026-0001',
    status: 'CONFIRMED',
    issue_date: '2026-04-01T00:00:00Z',
    due_date: '2026-05-01T00:00:00Z',
    period_start: '2026-04-01T00:00:00Z',
    period_end: '2026-04-30T00:00:00Z',
    subtotal_cents: 176000,
    discount_cents: 0,
    tax_cents: 31680,
    total_cents: 207680,
    balance_cents: 207680, // Unpaid
    notes: 'April 2026 Recurring Billing',
    confirmed_at: '2026-04-01T00:05:00Z',
    sent_at: '2026-04-01T00:06:00Z',
    lines: [
      {
        id: 'invline-2',
        invoice_id: 'inv-002',
        product_id: 'prod-001',
        description: 'BrightERP Core Platform with Odoo brand variant',
        quantity: 1,
        unit_price_cents: 176000,
        tax_rate_bp: 1800,
        tax_cents: 31680,
        discount_cents: 0,
        line_total_cents: 207680,
        position: 1,
      },
    ],
    created_at: '2026-04-01T00:00:00Z',
    updated_at: '2026-04-01T00:05:00Z',
  },
  {
    id: 'inv-003',
    number: 'INV-2026-0003',
    customer_id: 'cust-demo-2',
    customer_name: 'Acme Logistics Corp',
    subscription_id: 'sub-002',
    subscription_number: 'SUB-2026-0002',
    status: 'DRAFT',
    issue_date: '2026-04-15T00:00:00Z',
    due_date: '2026-04-30T00:00:00Z',
    subtotal_cents: 6000000,
    discount_cents: 600000, // 10% discount
    tax_cents: 972000,
    total_cents: 6372000,
    balance_cents: 6372000,
    notes: 'Draft enterprise quotation snapshot',
    lines: [
      {
        id: 'invline-3',
        invoice_id: 'inv-003',
        product_id: 'prod-001',
        description: 'BrightERP Enterprise Annual 5 Seats',
        quantity: 5,
        unit_price_cents: 1200000,
        tax_rate_bp: 1800,
        tax_cents: 972000,
        discount_cents: 600000,
        line_total_cents: 6372000,
        position: 1,
      },
    ],
    created_at: '2026-04-15T00:00:00Z',
    updated_at: '2026-04-15T00:00:00Z',
  },
];

let mockInvoices = [...INITIAL_INVOICES];

export const invoicesApi = {
  list: async (params?: {
    customer_id?: string;
    subscription_id?: string;
    status?: string;
    search?: string;
    page?: number;
    per_page?: number;
  }) => {
    try {
      return await api.get<Invoice[]>('/invoices', params);
    } catch (err) {
      if (isOfflineError(err)) {
        let filtered = [...mockInvoices];
        if (params?.customer_id) {
          filtered = filtered.filter((i) => i.customer_id === params.customer_id);
        }
        if (params?.subscription_id) {
          filtered = filtered.filter((i) => i.subscription_id === params.subscription_id);
        }
        if (params?.status) {
          filtered = filtered.filter((i) => i.status === params.status);
        }
        if (params?.search) {
          const q = params.search.toLowerCase();
          filtered = filtered.filter(
            (i) =>
              i.number.toLowerCase().includes(q) ||
              (i.customer_name && i.customer_name.toLowerCase().includes(q))
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

  get: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.get<Invoice>(`/invoices/${id}`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const found = mockInvoices.find((i) => i.id === id);
        if (found) return found;
      }
      throw err;
    }
  },

  create: async (data: {
    customer_id: string;
    subscription_id?: string | null;
    issue_date?: string;
    due_date?: string;
    period_start?: string | null;
    period_end?: string | null;
    discount_id?: string | null;
    notes?: string;
    customer_name?: string;
    lines?: {
      product_id?: string;
      variant_id?: string;
      description: string;
      quantity: number;
      unit_price_cents: number;
      tax_id?: string;
    }[];
  }): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>('/invoices', data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const id = `inv-${Date.now().toString(36)}`;
        const num = `INV-2026-${Math.floor(1000 + Math.random() * 9000)}`;
        const lines = (data.lines || []).map((l, i) => {
          const subtotal = l.quantity * l.unit_price_cents;
          const tax = Math.round((subtotal * 1800) / 10000);
          return {
            id: `invl-${id}-${i}`,
            invoice_id: id,
            product_id: l.product_id || null,
            variant_id: l.variant_id || null,
            description: l.description,
            quantity: l.quantity,
            unit_price_cents: l.unit_price_cents,
            tax_rate_bp: 1800,
            tax_cents: tax,
            discount_cents: 0,
            line_total_cents: subtotal + tax,
            position: i + 1,
          };
        });
        const subtotal = lines.reduce((acc, l) => acc + l.quantity * l.unit_price_cents, 0);
        const tax = lines.reduce((acc, l) => acc + l.tax_cents, 0);
        const total = subtotal + tax;

        const inv: Invoice = {
          id,
          number: num,
          customer_id: data.customer_id,
          customer_name: data.customer_name || 'Customer Profile',
          subscription_id: data.subscription_id || null,
          status: 'DRAFT',
          issue_date: data.issue_date || new Date().toISOString(),
          due_date: data.due_date || new Date(Date.now() + 30 * 86400000).toISOString(),
          period_start: data.period_start || null,
          period_end: data.period_end || null,
          subtotal_cents: subtotal,
          discount_cents: 0,
          tax_cents: tax,
          total_cents: total,
          balance_cents: total,
          notes: data.notes || null,
          lines,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        mockInvoices.unshift(inv);
        return inv;
      }
      throw err;
    }
  },

  update: async (id: string, data: Partial<Invoice>): Promise<Invoice> => {
    try {
      const res = await api.patch<Invoice>(`/invoices/${id}`, data);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) {
          Object.assign(inv, data, { updated_at: new Date().toISOString() });
          return inv;
        }
      }
      throw err;
    }
  },

  confirm: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>(`/invoices/${id}/confirm`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) {
          inv.status = 'CONFIRMED';
          inv.confirmed_at = new Date().toISOString();
          inv.balance_cents = inv.total_cents;
          inv.updated_at = new Date().toISOString();
          return { ...inv };
        }
      }
      throw err;
    }
  },

  cancel: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>(`/invoices/${id}/cancel`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) {
          inv.status = 'CANCELLED';
          inv.cancelled_at = new Date().toISOString();
          inv.updated_at = new Date().toISOString();
          return { ...inv };
        }
      }
      throw err;
    }
  },

  send: async (id: string): Promise<Invoice> => {
    try {
      const res = await api.post<Invoice>(`/invoices/${id}/send`);
      return res.data;
    } catch (err) {
      if (isOfflineError(err)) {
        const inv = mockInvoices.find((i) => i.id === id);
        if (inv) {
          inv.sent_at = new Date().toISOString();
          inv.updated_at = new Date().toISOString();
          return { ...inv };
        }
      }
      throw err;
    }
  },

  applyPaymentReduction: (invoiceId: string, amountCents: number): Invoice | null => {
    const inv = mockInvoices.find((i) => i.id === invoiceId);
    if (!inv) return null;
    inv.balance_cents = Math.max(0, inv.balance_cents - amountCents);
    if (inv.balance_cents === 0) {
      inv.status = 'PAID';
      inv.paid_at = new Date().toISOString();
    }
    inv.updated_at = new Date().toISOString();
    return { ...inv };
  },
};
