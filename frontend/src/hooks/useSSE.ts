import { useEffect, useRef, useState, useCallback } from 'react';

export interface SSEEvent<T = any> {
  type: string;
  data: T;
  timestamp: string;
}

export interface UseSSEOptions {
  url: string;
  enabled?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  onMessage?: (event: SSEEvent) => void;
  onError?: (error: Event) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
}

export function useSSE({
  url,
  enabled = true,
  reconnectInterval = 3000,
  maxReconnectAttempts = 5,
  onMessage,
  onError,
  onConnect,
  onDisconnect,
}: UseSSEOptions) {
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<Event | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const connect = useCallback(() => {
    if (!enabled || eventSourceRef.current) return;

    try {
      const eventSource = new EventSource(url);

      eventSource.addEventListener('open', () => {
        setIsConnected(true);
        setError(null);
        reconnectAttemptsRef.current = 0;
        onConnect?.();
      });

      eventSource.addEventListener('message', (event: Event) => {
        try {
          const data = JSON.parse((event as MessageEvent).data);
          onMessage?.({
            type: 'message',
            data,
            timestamp: new Date().toISOString(),
          });
        } catch {
          // Handle non-JSON messages
          onMessage?.({
            type: 'message',
            data: (event as MessageEvent).data,
            timestamp: new Date().toISOString(),
          });
        }
      });

      // Listen for specific event types (e.g., risk.updated, risk.score_updated)
      eventSource.addEventListener('risk.created', (event: Event) => {
        const messageEvent = event as MessageEvent;
        try {
          const data = JSON.parse(messageEvent.data);
          onMessage?.({
            type: 'risk.created',
            data,
            timestamp: new Date().toISOString(),
          });
        } catch {
          console.error('Failed to parse SSE message', messageEvent.data);
        }
      });

      eventSource.addEventListener('risk.updated', (event: Event) => {
        const messageEvent = event as MessageEvent;
        try {
          const data = JSON.parse(messageEvent.data);
          onMessage?.({
            type: 'risk.updated',
            data,
            timestamp: new Date().toISOString(),
          });
        } catch {
          console.error('Failed to parse SSE message', messageEvent.data);
        }
      });

      eventSource.addEventListener('risk.score_updated', (event: Event) => {
        const messageEvent = event as MessageEvent;
        try {
          const data = JSON.parse(messageEvent.data);
          onMessage?.({
            type: 'risk.score_updated',
            data,
            timestamp: new Date().toISOString(),
          });
        } catch {
          console.error('Failed to parse SSE message', messageEvent.data);
        }
      });

      eventSource.addEventListener('error', (event: Event) => {
        setError(event);
        onError?.(event);
        eventSourceRef.current?.close();
        eventSourceRef.current = null;
        setIsConnected(false);
        onDisconnect?.();

        // Attempt reconnect
        if (reconnectAttemptsRef.current < maxReconnectAttempts) {
          reconnectAttemptsRef.current += 1;
          reconnectTimeoutRef.current = setTimeout(() => {
            connect();
          }, reconnectInterval * Math.pow(1.5, reconnectAttemptsRef.current - 1)); // Exponential backoff
        }
      });

      eventSourceRef.current = eventSource;
    } catch (err) {
      console.error('Failed to connect to SSE:', err);
      setError(err instanceof Event ? err : new Event('unknown_error'));
    }
  }, [url, enabled, onMessage, onError, onConnect, onDisconnect, reconnectInterval, maxReconnectAttempts]);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
    setIsConnected(false);
    onDisconnect?.();
  }, [onDisconnect]);

  const reconnect = useCallback(() => {
    reconnectAttemptsRef.current = 0;
    disconnect();
    connect();
  }, [disconnect, connect]);

  useEffect(() => {
    if (enabled) {
      connect();
    }
    return () => {
      disconnect();
    };
  }, [enabled, connect, disconnect]);

  return {
    isConnected,
    error,
    reconnect,
    disconnect,
  };
}
