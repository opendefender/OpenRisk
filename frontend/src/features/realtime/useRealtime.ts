// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// React bindings for the shared realtime transport.
//
// The rule these hooks exist to enforce: a component subscribes, it never
// connects. There is one connection per tab (src/lib/realtime.ts) and these are
// the only sanctioned ways to read from it.

import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import {
  realtime,
  type RealtimeEnvelope,
  type RealtimeStatus,
  type ResyncReason,
} from '../../lib/realtime';

/**
 * Which query keys an aggregate's events invalidate.
 *
 * Invalidation rather than patching the cache from the payload, deliberately.
 * The event says an aggregate changed and names the fields; the API says what
 * it changed to. Writing the payload straight into the cache would make the
 * stream a second source of truth, and the first time the two disagreed there
 * would be no way to tell which was right.
 */
const AGGREGATE_QUERY_KEYS: Record<string, string[][]> = {
  risk: [['risks'], ['dashboard'], ['analytics'], ['executive']],
  asset: [['assets'], ['asset-dependencies'], ['dashboard'], ['executive']],
  vulnerability: [['vulnerabilities'], ['vuln-stats'], ['dashboard'], ['executive']],
  incident: [['incidents'], ['incident-stats'], ['dashboard'], ['executive']],
  control: [['compliance'], ['compliance', 'overview'], ['executive']],
  compliance_audit: [['compliance', 'audits'], ['compliance'], ['executive']],
  mitigation: [['mitigations'], ['risks'], ['dashboard']],
};

/** The live connection status, for anything that displays it. */
export function useRealtimeStatus(): RealtimeStatus {
  const [status, setStatus] = useState<RealtimeStatus>(() => realtime.getStatus());
  useEffect(() => realtime.onStatus(setStatus), []);
  return status;
}

/**
 * Subscribes to events, optionally narrowed to some aggregates.
 *
 * The filter here is a client-side convenience on top of the one the server
 * already applied; it is never what keeps another tenant's events away, which
 * is the server's job and only the server's.
 */
export function useRealtimeEvents(
  onEvent: (event: RealtimeEnvelope) => void,
  aggregates?: string[],
): void {
  useEffect(() => {
    return realtime.onEvent((event) => {
      if (aggregates && !aggregates.includes(event.aggregate.type)) return;
      onEvent(event);
    });
    // The callback is intentionally in the dependency list: a caller passing an
    // inline function re-subscribes, which is correct and cheap because
    // subscribing does not touch the connection.
  }, [onEvent, aggregates]);
}

/**
 * Opens the shared connection and keeps the query cache in step with it.
 *
 * Mounted once, high in the tree. Everything else in the app keeps using
 * ordinary queries and simply sees them refresh.
 */
export function useRealtimeQuerySync(): RealtimeStatus {
  const queryClient = useQueryClient();
  const status = useRealtimeStatus();

  useEffect(() => {
    realtime.start();
    return () => {
      // Deliberately NOT stopping the client here. The connection belongs to
      // the tab, not to this component; tearing it down on a re-mount would
      // drop the cursor and cost a replay for nothing. It is stopped on logout,
      // which is the moment it genuinely must end.
    };
  }, []);

  useEffect(() => {
    const offEvent = realtime.onEvent((event) => {
      const keys = AGGREGATE_QUERY_KEYS[event.aggregate.type];
      if (!keys) return;
      for (const key of keys) {
        queryClient.invalidateQueries({ queryKey: key });
      }
    });

    const offResync = realtime.onResync((reason: ResyncReason) => {
      // The client's view can no longer be trusted to be complete, so
      // everything server-derived is dropped and refetched. This is the
      // recovery path the whole design leans on: the realtime stream is never
      // the durable state, so a full refetch is always a correct answer.
      void reason;
      queryClient.invalidateQueries();
    });

    return () => {
      offEvent();
      offResync();
    };
  }, [queryClient]);

  return status;
}
