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
}

export interface Product {
  id: string;
  name: string;
  type: string;
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
  name: string;
  billing_interval: BillingInterval;
  price_cents: number;
  min_quantity: number;
  available_from: string | null;
  available_until: string | null;
  auto_close: boolean;
  allow_early_close: boolean;
  allow_pause: boolean;
  allow_renewal: boolean;
  status: 'ACTIVE' | 'ARCHIVED';
  created_at?: string;
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
  product_id: string;
  product_name: string;
  variant_id?: string | null;
  variant_name?: string | null;
  quantity: number;
  unit_price_cents: number;
  tax_rate_bp: number;
  tax_cents: number;
  discount_cents: number;
  total_cents: number;
}

export interface Subscription {
  id: string;
  subscription_number: string;
  customer_id: string;
  customer_name?: string;
  plan_id: string;
  plan_name?: string;
  status: SubscriptionStatus;
  start_date: string;
  expiry_date: string | null;
  payment_terms: string;
  subtotal_cents: number;
  tax_cents: number;
  total_cents: number;
  lines: SubscriptionLine[];
  created_at: string;
  updated_at: string;
}

export interface QuotationTemplate {
  id: string;
  name: string;
  plan_id: string;
  plan_name?: string;
  validity_days: number;
  lines: {
    product_id: string;
    variant_id?: string | null;
    quantity: number;
  }[];
}
