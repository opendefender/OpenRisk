// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// W0-07 — the realtime client's guarantees to its subscribers.
//
// These assert observable behaviour: what a subscriber receives, what the
// status says, what URL is opened. A fake EventSource stands in for the browser
// so a reconnect, a duplicate and a sequence gap can be produced on demand —
// none of which can be provoked reliably against a real server.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

import {
  createRealtimeClientForTests,
  type RealtimeClient,
  type RealtimeEnvelope,
  type ResyncReason,
} from '../realtime';

// --- a controllable EventSource -------------------------------------------

type Listener = (e: MessageEvent) => void;

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  static last(): FakeEventSource {
    return FakeEventSource.instances[FakeEventSource.instances.length - 1];
  }

  onmessage: Listener | null = null;
  onerror: (() => void) | null = null;
  closed = false;
  readonly listeners = new Map<string, Listener[]>();

  constructor(
    readonly url: string,
    readonly init?: { withCredentials?: boolean },
  ) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: Listener): void {
    const existing = this.listeners.get(type) ?? [];
    existing.push(listener);
    this.listeners.set(type, existing);
  }

  close(): void {
    this.closed = true;
  }

  /** Delivers a named frame, as the server would. */
  emit(type: string, data: unknown): void {
    const payload = { data: JSON.stringify(data) } as MessageEvent;
    for (const l of this.listeners.get(type) ?? []) l(payload);
  }

  fail(): void {
    this.onerror?.();
  }
}

function envelope(overrides: Partial<RealtimeEnvelope> = {}): RealtimeEnvelope {
  return {
    id: 'evt-1',
    envelopeVersion: 1,
    type: 'risk.created',
    version: 1,
    occurredAt: '2026-08-24T10:00:00Z',
    tenantId: 'tenant-a',
    aggregate: { type: 'risk', id: 'risk-1' },
    sequence: 1,
    ...overrides,
  };
}

/** Opens the stream and completes the handshake the server always sends. */
function connect(client: RealtimeClient, tenantId = 'tenant-a'): FakeEventSource {
  client.start();
  const source = FakeEventSource.last();
  source.emit('stream.hello', { tenant_id: tenantId, connection_id: 'conn-1', envelope_version: 1 });
  return source;
}

describe('realtime client', () => {
  let client: RealtimeClient;

  beforeEach(() => {
    FakeEventSource.instances = [];
    vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);
    vi.useFakeTimers();
    client = createRealtimeClientForTests();
  });

  afterEach(() => {
    client.stop();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('opens one connection for the tab, however many consumers ask', () => {
    client.start();
    client.start();
    client.start();
    expect(FakeEventSource.instances).toHaveLength(1);
  });

  // A credential in a URL ends up in access logs, proxy logs and browser
  // history. The session travels in the HttpOnly cookie instead.
  it('sends no credential in the stream URL and asks for cookies', () => {
    client.start();
    const source = FakeEventSource.last();
    expect(source.url).not.toMatch(/token|jwt|authorization/i);
    expect(source.init?.withCredentials).toBe(true);
  });

  it('reports CONNECTED only after the server has identified the stream', () => {
    client.start();
    expect(client.getStatus().state).toBe('INITIALIZING');

    FakeEventSource.last().emit('stream.hello', { tenant_id: 'tenant-a' });
    expect(client.getStatus().state).toBe('CONNECTED');
  });

  it('delivers an event to every subscriber and advances the cursor', () => {
    const seen: RealtimeEnvelope[] = [];
    client.onEvent((e) => seen.push(e));
    const source = connect(client);

    source.emit('risk.created', envelope({ id: 'evt-1', sequence: 1 }));

    expect(seen).toHaveLength(1);
    expect(seen[0].id).toBe('evt-1');
    expect(client.getStatus().cursor).toBe(1);
  });

  // THE duplicate guarantee: the same event three times is applied once.
  it('delivers a repeated event exactly once', () => {
    const seen: RealtimeEnvelope[] = [];
    client.onEvent((e) => seen.push(e));
    const source = connect(client);

    const e = envelope({ id: 'evt-dup', sequence: 1 });
    source.emit('risk.created', e);
    source.emit('risk.created', e);
    source.emit('risk.created', e);

    expect(seen).toHaveLength(1);
  });

  // A jump in the sequence means something was missed. Applying the newer event
  // over a change nobody saw would leave the screen quietly wrong.
  it('reports a sequence gap instead of hiding it', () => {
    const resyncs: ResyncReason[] = [];
    client.onResync((r) => resyncs.push(r));
    const source = connect(client);

    source.emit('risk.created', envelope({ id: 'e1', sequence: 1 }));
    expect(resyncs).toHaveLength(0);

    source.emit('risk.updated', envelope({ id: 'e5', type: 'risk.updated', sequence: 5 }));
    expect(resyncs).toEqual(['sequence_gap']);
  });

  it('does not report a gap for the first event of a fresh connection', () => {
    const resyncs: ResyncReason[] = [];
    client.onResync((r) => resyncs.push(r));
    const source = connect(client);

    source.emit('risk.created', envelope({ id: 'e42', sequence: 42 }));
    expect(resyncs).toEqual([]);
    expect(client.getStatus().cursor).toBe(42);
  });

  it('resumes from its cursor when it drives the reconnect itself', () => {
    const source = connect(client);
    source.emit('risk.created', envelope({ id: 'e7', sequence: 7 }));

    source.fail();
    vi.advanceTimersByTime(2_000);

    const reconnected = FakeEventSource.last();
    expect(reconnected).not.toBe(source);
    expect(reconnected.url).toContain('last_event_id=7');
  });

  it('backs off with jitter instead of retrying in a tight loop', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    connect(client);

    const opened = () => FakeEventSource.instances.length;
    const before = opened();

    FakeEventSource.last().fail();
    expect(client.getStatus().state).toBe('RECONNECTING');
    // Nothing at all for the first half-second: a retry storm is caused by the
    // retry policy, not by the outage.
    vi.advanceTimersByTime(400);
    expect(opened()).toBe(before);

    vi.advanceTimersByTime(200);
    expect(opened()).toBe(before + 1);
  });

  it('escalates the delay across consecutive failures', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0);
    connect(client);

    FakeEventSource.last().fail();
    vi.advanceTimersByTime(600); // ~500ms for attempt 1
    expect(client.getStatus().attempts).toBe(1);

    FakeEventSource.last().fail();
    expect(client.getStatus().attempts).toBe(2);
    const beforeThird = FakeEventSource.instances.length;
    vi.advanceTimersByTime(600); // attempt 2 waits ~1s, so this is not enough
    expect(FakeEventSource.instances.length).toBe(beforeThird);
    vi.advanceTimersByTime(600);
    expect(FakeEventSource.instances.length).toBe(beforeThird + 1);
  });

  // Silence past the watchdog is a dead socket that still looks open. The
  // client has to notice it itself, because nothing else will tell it.
  it('treats prolonged silence as a dead connection', () => {
    connect(client);
    expect(client.getStatus().state).toBe('CONNECTED');

    vi.advanceTimersByTime(71_000);
    expect(client.getStatus().state).toBe('RECONNECTING');
  });

  it('keeps the connection alive while heartbeats arrive', () => {
    const source = connect(client);
    for (let i = 0; i < 5; i += 1) {
      vi.advanceTimersByTime(30_000);
      source.emit('stream.heartbeat', { server_time: '2026-08-24T10:00:00Z' });
    }
    expect(client.getStatus().state).toBe('CONNECTED');
  });

  it('drops its cursor and asks consumers to refetch when told to resync', () => {
    const resyncs: ResyncReason[] = [];
    client.onResync((r) => resyncs.push(r));
    const source = connect(client);
    source.emit('risk.created', envelope({ id: 'e3', sequence: 3 }));

    source.emit('stream.resync', { reason: 'cursor_expired', oldest_retained: 99 });

    expect(resyncs).toEqual(['cursor_expired']);
    expect(client.getStatus().cursor).toBe(0);
    expect(client.getStatus().state).toBe('RESYNCING');
  });

  // A recycled stream is routine housekeeping. Asking every client to refetch
  // everything for it would turn a scheduled reconnect into a thundering herd.
  it('does not ask for a refetch when the server merely recycles the stream', () => {
    const resyncs: ResyncReason[] = [];
    client.onResync((r) => resyncs.push(r));
    const source = connect(client);
    source.emit('risk.created', envelope({ id: 'e3', sequence: 3 }));

    source.emit('stream.resync', { reason: 'stream_recycled' });

    expect(resyncs).toEqual([]);
    expect(client.getStatus().cursor).toBe(3);
  });

  // THE tenant-switch guarantee: no stream from tenant A may survive under a UI
  // showing tenant B, and A's cursor must not be replayed against B's log.
  it('tears the stream down and drops the cursor on a tenant switch', () => {
    const resyncs: ResyncReason[] = [];
    client.onResync((r) => resyncs.push(r));
    const source = connect(client, 'tenant-a');
    source.emit('risk.created', envelope({ id: 'e9', sequence: 9 }));
    expect(client.getStatus().cursor).toBe(9);

    client.switchTenant();

    expect(source.closed).toBe(true);
    expect(resyncs).toContain('tenant_changed');
    expect(client.getStatus().cursor).toBe(0);

    const reconnected = FakeEventSource.last();
    expect(reconnected).not.toBe(source);
    expect(reconnected.url).not.toContain('last_event_id');
  });

  // Belt and braces: if the server reports a different tenant than the one the
  // previous connection served, the cursor is meaningless and is dropped.
  it('drops the cursor when the server reports a different tenant', () => {
    const resyncs: ResyncReason[] = [];
    client.onResync((r) => resyncs.push(r));
    const source = connect(client, 'tenant-a');
    source.emit('risk.created', envelope({ id: 'e4', sequence: 4 }));

    source.fail();
    vi.advanceTimersByTime(2_000);
    FakeEventSource.last().emit('stream.hello', { tenant_id: 'tenant-b' });

    expect(resyncs).toContain('tenant_changed');
    expect(client.getStatus().cursor).toBe(0);
  });

  it('stops for good when told to, and closes the socket', () => {
    const source = connect(client);
    client.stop();

    expect(source.closed).toBe(true);
    expect(client.getStatus().state).toBe('DISCONNECTED');

    const count = FakeEventSource.instances.length;
    source.fail();
    vi.advanceTimersByTime(60_000);
    expect(FakeEventSource.instances.length).toBe(count);
  });

  it('ignores an unreadable frame without tearing the connection down', () => {
    const seen: RealtimeEnvelope[] = [];
    client.onEvent((e) => seen.push(e));
    const source = connect(client);

    for (const l of source.listeners.get('risk.created') ?? []) {
      l({ data: '{not json' } as MessageEvent);
    }
    expect(seen).toHaveLength(0);
    expect(client.getStatus().state).toBe('CONNECTED');

    source.emit('risk.created', envelope({ id: 'good', sequence: 1 }));
    expect(seen).toHaveLength(1);
  });

  it('keeps notifying the other subscribers when one throws', () => {
    const seen: string[] = [];
    client.onEvent(() => {
      throw new Error('a consumer defect');
    });
    client.onEvent((e) => seen.push(e.id));
    const source = connect(client);

    source.emit('risk.created', envelope({ id: 'e1', sequence: 1 }));
    expect(seen).toEqual(['e1']);
  });

  it('unsubscribing actually stops delivery', () => {
    const seen: string[] = [];
    const off = client.onEvent((e) => seen.push(e.id));
    const source = connect(client);

    source.emit('risk.created', envelope({ id: 'e1', sequence: 1 }));
    off();
    source.emit('risk.updated', envelope({ id: 'e2', type: 'risk.updated', sequence: 2 }));

    expect(seen).toEqual(['e1']);
  });
});
