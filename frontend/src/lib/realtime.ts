// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The realtime client — one shared transport for the whole app.
//
// WHY ONE. Before this, every surface that wanted live data opened its own
// connection: a mitigation EventSource here, two WebSocket hooks pointing at
// endpoints the backend never had, and polling everywhere else. N components
// meant N connections, N reconnect strategies and N sets of duplicate handling,
// which is how a browser ends up holding six sockets to say one thing.
//
// This module owns exactly one connection for the tab. Components subscribe to
// it; they never construct one.
//
// WHAT IT GUARANTEES TO ITS SUBSCRIBERS
//   • an event is delivered at most once, keyed on its server-minted id;
//   • events are delivered in the tenant's sequence order, and a gap in that
//     sequence is reported rather than hidden;
//   • the connection state is always an honest one of the states below — there
//     is no "connected" that means "we gave up and are polling";
//   • when the server says the client's view can no longer be trusted, the
//     client says so too, and asks its consumers to refetch.
//
// WHAT IT IS NOT. It is not a store, and the events are not the source of
// truth. An event says "this changed, go and look"; the API says what the
// change was. Treating the stream as the state is how two clients end up
// disagreeing about the same record with no way to tell which is right.

import { getAccessToken } from './session';

/**
 * Connection states.
 *
 * These are the ones a user could be shown without being lied to. In
 * particular there is no state for "degraded but pretending": a client that
 * cannot hold a stream is DISCONNECTED or ERROR, and the UI is entitled to say
 * so.
 */
export type RealtimeState =
  | 'INITIALIZING'
  | 'CONNECTED'
  | 'RECONNECTING'
  | 'DISCONNECTED'
  | 'RESYNCING'
  | 'FORBIDDEN'
  | 'ERROR';

/** The canonical envelope, mirroring backend/pkg/realtime.Envelope. */
export interface RealtimeEnvelope {
  id: string;
  envelopeVersion: number;
  type: string;
  version: number;
  occurredAt: string;
  tenantId: string;
  actorId?: string;
  aggregate: { type: string; id: string };
  sequence: number;
  correlationId?: string;
  causationId?: string;
  payload?: Record<string, unknown>;
}

/** Why the client believes its view is stale. */
export type ResyncReason =
  | 'cursor_expired'
  | 'buffer_overflow'
  | 'replay_too_large'
  | 'replay_failed'
  | 'replay_unavailable'
  | 'stream_recycled'
  | 'sequence_gap'
  | 'tenant_changed';

export interface RealtimeStatus {
  state: RealtimeState;
  /** Sequence of the last event applied; 0 before anything arrived. */
  cursor: number;
  /** Consecutive failed connection attempts. */
  attempts: number;
  /** Last time a frame of any kind arrived (ms epoch), 0 if never. */
  lastFrameAt: number;
  /** Set when the state is ERROR or FORBIDDEN, for the UI to show. */
  error?: string;
}

type EventListener = (event: RealtimeEnvelope) => void;
type StatusListener = (status: RealtimeStatus) => void;
type ResyncListener = (reason: ResyncReason) => void;

/** Backoff schedule, in milliseconds, before jitter. */
const BACKOFF_MS = [1_000, 2_000, 4_000, 8_000, 16_000, 30_000];

/**
 * How long without any frame before the connection is presumed dead.
 *
 * The server sends a named heartbeat every 20 s. Three missed heartbeats is a
 * dead connection, not a slow one — and the case this exists for is the silent
 * one: a laptop that suspends, or a middlebox that drops the socket without
 * either side seeing a FIN. Without a watchdog, EventSource sits in a state that
 * looks open forever.
 */
const LIVENESS_TIMEOUT_MS = 70_000;

/** Ids remembered for duplicate suppression. */
const DEDUP_WINDOW = 2_000;

const API_BASE = import.meta.env.VITE_API_URL ?? '/api/v1';

class RealtimeClient {
  private source: EventSource | null = null;
  private status: RealtimeStatus = {
    state: 'INITIALIZING',
    cursor: 0,
    attempts: 0,
    lastFrameAt: 0,
  };

  private readonly eventListeners = new Set<EventListener>();
  private readonly statusListeners = new Set<StatusListener>();
  private readonly resyncListeners = new Set<ResyncListener>();

  private seenIds = new Set<string>();
  private seenOrder: string[] = [];

  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private livenessTimer: ReturnType<typeof setTimeout> | null = null;

  /** The tenant this connection belongs to, as the SERVER reported it. */
  private tenantId: string | null = null;
  private started = false;

  // -- public API ---------------------------------------------------------

  /**
   * Opens the stream, if it is not already open.
   *
   * Idempotent: every consumer may call it on mount without coordinating with
   * the others, which is what lets components stay ignorant of each other.
   */
  start(): void {
    if (this.started) return;
    this.started = true;
    this.connect();

    if (typeof window !== 'undefined') {
      window.addEventListener('online', this.handleOnline);
      window.addEventListener('offline', this.handleOffline);
    }
  }

  /** Closes the stream and forgets everything about it. */
  stop(): void {
    this.started = false;
    this.clearTimers();
    this.closeSource();
    this.setStatus({ state: 'DISCONNECTED', attempts: 0 });

    if (typeof window !== 'undefined') {
      window.removeEventListener('online', this.handleOnline);
      window.removeEventListener('offline', this.handleOffline);
    }
  }

  /**
   * Tears the stream down and rebuilds it for a different tenant.
   *
   * The cursor and the deduplication window are dropped, not carried over: a
   * cursor is a position in ONE tenant's sequence, and replaying it against
   * another would ask for events by a number that means something else there.
   * Consumers are told to resynchronise because nothing they hold is about the
   * tenant now on screen.
   */
  switchTenant(): void {
    this.clearTimers();
    this.closeSource();
    this.resetCursor();
    this.tenantId = null;
    this.emitResync('tenant_changed');
    if (this.started) {
      this.setStatus({ state: 'RESYNCING', attempts: 0 });
      this.connect();
    }
  }

  onEvent(listener: EventListener): () => void {
    this.eventListeners.add(listener);
    return () => this.eventListeners.delete(listener);
  }

  onStatus(listener: StatusListener): () => void {
    this.statusListeners.add(listener);
    listener(this.status);
    return () => this.statusListeners.delete(listener);
  }

  /** Fires when the client's view can no longer be trusted to be complete. */
  onResync(listener: ResyncListener): () => void {
    this.resyncListeners.add(listener);
    return () => this.resyncListeners.delete(listener);
  }

  getStatus(): RealtimeStatus {
    return this.status;
  }

  // -- connection ---------------------------------------------------------

  private connect(): void {
    if (typeof window === 'undefined' || typeof window.EventSource === 'undefined') {
      // No EventSource (SSR, or a very old browser): say so rather than
      // pretending to be connected. Consumers fall back to their own refetching.
      this.setStatus({ state: 'ERROR', error: 'This browser cannot hold a live connection.' });
      return;
    }
    this.closeSource();

    // The session travels in the HttpOnly cookie, which is why withCredentials
    // is set and why no token appears in this URL. A token in a query string
    // ends up in access logs, proxy logs and browser history, and the cookie
    // makes it unnecessary. The bearer fallback below exists only for a
    // deployment that genuinely splits the SPA and API origins, where the
    // cookie cannot be attached.
    const params = new URLSearchParams();
    if (this.status.cursor > 0) {
      // EventSource sends Last-Event-ID by itself on ITS OWN reconnects. This
      // parameter covers the reconnects we drive, where the browser starts a
      // fresh connection with no memory of the last id.
      params.set('last_event_id', String(this.status.cursor));
    }
    const splitOriginToken = import.meta.env.VITE_API_URL ? getAccessToken() : null;
    if (splitOriginToken) params.set('access_token', splitOriginToken);

    const query = params.toString();
    const url = `${API_BASE}/realtime/events${query ? `?${query}` : ''}`;

    const source = new EventSource(url, { withCredentials: true });
    this.source = source;

    source.addEventListener('stream.hello', (e) => this.handleHello(e as MessageEvent));
    source.addEventListener('stream.heartbeat', () => this.markFrame());
    source.addEventListener('stream.resync', (e) => this.handleResync(e as MessageEvent));
    source.onmessage = (e) => this.handleEvent(e);
    source.onerror = () => this.handleError();

    // Any named domain event arrives under its own type, so the catalog's names
    // are attached individually rather than through onmessage.
    for (const type of KNOWN_EVENT_TYPES) {
      source.addEventListener(type, (e) => this.handleEvent(e as MessageEvent));
    }

    this.armLiveness();
  }

  private handleHello(e: MessageEvent): void {
    this.markFrame();
    let hello: { tenant_id?: string } = {};
    try {
      hello = JSON.parse(e.data) as { tenant_id?: string };
    } catch {
      hello = {};
    }

    // The server names the tenant behind this stream. If it is not the one the
    // previous connection served, the cursor we were about to resume from
    // belongs to a different sequence — so it is dropped and consumers are told
    // to refetch. This is what stops a session change from quietly replaying
    // one tenant's positions against another's log.
    if (hello.tenant_id && this.tenantId && hello.tenant_id !== this.tenantId) {
      this.resetCursor();
      this.emitResync('tenant_changed');
    }
    if (hello.tenant_id) this.tenantId = hello.tenant_id;

    this.setStatus({ state: 'CONNECTED', attempts: 0, error: undefined });
  }

  private handleResync(e: MessageEvent): void {
    this.markFrame();
    let reason: ResyncReason = 'replay_failed';
    try {
      const body = JSON.parse(e.data) as { reason?: ResyncReason };
      if (body.reason) reason = body.reason;
    } catch {
      /* keep the default: something went wrong is still the right message */
    }
    // A recycled stream is routine housekeeping, not a lost view: the server
    // closes long-lived connections on purpose and the cursor is still valid,
    // so consumers are not asked to refetch for it.
    if (reason === 'stream_recycled') return;

    this.resetCursor();
    this.setStatus({ state: 'RESYNCING' });
    this.emitResync(reason);
  }

  private handleEvent(e: MessageEvent): void {
    this.markFrame();
    let envelope: RealtimeEnvelope;
    try {
      envelope = JSON.parse(e.data) as RealtimeEnvelope;
    } catch {
      // A frame we cannot read is dropped rather than guessed at. It is not a
      // reason to tear the connection down: the next one is probably fine.
      return;
    }
    if (!envelope?.id || !envelope.type) return;

    // Duplicate suppression. The server deduplicates too, but a client that
    // trusts a distributed system to deliver exactly once has misunderstood the
    // system it is in.
    if (this.seenIds.has(envelope.id)) return;
    this.remember(envelope.id);

    // Gap detection. Sequences are contiguous per tenant, so a jump means
    // something was missed — say so instead of applying the newer event over a
    // change nobody saw.
    if (envelope.sequence > 0) {
      const expected = this.status.cursor + 1;
      if (this.status.cursor > 0 && envelope.sequence > expected) {
        this.emitResync('sequence_gap');
      }
      if (envelope.sequence > this.status.cursor) {
        this.setStatus({ cursor: envelope.sequence });
      }
    }

    for (const listener of this.eventListeners) {
      try {
        listener(envelope);
      } catch {
        // One misbehaving consumer must not stop the others from being told.
      }
    }
  }

  private handleError(): void {
    // EventSource reconnects on its own, but on its own schedule and with no
    // ceiling on how hard it tries. Driving reconnection here is what makes the
    // backoff, the jitter and the offline awareness ours.
    this.closeSource();
    if (!this.started) return;

    if (typeof navigator !== 'undefined' && navigator.onLine === false) {
      // Offline is not a server failure and must not burn backoff attempts:
      // the 'online' event will bring us back immediately.
      this.setStatus({ state: 'DISCONNECTED' });
      return;
    }

    const attempts = this.status.attempts + 1;
    this.setStatus({ state: 'RECONNECTING', attempts });
    this.scheduleReconnect(attempts);
  }

  private scheduleReconnect(attempts: number): void {
    const base = BACKOFF_MS[Math.min(attempts - 1, BACKOFF_MS.length - 1)];
    // Full jitter. Without it, every tab in every browser that lost the same
    // deploy reconnects at the same instant and knocks the instance over as it
    // comes back — the reconnect storm is caused by the retry policy, not by
    // the outage.
    const delay = Math.floor(base / 2 + Math.random() * (base / 2));
    this.clearReconnect();
    this.reconnectTimer = setTimeout(() => this.connect(), delay);
  }

  private handleOnline = (): void => {
    if (!this.started) return;
    // The network came back; there is nothing to wait for.
    this.clearReconnect();
    this.setStatus({ state: 'RECONNECTING' });
    this.connect();
  };

  private handleOffline = (): void => {
    this.clearTimers();
    this.closeSource();
    this.setStatus({ state: 'DISCONNECTED' });
  };

  // -- liveness -----------------------------------------------------------

  private markFrame(): void {
    this.setStatus({ lastFrameAt: Date.now() });
    this.armLiveness();
  }

  private armLiveness(): void {
    if (this.livenessTimer) clearTimeout(this.livenessTimer);
    this.livenessTimer = setTimeout(() => {
      // Silence past the watchdog means the socket is dead even though it looks
      // open. Tearing it down ourselves is the only way out of that state.
      if (!this.started) return;
      this.handleError();
    }, LIVENESS_TIMEOUT_MS);
  }

  // -- bookkeeping --------------------------------------------------------

  private remember(id: string): void {
    this.seenIds.add(id);
    this.seenOrder.push(id);
    if (this.seenOrder.length > DEDUP_WINDOW) {
      const evicted = this.seenOrder.shift();
      if (evicted) this.seenIds.delete(evicted);
    }
  }

  private resetCursor(): void {
    this.setStatus({ cursor: 0 });
    this.seenIds = new Set();
    this.seenOrder = [];
  }

  private setStatus(patch: Partial<RealtimeStatus>): void {
    this.status = { ...this.status, ...patch };
    for (const listener of this.statusListeners) {
      try {
        listener(this.status);
      } catch {
        /* a listener that throws must not stop the others */
      }
    }
  }

  private emitResync(reason: ResyncReason): void {
    for (const listener of this.resyncListeners) {
      try {
        listener(reason);
      } catch {
        /* as above */
      }
    }
  }

  private closeSource(): void {
    if (this.source) {
      this.source.close();
      this.source = null;
    }
  }

  private clearReconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private clearTimers(): void {
    this.clearReconnect();
    if (this.livenessTimer) {
      clearTimeout(this.livenessTimer);
      this.livenessTimer = null;
    }
  }
}

/**
 * The event types the client listens for by name.
 *
 * SSE dispatches a frame carrying `event: risk.created` to a listener
 * registered for that name, and to `onmessage` only when the frame has no name.
 * Every domain event is named, so the names have to be attached explicitly —
 * this list is the client-side mirror of backend/pkg/realtime's catalog, and
 * the contract test on GET /realtime/catalog is what keeps them honest with
 * each other.
 */
export const KNOWN_EVENT_TYPES = [
  'risk.created',
  'risk.updated',
  'risk.deleted',
  'risk.status_changed',
  'risk.score_changed',
  'asset.created',
  'asset.updated',
  'asset.deleted',
  'asset.criticality_changed',
  'vulnerability.detected',
  'vulnerability.updated',
  'vulnerability.deleted',
  'incident.created',
  'incident.updated',
  'incident.deleted',
  'control.created',
  'control.updated',
  'control.deleted',
  'assessment.created',
  'assessment.updated',
  'assessment.deleted',
  'mitigation.created',
  'mitigation.updated',
  'mitigation.deleted',
  'mitigation.auto_completed',
] as const;

export type KnownEventType = (typeof KNOWN_EVENT_TYPES)[number];

/** The tab's single realtime connection. */
export const realtime = new RealtimeClient();

/** Exported for tests, which need a client that is not the tab's shared one. */
export function createRealtimeClientForTests(): RealtimeClient {
  return new RealtimeClient();
}

export type { RealtimeClient };
