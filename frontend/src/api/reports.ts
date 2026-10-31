import { api } from './client';
import { Invoice, ReportSummary, RevenuePoint } from './types';

export const reportsApi = {
  getSummary: async (): Promise<ReportSummary> => {
    const res = await api.get<ReportSummary>('/reports/summary');
    return res.data;
  },

  getRevenue: async (months = 12): Promise<RevenuePoint[]> => {
    const res = await api.get<RevenuePoint[]>('/reports/revenue', { months });
    return res.data;
  },

  getOverdue: async (): Promise<Invoice[]> => {
    const res = await api.get<Invoice[]>('/reports/overdue');
    return res.data;
  },
};
