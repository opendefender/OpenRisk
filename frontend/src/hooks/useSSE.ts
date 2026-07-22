// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

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

  // Keep the caller's callbacks in refs so `connect` never has to list them as
  // dependencies. Passing inline callbacks (the common case) otherwise recreates
  // `connect` on every render, which re-runs the mount effect (disconnect + reconnect)
  // on every render — turning a single failing endpoint into a burst of reconnects and
  // onError calls. Reading through refs decouples callback identity from the socket
  // lifecycle. See ROADMAP.md — Risks page "server error" toast bursts.
  const onMessageRef = useRef(onMessage);
  const onErrorRef = useRef(onError);
  const onConnectRef = useRef(onConnect);
  const onDisconnectRef = useRef(onDisconnect);
  useEffect(() => {
    onMessageRef.current = onMessage;
    onErrorRef.current = onError;
    onConnectRef.current = onConnect;
    onDisconnectRef.current = onDisconnect;
  }, [onMessage, onError, onConnect, onDisconnect]);

  const connect = useCallback(() => {
    if (!enabled || eventSourceRef.current) return;

    try {
      const eventSource = new EventSource(url);

      eventSource.addEventListener('open', () => {
        setIsConnected(true);
        setError(null);
        reconnectAttemptsRef.current = 0;
        onConnectRef.current?.();
      });

      eventSource.addEventListener('message', (event: Event) => {
        try {
          const data = JSON.parse((event as MessageEvent).data);
          onMessageRef.current?.({
            type: 'message',
            data,
            timestamp: new Date().toISOString(),
          });
        } catch {
          // Handle non-JSON messages
          onMessageRef.current?.({
            type: 'message',
            data: (event as MessageEvent).data,
            timestamp: new Date().toISOString(),
          });
        }
      });

      // Listen for specific event types (e.g., risk.updated, risk.score_updated)
      const forward = (type: string) => (event: Event) => {
        const messageEvent = event as MessageEvent;
        try {
          const data = JSON.parse(messageEvent.data);
          onMessageRef.current?.({ type, data, timestamp: new Date().toISOString() });
        } catch {
          console.error('Failed to parse SSE message', messageEvent.data);
        }
      };
      eventSource.addEventListener('risk.created', forward('risk.created'));
      eventSource.addEventListener('risk.updated', forward('risk.updated'));
      eventSource.addEventListener('risk.score_updated', forward('risk.score_updated'));

      eventSource.addEventListener('error', (event: Event) => {
        setError(event);
        onErrorRef.current?.(event);
        eventSourceRef.current?.close();
        eventSourceRef.current = null;
        setIsConnected(false);
        onDisconnectRef.current?.();

        // Attempt reconnect, capped. Once the cap is hit we stop for good instead of
        // hammering a permanently-unavailable endpoint (e.g. one that isn't deployed).
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
  }, [url, enabled, reconnectInterval, maxReconnectAttempts]);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    setIsConnected(false);
    onDisconnectRef.current?.();
  }, []);

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
