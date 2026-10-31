import { api, ApiException, isOfflineError } from './client';
import { User } from './types';

const DEMO_STORAGE_KEY = 'bl_demo_user';

export const authApi = {
  login: async (email: string, password: string): Promise<User> => {
    try {
      const res = await api.post<User>('/auth/login', { email, password });
      return res.data;
    } catch (err: any) {
      if (isOfflineError(err)) {
        const lowerEmail = email.toLowerCase();
        let mockUser: User;

        if (lowerEmail.includes('admin')) {
          mockUser = {
            id: 'usr-admin-001',
            email: email,
            full_name: 'Brightloop Admin',
            role: 'ADMIN',
            status: 'ACTIVE',
            customer_id: null,
            email_verified: true,
            must_change_password: false,
            permissions: [
              'users.read', 'users.write', 'audit.read',
              'customers.read', 'customers.write',
              'products.read', 'products.write',
              'plans.read', 'plans.write',
              'taxes.read', 'taxes.write',
              'discounts.read', 'discounts.write',
              'subscriptions.read', 'subscriptions.write',
              'invoices.read', 'invoices.write',
              'payments.read', 'payments.write',
              'reports.read',
            ],
          };
        } else if (lowerEmail.includes('finance')) {
          mockUser = {
            id: 'usr-finance-001',
            email: email,
            full_name: 'Finance Operations Lead',
            role: 'STAFF',
            status: 'ACTIVE',
            customer_id: null,
            email_verified: true,
            must_change_password: false,
            permissions: [
              'customers.read', 'customers.write',
              'products.read', 'plans.read',
              'taxes.read', 'discounts.read',
              'subscriptions.read', 'subscriptions.write',
              'invoices.read', 'invoices.write',
              'payments.read', 'payments.write',
              'reports.read',
            ],
          };
        } else {
          // Customer portal account
          mockUser = {
            id: 'usr-cust-001',
            email: email,
            full_name: 'Client Portal User',
            role: 'CUSTOMER',
            status: 'ACTIVE',
            customer_id: 'cust-demo-1',
            email_verified: false, // demonstrates email verification banner
            must_change_password: false,
            permissions: [],
          };
        }

        localStorage.setItem(DEMO_STORAGE_KEY, JSON.stringify(mockUser));
        return mockUser;
      }
      throw err;
    }
  },

  register: async (data: { email: string; password: string; full_name: string; company_name: string }): Promise<User> => {
    try {
      const res = await api.post<User>('/auth/register', data);
      return res.data;
    } catch (err: any) {
      if (isOfflineError(err)) {
        const mockUser: User = {
          id: `usr-${Date.now().toString(36)}`,
          email: data.email,
          full_name: data.full_name,
          role: 'CUSTOMER',
          status: 'ACTIVE',
          customer_id: `cust-${Date.now().toString(36)}`,
          email_verified: false,
          must_change_password: false,
          permissions: [],
        };
        localStorage.setItem(DEMO_STORAGE_KEY, JSON.stringify(mockUser));
        return mockUser;
      }
      throw err;
    }
  },

  logout: async () => {
    try {
      await api.post<void>('/auth/logout');
    } catch {
      // ignore
    } finally {
      localStorage.removeItem(DEMO_STORAGE_KEY);
    }
  },

  getMe: async (): Promise<User> => {
    try {
      const res = await api.get<User>('/auth/me');
      return res.data;
    } catch (err: any) {
      if (isOfflineError(err)) {
        const stored = localStorage.getItem(DEMO_STORAGE_KEY);
        if (stored) {
          try {
            return JSON.parse(stored);
          } catch {
            // fallthrough
          }
        }
        throw new ApiException(401, {
          code: 'UNAUTHENTICATED',
          message: 'No active session',
        });
      }
      throw err;
    }
  },

  forgotPassword: async (email: string) => {
    try {
      await api.post<void>('/auth/forgot-password', { email });
    } catch (err: any) {
      if (isOfflineError(err)) {
        return;
      }
      throw err;
    }
  },

  resetPassword: async (token: string, password: string) => {
    await api.post<void>('/auth/reset-password', { token, password });
  },

  verifyEmail: async (token: string) => {
    await api.post<void>('/auth/verify-email', { token });
  },

  changePassword: async (currentPassword: string, newPassword: string) => {
    try {
      await api.post<void>('/auth/change-password', {
        current_password: currentPassword,
        new_password: newPassword,
      });
    } catch (err: any) {
      if (isOfflineError(err)) {
        const stored = localStorage.getItem(DEMO_STORAGE_KEY);
        if (stored) {
          const user = JSON.parse(stored);
          user.must_change_password = false;
          localStorage.setItem(DEMO_STORAGE_KEY, JSON.stringify(user));
        }
        return;
      }
      throw err;
    }
  },

  resendVerification: async () => {
    try {
      await api.post<void>('/auth/resend-verification');
    } catch (err: any) {
      if (isOfflineError(err)) {
        return;
      }
      throw err;
    }
  },
};
