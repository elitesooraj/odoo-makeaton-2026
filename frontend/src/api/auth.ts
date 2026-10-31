import { api } from './client';
import { User } from './types';

export const authApi = {
  login: async (email: string, password: string): Promise<User> => {
    const res = await api.post<User>('/auth/login', { email, password });
    return res.data;
  },

  register: async (data: {
    email: string;
    password: string;
    full_name: string;
    company_name: string;
  }): Promise<User> => {
    const res = await api.post<User>('/auth/register', data);
    return res.data;
  },

  logout: async (): Promise<void> => {
    await api.post<void>('/auth/logout');
  },

  getMe: async (): Promise<User> => {
    const res = await api.get<User>('/auth/me');
    return res.data;
  },

  forgotPassword: async (email: string): Promise<void> => {
    await api.post<void>('/auth/forgot-password', { email });
  },

  resetPassword: async (token: string, password: string): Promise<void> => {
    await api.post<void>('/auth/reset-password', { token, password });
  },

  verifyEmail: async (token: string): Promise<void> => {
    await api.post<void>('/auth/verify-email', { token });
  },

  changePassword: async (currentPassword: string, newPassword: string): Promise<void> => {
    await api.post<void>('/auth/change-password', {
      current_password: currentPassword,
      new_password: newPassword,
    });
  },

  resendVerification: async (): Promise<void> => {
    await api.post<void>('/auth/resend-verification');
  },
};
