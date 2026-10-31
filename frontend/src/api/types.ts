export interface ApiResponse<T> {
  data: T;
  meta?: {
    request_id: string;
    page?: number;
    per_page?: number;
    total?: number;
    total_pages?: number;
  };
}

export interface ApiError {
  error: {
    code: string;
    message: string;
    details?: Record<string, string>;
    request_id?: string;
  };
}

export interface User {
  id: string;
  email: string;
  full_name: string;
  role: 'ADMIN' | 'STAFF' | 'CUSTOMER';
  status: 'ACTIVE' | 'DISABLED';
  customer_id: string | null;
  email_verified: boolean;
  must_change_password: boolean;
  permissions?: string[];
  created_at?: string;
}

export interface CustomerAddress {
  street?: string;
  city?: string;
  state?: string;
  postal_code?: string;
  country?: string;
}

export interface Customer {
  id: string;
  name: string;
  email: string | null;
  phone: string | null;
  billing_address: CustomerAddress;
  tax_id: string | null;
  status: 'ACTIVE' | 'INACTIVE';
  notes: string | null;
  created_at: string;
  updated_at: string;
}

export interface CustomerCreateInput {
  name: string;
  email?: string;
  phone?: string;
  billing_address?: CustomerAddress;
  tax_id?: string;
  notes?: string;
}

export interface CustomerUpdateInput {
  name?: string;
  email?: string;
  phone?: string;
  billing_address?: CustomerAddress;
  tax_id?: string;
  notes?: string;
  status?: 'ACTIVE' | 'INACTIVE';
}

export interface ProductVariant {
  id: string;
  product_id?: string;
  name: string;
  price_delta_cents: number;
  is_active?: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface Product {
  id: string;
  name: string;
  type: 'SERVICE' | 'GOODS' | string;
  sales_price_cents: number;
  cost_price_cents: number;
  notes: string | null;
  is_active: boolean;
  variants: ProductVariant[];
  created_at: string;
  updated_at: string;
}

export type BillingInterval = 'DAILY' | 'WEEKLY' | 'MONTHLY' | 'YEARLY';

export interface Plan {
  id: string;
  product_id: string;
  product_name?: string;
  tax_id?: string | null;
  tax_name?: string;
  name: string;
  interval: BillingInterval;
  billing_interval?: BillingInterval; // Alias for backward compatibility
  price_cents: number;
  minimum_quantity: number;
  min_quantity?: number; // Alias for backward compatibility
  available_from: string | null;
  available_to: string | null;
  available_until?: string | null; // Alias
  auto_close: boolean;
  auto_renew: boolean;
  allow_renewal?: boolean; // Alias
  allow_pause: boolean;
  allow_early_close: boolean;
  is_active: boolean;
  status?: 'ACTIVE' | 'ARCHIVED'; // Alias
  created_at?: string;
  updated_at?: string;
}

export interface Tax {
  id: string;
  name: string;
  rate_bp: number; // Basis points (1800 = 18%)
  is_inclusive: boolean;
  country_code?: string | null;
  is_active: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface Discount {
  id: string;
  name: string;
  kind: 'PERCENT' | 'FIXED';
  value_cents: number;
  value_bp: number;
  scope: 'ORDER' | 'PRODUCT';
  product_id?: string | null;
  product_name?: string;
  min_purchase_cents: number;
  min_quantity: number;
  starts_at: string | null;
  ends_at: string | null;
  max_uses: number | null;
  used_count: number;
  is_active: boolean;
  created_at?: string;
  updated_at?: string;
}

export type SubscriptionStatus =
  | 'DRAFT'
  | 'QUOTATION'
  | 'CONFIRMED'
  | 'ACTIVE'
  | 'PAUSED'
  | 'CLOSED';

export interface SubscriptionLine {
  id: string;
  subscription_id?: string;
  product_id: string;
  product_name?: string;
  variant_id?: string | null;
  variant_name?: string | null;
  description?: string;
  quantity: number;
  unit_price_cents: number;
  tax_id?: string | null;
  tax_rate_bp?: number;
  tax_cents?: number;
  discount_cents?: number;
  total_cents?: number;
  position?: number;
}

export interface Subscription {
  id: string;
  number: string;
  subscription_number?: string; // Alias
  customer_id: string;
  customer_name?: string;
  plan_id: string;
  plan_name?: string;
  status: SubscriptionStatus;
  start_date: string;
  expiry_date: string | null;
  payment_terms_days: number;
  payment_terms?: string; // NET_30 / NET_15 string helper
  quantity: number;
  discount_id?: string | null;
  next_billing_date?: string | null;
  quote_expires_at?: string | null;
  notes?: string | null;
  subtotal_cents?: number;
  tax_cents?: number;
  total_cents?: number;
  lines: SubscriptionLine[];
  activated_at?: string | null;
  paused_at?: string | null;
  closed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface QuotationTemplateLine {
  id?: string;
  template_id?: string;
  plan_id: string;
  plan_name?: string;
  quantity: number;
  position?: number;
}

export interface QuotationTemplate {
  id: string;
  name: string;
  validity_days: number;
  notes?: string | null;
  is_active: boolean;
  lines: QuotationTemplateLine[];
  created_at?: string;
  updated_at?: string;
}

export type InvoiceStatus = 'DRAFT' | 'CONFIRMED' | 'PAID' | 'CANCELLED';

export interface InvoiceLine {
  id: string;
  invoice_id: string;
  product_id?: string | null;
  variant_id?: string | null;
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
  subscription_id?: string | null;
  subscription_number?: string | null;
  status: InvoiceStatus;
  issue_date: string;
  due_date: string;
  period_start?: string | null;
  period_end?: string | null;
  subtotal_cents: number;
  discount_cents: number;
  tax_cents: number;
  total_cents: number;
  balance_cents: number;
  notes?: string | null;
  confirmed_at?: string | null;
  sent_at?: string | null;
  paid_at?: string | null;
  cancelled_at?: string | null;
  lines: InvoiceLine[];
  created_at?: string;
  updated_at?: string;
}

export type PaymentMethod = 'CASH' | 'CARD' | 'BANK_TRANSFER' | 'MOBILE_MONEY' | 'OTHER';

export interface Payment {
  id: string;
  invoice_id: string;
  invoice_number?: string;
  customer_name?: string;
  method: PaymentMethod;
  amount_cents: number;
  paid_at: string;
  note?: string | null;
  recorded_by?: string | null;
  created_at: string;
}

export interface BillingRunResult {
  as_of: string;
  billed: number;
  skipped: number;
  closed: number;
  failed: number;
  invoices: string[];
  results: {
    subscription_id: string;
    period_start: string;
    period_end: string;
    invoice_id: string;
    status: string;
  }[];
}

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

export interface PortalMe {
  customer_id: string;
  email: string;
  full_name: string;
  subscriptions: number;
  active_status: number;
  invoices: number;
  unpaid_invoices: number;
}
