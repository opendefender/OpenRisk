import { create } from 'zustand';
import { useAuthStore } from './useAuthStore';

export interface Incident {
  id: string;
  title: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  status: 'open' | 'investigating' | 'resolved';
  date: string;
  assignee: string;
  description: string;
  source?: string;
  external_id?: string;
}

interface IncidentStore {
  incidents: Incident[];
  total: number;
  page: number;
  pageSize: number;
  isLoading: boolean;
  error: string | null;
  fetchIncidents: (params?: { page?: number; limit?: number; severity?: string; status?: string }) => Promise<void>;
  getIncident: (id: string) => Promise<Incident | null>;
}

export const useIncidentStore = create<IncidentStore>((set) => {
  const apiBaseUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

  return {
    incidents: [],
    total: 0,
    page: 1,
    pageSize: 10,
    isLoading: false,
    error: null,

    fetchIncidents: async (params = {}) => {
      set({ isLoading: true, error: null });
      try {
        const token = useAuthStore.getState().token;
        const page = params.page ?? 1;
        const limit = params.limit ?? 10;
        
        let url = `${apiBaseUrl}/incidents?page=${page}&limit=${limit}`;
        if (params.severity) url += `&severity=${params.severity}`;
        if (params.status) url += `&status=${params.status}`;

        const response = await fetch(url, {
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();
        set({
          incidents: data.incidents || [],
          total: data.total || 0,
          page: data.page || 1,
          pageSize: data.limit || 10,
          isLoading: false,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to fetch incidents';
        set({ error: message, isLoading: false });
      }
    },

    getIncident: async (id: string) => {
      try {
        const token = useAuthStore.getState().token;
        const response = await fetch(`${apiBaseUrl}/incidents/${id}`, {
          headers: {
            Authorization: `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        return await response.json();
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to fetch incident';
        set({ error: message });
        return null;
      }
    },
  };
});
