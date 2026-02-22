/**
 * WebSocket Dashboard Hook
 * Manages WebSocket connections for real-time dashboard updates
 */

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  CompleteDashboardAnalytics,
  DashboardMetrics,
  RiskTrendDataPoint,
  RiskSeverityDistribution,
  MitigationStatus,
  TopRisk,
  MitigationProgress,
} from '../types/dashboard.types';

interface WebSocketMessage {
  type: 'dashboard_update' | 'error' | 'pong' | 'connected';
  data?: CompleteDashboardAnalytics;
  error?: string;
}

interface UseWebSocketState {
  data: CompleteDashboardAnalytics | null;
  loading: boolean;
  error: string | null;
  connected: boolean;
  reconnecting: boolean;
  clientCount?: number;
  refresh: () => void;
}

interface WebSocketConfig {
  url?: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  heartbeatInterval?: number;
  fallbackToPoll?: boolean;
  pollInterval?: number;
}

const DEFAULT_CONFIG: Required<WebSocketConfig> = {
  url: `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/v1/ws/dashboard`,
  reconnectInterval: 1000,
  maxReconnectAttempts: 5,
  heartbeatInterval: 30000,
  fallbackToPoll: true,
  pollInterval: 30000,
};

/**
 * Hook for real-time dashboard updates via WebSocket
 * Falls back to polling if WebSocket is unavailable
 */
export function useWebSocketDashboard(config: WebSocketConfig = {}): UseWebSocketState {
  const mergedConfig = { ...DEFAULT_CONFIG, ...config };

  const [data, setData] = useState<CompleteDashboardAnalytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [clientCount, setClientCount] = useState<number | undefined>();

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const heartbeatTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const pollTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const isMountedRef = useRef(true);

  // Fallback to polling
  const startPolling = useCallback(() => {
    if (!mergedConfig.fallbackToPoll) return;

    const poll = async () => {
      try {
        const response = await fetch('/api/v1/dashboard/complete', {
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('auth_token')}`,
          },
        });

        if (!response.ok) throw new Error('Poll failed');

        const result = await response.json();
        if (isMountedRef.current) {
          setData(result);
          setError(null);
        }
      } catch (err) {
        if (isMountedRef.current) {
          setError(err instanceof Error ? err.message : 'Polling error');
        }
      }

      pollTimeoutRef.current = setTimeout(poll, mergedConfig.pollInterval);
    };

    poll();
  }, [mergedConfig.fallbackToPoll, mergedConfig.pollInterval]);

  const stopPolling = useCallback(() => {
    if (pollTimeoutRef.current) {
      clearTimeout(pollTimeoutRef.current);
      pollTimeoutRef.current = null;
    }
  }, []);

  // Setup heartbeat (periodic ping)
  const setupHeartbeat = useCallback(() => {
    if (heartbeatTimeoutRef.current) {
      clearTimeout(heartbeatTimeoutRef.current);
    }

    const sendHeartbeat = () => {
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        try {
          wsRef.current.send(JSON.stringify({ action: 'ping' }));
        } catch (err) {
          console.error('Heartbeat send error:', err);
        }
      }
      heartbeatTimeoutRef.current = setTimeout(sendHeartbeat, mergedConfig.heartbeatInterval);
    };

    heartbeatTimeoutRef.current = setTimeout(sendHeartbeat, mergedConfig.heartbeatInterval);
  }, [mergedConfig.heartbeatInterval]);

  const clearHeartbeat = useCallback(() => {
    if (heartbeatTimeoutRef.current) {
      clearTimeout(heartbeatTimeoutRef.current);
      heartbeatTimeoutRef.current = null;
    }
  }, []);

  // Connect to WebSocket
  const connect = useCallback(() => {
    if (!isMountedRef.current) return;

    // Clear any existing timeouts
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }

    setReconnecting(true);

    try {
      const token = localStorage.getItem('auth_token');
      const wsUrl = `${mergedConfig.url}?token=${encodeURIComponent(token || '')}`;

      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        if (!isMountedRef.current) return;

        console.log('WebSocket connected');
        setConnected(true);
        setReconnecting(false);
        setError(null);
        reconnectAttemptRef.current = 0;
        setupHeartbeat();
        stopPolling();
        setLoading(false);
      };

      wsRef.current.onmessage = (event) => {
        if (!isMountedRef.current) return;

        try {
          const message: WebSocketMessage = JSON.parse(event.data);

          if (message.type === 'dashboard_update' && message.data) {
            setData(message.data);
            setError(null);
          } else if (message.type === 'error' && message.error) {
            setError(message.error);
          } else if (message.type === 'pong') {
            // Heartbeat response received
            console.log('Heartbeat pong received');
          }
        } catch (err) {
          console.error('Message parse error:', err);
        }
      };

      wsRef.current.onerror = (event) => {
        if (!isMountedRef.current) return;

        const errorMsg = 'WebSocket error occurred';
        console.error(errorMsg, event);
        setError(errorMsg);
      };

      wsRef.current.onclose = () => {
        if (!isMountedRef.current) return;

        console.log('WebSocket disconnected');
        clearHeartbeat();
        setConnected(false);

        // Attempt to reconnect with exponential backoff
        if (reconnectAttemptRef.current < mergedConfig.maxReconnectAttempts) {
          const delayMs = mergedConfig.reconnectInterval * Math.pow(2, reconnectAttemptRef.current);
          reconnectAttemptRef.current++;

          console.log(
            `Reconnecting in ${delayMs}ms (attempt ${reconnectAttemptRef.current}/${mergedConfig.maxReconnectAttempts})`
          );

          setReconnecting(true);
          reconnectTimeoutRef.current = setTimeout(connect, delayMs);
        } else {
          // Max reconnect attempts reached, fall back to polling
          console.warn('Max reconnect attempts reached, falling back to polling');
          startPolling();
        }
      };
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Connection error';
      console.error('WebSocket connection error:', err);

      if (isMountedRef.current) {
        setError(errorMsg);
        setConnected(false);

        // Attempt to reconnect
        if (reconnectAttemptRef.current < mergedConfig.maxReconnectAttempts) {
          const delayMs = mergedConfig.reconnectInterval * Math.pow(2, reconnectAttemptRef.current);
          reconnectAttemptRef.current++;
          reconnectTimeoutRef.current = setTimeout(connect, delayMs);
        } else {
          startPolling();
        }
      }
    }
  }, [mergedConfig, setupHeartbeat, clearHeartbeat, stopPolling, startPolling]);

  // Disconnect from WebSocket
  const disconnect = useCallback(() => {
    clearHeartbeat();
    stopPolling();

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    setConnected(false);
    setReconnecting(false);
  }, [clearHeartbeat, stopPolling]);

  // Manual refresh
  const refresh = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      try {
        wsRef.current.send(JSON.stringify({ action: 'refresh' }));
      } catch (err) {
        console.error('Refresh send error:', err);
      }
    } else if (connected === false) {
      // If not connected, try to reconnect
      connect();
    }
  }, [connected, connect]);

  // Setup and teardown
  useEffect(() => {
    isMountedRef.current = true;
    setLoading(true);

    // Attempt WebSocket connection
    connect();

    return () => {
      isMountedRef.current = false;
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    data,
    loading,
    error,
    connected,
    reconnecting,
    clientCount,
    refresh,
  };
}

/**
 * Hook with polling as primary (for fallback scenarios)
 * Uses WebSocket if available, falls back to polling
 */
export function useDashboardWithWebSocket(
  webSocketEnabled: boolean = true
): UseWebSocketState & { source: 'websocket' | 'polling' } {
  const webSocketState = useWebSocketDashboard({
    fallbackToPoll: true,
  });

  const source = webSocketState.connected ? 'websocket' : 'polling';

  return {
    ...webSocketState,
    source,
  };
}

/**
 * Hook for monitoring WebSocket connection status
 */
export function useWebSocketStatus() {
  const { connected, reconnecting, error, clientCount } = useWebSocketDashboard();

  return {
    connected,
    reconnecting,
    error,
    clientCount,
    isHealthy: connected && !error,
  };
}
