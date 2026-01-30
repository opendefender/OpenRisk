import { create } from 'zustand';
import { useAuthStore } from './useAuthStore';

export interface Report {
  id: string;
  title: string;
  type: 'executive' | 'technical' | 'compliance' | 'incident';
  format: 'PDF' | 'XLSX' | 'DOCX';
  created_at: string;
  generated_by: string;
  status: 'completed' | 'generating' | 'scheduled';
  size: string;
}

interface ReportStats {
  total_reports: number;
  completed: number;
  generating: number;
  scheduled: number;
}

interface ReportStore {
  reports: Report[];
  stats: ReportStats | null;
  total: number;
  page: number;
  pageSize: number;
  isLoading: boolean;
  error: string | null;
  fetchReports: (params?: { page?: number; limit?: number; type?: string; status?: string }) => Promise<void>;
  fetchReportStats: () => Promise<void>;
  getReport: (id: string) => Promise<Report | null>;
}

export const useReportStore = create<ReportStore>((set) => {
  const apiBaseUrl = import.meta.env.VITE_API_URL || 'http://localhost:/api/v';

  return {
    reports: [],
    stats: null,
    total: ,
    page: ,
    pageSize: ,
    isLoading: false,
    error: null,

    fetchReports: async (params = {}) => {
      set({ isLoading: true, error: null });
      try {
        const token = useAuthStore.getState().token;
        const page = params.page ?? ;
        const limit = params.limit ?? ;
        
        let url = ${apiBaseUrl}/reports?page=${page}&limit=${limit};
        if (params.type) url += &type=${params.type};
        if (params.status) url += &status=${params.status};

        const response = await fetch(url, {
          headers: {
            Authorization: Bearer ${token},
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          throw new Error(HTTP ${response.status});
        }

        const data = await response.json();
        set({
          reports: data.reports || [],
          total: data.total || ,
          page: data.page || ,
          pageSize: data.limit || ,
          isLoading: false,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to fetch reports';
        set({ error: message, isLoading: false });
      }
    },

    fetchReportStats: async () => {
      try {
        const token = useAuthStore.getState().token;
        const response = await fetch(${apiBaseUrl}/reports/stats, {
          headers: {
            Authorization: Bearer ${token},
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          throw new Error(HTTP ${response.status});
        }

        const stats = await response.json();
        set({ stats });
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to fetch report stats';
        set({ error: message });
      }
    },

    getReport: async (id: string) => {
      try {
        const token = useAuthStore.getState().token;
        const response = await fetch(${apiBaseUrl}/reports/${id}, {
          headers: {
            Authorization: Bearer ${token},
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          throw new Error(HTTP ${response.status});
        }

        return await response.json();
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to fetch report';
        set({ error: message });
        return null;
      }
    },
  };
});
