import { create } from 'zustand';
import { useAuthStore } from './useAuthStore';

export interface Threat {
  id: string;
  country: string;
  code: string;
  threats: number;
  severity: 'critical' | 'high' | 'medium' | 'low';
  lat: number;
  lon: number;
}

interface ThreatStats {
  total_threats: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  trend_percent: number;
}

interface ThreatStore {
  threats: Threat[];
  stats: ThreatStats | null;
  isLoading: boolean;
  error: string | null;
  fetchThreats: (params?: { severity?: string; country?: string }) => Promise<void>;
  fetchThreatStats: () => Promise<void>;
}

export const useThreatStore = create<ThreatStore>((set) => {
  const apiBaseUrl = import.meta.env.VITE_API_URL || 'http://localhost:/api/v';

  return {
    threats: [],
    stats: null,
    isLoading: false,
    error: null,

    fetchThreats: async (params = {}) => {
      set({ isLoading: true, error: null });
      try {
        const token = useAuthStore.getState().token;
        
        let url = ${apiBaseUrl}/threats;
        const queryParams = new URLSearchParams();
        if (params.severity) queryParams.append('severity', params.severity);
        if (params.country) queryParams.append('country', params.country);
        
        if (queryParams.toString()) {
          url += ?${queryParams.toString()};
        }

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
          threats: data.threats || [],
          isLoading: false,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to fetch threats';
        set({ error: message, isLoading: false });
      }
    },

    fetchThreatStats: async () => {
      try {
        const token = useAuthStore.getState().token;
        const response = await fetch(${apiBaseUrl}/threats/stats, {
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
        const message = error instanceof Error ? error.message : 'Failed to fetch threat stats';
        set({ error: message });
      }
    },
  };
});
