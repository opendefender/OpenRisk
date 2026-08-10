// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Connection status — the truth behind the header's status dot.
//
// The header used to render a green pulsing dot labelled "Realtime". It was a
// styled <span>: it pulsed green on a backend that was down, on a laptop with
// no network, on a tenant with no realtime channel at all. In a security tool a
// green light that is always green is worse than no light, because operators
// calibrate on it.
//
// This module is the real signal. It has exactly two inputs:
//   • the browser's online/offline events;
//   • the outcome of every call the axios client makes (wired in lib/api.ts).
// Nothing here polls or invents state: with no traffic and no offline event,
// the status stays where the last observation left it.

export type ConnectionState = 'online' | 'degraded' | 'offline';

export interface ConnectionStatus {
  state: ConnectionState;
  /** Timestamp (ms) of the last successful API response, 0 if none yet. */
  lastSuccessAt: number;
  /** Timestamp (ms) of the last transport-level failure, 0 if none. */
  lastFailureAt: number;
}

type Listener = (status: ConnectionStatus) => void;

const listeners = new Set<Listener>();

let status: ConnectionStatus = {
  state: typeof navigator !== 'undefined' && navigator.onLine === false ? 'offline' : 'online',
  lastSuccessAt: 0,
  lastFailureAt: 0,
};

function emit(next: ConnectionStatus): void {
  const changed =
    next.state !== status.state ||
    next.lastSuccessAt !== status.lastSuccessAt ||
    next.lastFailureAt !== status.lastFailureAt;
  status = next;
  if (changed) listeners.forEach((l) => l(status));
}

/** A response came back from the API. */
export function markApiSuccess(): void {
  emit({ ...status, state: navigator.onLine === false ? 'offline' : 'online', lastSuccessAt: Date.now() });
}

/**
 * A request failed at the transport level (no response: DNS, refused, timeout,
 * CORS). An HTTP error response is NOT a connection failure — a 403 means the
 * backend is very much alive — so callers must only report transport faults.
 */
export function markApiFailure(): void {
  emit({ ...status, state: navigator.onLine === false ? 'offline' : 'degraded', lastFailureAt: Date.now() });
}

export function getConnectionStatus(): ConnectionStatus {
  return status;
}

export function subscribeConnection(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

if (typeof window !== 'undefined') {
  window.addEventListener('online', () => emit({ ...status, state: 'online' }));
  window.addEventListener('offline', () => emit({ ...status, state: 'offline' }));
}
