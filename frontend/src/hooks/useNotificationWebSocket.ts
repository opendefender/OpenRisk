import { useEffect, useRef, useCallback } from 'react';

export interface Notification {
  id: string;
  type: string;
  subject: string;
  message: string;
  status: string;
  created_at: string;
}

export interface UseNotificationWebSocketOptions {
  authToken: string;
  url?: string;
  onMessage?: (notification: Notification) => void;
  onError?: (error: Error) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
}

/**
 * Custom hook for WebSocket notification updates
 * Provides real-time notification delivery with auto-reconnect
 * 
 * @example
 * const { isConnected, reconnect } = useNotificationWebSocket({
 *   authToken: token,
 *   onMessage: (notif) => console.log('New notification:', notif),
 *   onError: (err) => console.error('WebSocket error:', err),
 * });
 */
export const useNotificationWebSocket = ({
  authToken,
  url = process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws/notifications',
  onMessage,
  onError,
  onConnect,
  onDisconnect,
}: UseNotificationWebSocketOptions) => {
  const webSocketRef = useRef<WebSocket | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const maxReconnectAttemptsRef = useRef(5);
  const reconnectDelayRef = useRef(1000); // Start with 1 second
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const connect = useCallback(() => {
    if (webSocketRef.current?.readyState === WebSocket.OPEN) {
      return;
    }

    try {
      const wsUrl = `${url}?token=${encodeURIComponent(authToken)}`;
      webSocketRef.current = new WebSocket(wsUrl);

      webSocketRef.current.onopen = () => {
        console.log('[WebSocket] Connected');
        reconnectAttemptsRef.current = 0;
        reconnectDelayRef.current = 1000;
        onConnect?.();
      };

      webSocketRef.current.onmessage = (event: MessageEvent) => {
        try {
          const notification: Notification = JSON.parse(event.data);
          onMessage?.(notification);
        } catch (error) {
          console.error('[WebSocket] Failed to parse message:', error);
        }
      };

      webSocketRef.current.onerror = (event: Event) => {
        const error = new Error('WebSocket error occurred');
        console.error('[WebSocket] Error:', error);
        onError?.(error);
      };

      webSocketRef.current.onclose = () => {
        console.log('[WebSocket] Disconnected');
        onDisconnect?.();
        attemptReconnect();
      };
    } catch (error) {
      const err = error instanceof Error ? error : new Error('WebSocket connection failed');
      onError?.(err);
    }
  }, [authToken, url, onMessage, onError, onConnect, onDisconnect]);

  const attemptReconnect = useCallback(() => {
    if (reconnectAttemptsRef.current >= maxReconnectAttemptsRef.current) {
      console.error('[WebSocket] Max reconnection attempts reached');
      return;
    }

    reconnectAttemptsRef.current += 1;
    const delay = Math.min(reconnectDelayRef.current * 2, 30000); // Cap at 30 seconds
    reconnectDelayRef.current = delay;

    console.log(
      `[WebSocket] Attempting to reconnect in ${delay}ms (attempt ${reconnectAttemptsRef.current}/${maxReconnectAttemptsRef.current})`
    );

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }

    reconnectTimeoutRef.current = setTimeout(() => {
      connect();
    }, delay);
  }, [connect]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }

    if (webSocketRef.current) {
      webSocketRef.current.close();
      webSocketRef.current = null;
    }

    reconnectAttemptsRef.current = 0;
  }, []);

  const reconnect = useCallback(() => {
    disconnect();
    setTimeout(() => {
      connect();
    }, 100);
  }, [connect, disconnect]);

  const send = useCallback(
    (data: Record<string, any>) => {
      if (webSocketRef.current?.readyState === WebSocket.OPEN) {
        webSocketRef.current.send(JSON.stringify(data));
      } else {
        console.warn('[WebSocket] Not connected, message not sent');
      }
    },
    []
  );

  useEffect(() => {
    connect();

    return () => {
      disconnect();
    };
  }, [authToken]); // Only reconnect if authToken changes

  return {
    isConnected: webSocketRef.current?.readyState === WebSocket.OPEN,
    reconnect,
    send,
    disconnect,
  };
};

export default useNotificationWebSocket;
