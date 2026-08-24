// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Scanner-driven mitigation completions, read from the shared realtime stream.
//
// This hook used to open its own EventSource on /mitigations/events, passing the
// access token as a query parameter because EventSource cannot set an
// Authorization header. That endpoint predates cookie sessions; it still exists
// for compatibility, but nothing in the app needs it any more:
//
//   • the session now travels in an HttpOnly cookie, so no credential goes in a
//     URL — and a token in a URL lands in access logs, proxy logs and history;
//   • the shared transport is one connection for the whole tab instead of one
//     per feature, with a single reconnect strategy and a single dedup window;
//   • the event arrives inside the canonical envelope, so a redelivery after a
//     reconnect no longer toasts the user a second time for the same thing.

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

import { realtime, type RealtimeEnvelope } from '../../lib/realtime';

export interface MitigationAutoCompleted {
  planId: string;
  subActionId: string;
  scannerJobId: string;
}

/**
 * Refreshes the mitigation views when the scanner auto-completes a sub-action.
 *
 * The callback receives the event's payload; the board itself re-reads from the
 * API rather than patching its own state, because the event says a change
 * happened and the API says what it is.
 */
export function useMitigationEvents(onAutoCompleted?: (evt: MitigationAutoCompleted) => void) {
  const qc = useQueryClient();

  useEffect(() => {
    realtime.start();

    return realtime.onEvent((event: RealtimeEnvelope) => {
      if (event.type !== 'mitigation.auto_completed') return;

      qc.invalidateQueries({ queryKey: ['mitigations'] });
      toast.success('Mitigation auto-détectée par le scanner ✓');

      if (onAutoCompleted) {
        const payload = event.payload ?? {};
        onAutoCompleted({
          planId: String(payload.planId ?? event.aggregate.id),
          subActionId: String(payload.subActionId ?? ''),
          scannerJobId: String(payload.scannerJobId ?? ''),
        });
      }
    });
  }, [qc, onAutoCompleted]);
}
