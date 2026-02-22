/**
 * Dashboard API Hooks
 * Custom React hooks for fetching dashboard data from backend APIs
 */

import { useState, useEffect, useCallback } from 'react';
import {
  DashboardMetrics,
  RiskTrendDataPoint,
  RiskSeverityDistribution,
  MitigationStatus,
  TopRisk,
  MitigationProgress,
  CompleteDashboardAnalytics,
} from '../types/dashboard.types';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';

interface UseDataState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

/**
 * Generic hook for fetching data from API endpoints
 */
function useApiData<T>(
  url: string,
  options?: RequestInit
): UseDataState<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const response = await fetch(url, {
        headers: {
          'Content-Type': 'application/json',
          ...options?.headers,
        },
        ...options,
      });

      if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`);
      }

      const result = await response.json();
      setData(result);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setError(message);
      console.error(`Error fetching ${url}:`, err);
    } finally {
      setLoading(false);
    }
  }, [url, options]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { data, loading, error, refetch: fetchData };
}

/**
 * Hook for fetching dashboard KPI metrics
 */
export function useDashboardMetrics(): UseDataState<DashboardMetrics> {
  return useApiData<DashboardMetrics>(`${API_BASE_URL}/dashboard/metrics`);
}

/**
 * Hook for fetching 7-day risk trends
 */
export function useRiskTrends(): UseDataState<RiskTrendDataPoint[]> {
  const result = useApiData<{ trends: RiskTrendDataPoint[] }>(
    `${API_BASE_URL}/dashboard/risk-trends`
  );

  return {
    data: result.data?.trends || null,
    loading: result.loading,
    error: result.error,
    refetch: result.refetch,
  };
}

/**
 * Hook for fetching risk severity distribution
 */
export function useSeverityDistribution(): UseDataState<RiskSeverityDistribution> {
  return useApiData<RiskSeverityDistribution>(
    `${API_BASE_URL}/dashboard/severity-distribution`
  );
}

/**
 * Hook for fetching mitigation status summary
 */
export function useMitigationStatus(): UseDataState<MitigationStatus> {
  return useApiData<MitigationStatus>(
    `${API_BASE_URL}/dashboard/mitigation-status`
  );
}

/**
 * Hook for fetching top risks by score
 * @param limit Number of top risks to fetch (default: 5, max: 50)
 */
export function useTopRisks(limit: number = 5): UseDataState<TopRisk[]> {
  const result = useApiData<{ top_risks: TopRisk[]; count: number }>(
    `${API_BASE_URL}/dashboard/top-risks?limit=${Math.min(limit, 50)}`
  );

  return {
    data: result.data?.top_risks || null,
    loading: result.loading,
    error: result.error,
    refetch: result.refetch,
  };
}

/**
 * Hook for fetching mitigation progress tracking
 * @param limit Number of mitigations to fetch (default: 10, max: 100)
 */
export function useMitigationProgress(limit: number = 10): UseDataState<MitigationProgress[]> {
  const result = useApiData<{ mitigations: MitigationProgress[]; count: number }>(
    `${API_BASE_URL}/dashboard/mitigation-progress?limit=${Math.min(limit, 100)}`
  );

  return {
    data: result.data?.mitigations || null,
    loading: result.loading,
    error: result.error,
    refetch: result.refetch,
  };
}

/**
 * Hook for fetching complete dashboard data in one request
 * More efficient for initial page load
 */
export function useCompleteDashboard(): UseDataState<CompleteDashboardAnalytics> {
  return useApiData<CompleteDashboardAnalytics>(
    `${API_BASE_URL}/dashboard/complete`
  );
}

/**
 * Hook for polling dashboard data at regular intervals
 * Useful for keeping dashboard fresh without WebSocket
 */
export function useDashboardPoller(
  interval: number = 30000 // Default: 30 seconds
): UseDataState<CompleteDashboardAnalytics> {
  const state = useCompleteDashboard();

  useEffect(() => {
    if (interval <= 0) return;

    const timer = setInterval(() => {
      state.refetch();
    }, interval);

    return () => clearInterval(timer);
  }, [interval, state]);

  return state;
}

/**
 * Hook for manual refresh with debounce
 * Prevents excessive API calls on rapid refresh clicks
 */
export function useRefreshWithDebounce(delay: number = 1000) {
  const [lastRefresh, setLastRefresh] = useState(0);
  const state = useCompleteDashboard();

  const debouncedRefresh = useCallback(() => {
    const now = Date.now();
    if (now - lastRefresh >= delay) {
      setLastRefresh(now);
      state.refetch();
    }
  }, [lastRefresh, delay, state]);

  return {
    ...state,
    refresh: debouncedRefresh,
  };
}
