// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Subscribes to the backend mitigation SSE stream (GET /mitigations/events).
// When the Infrastructure Scanner auto-completes a sub-action (a CVE it can no
// longer detect), the backend publishes mitigation.auto_completed; here we
// invalidate the mitigations queries so the map re-renders with the
// "Auto-detected" badge + green state, and toast the user.
//
// Native EventSource can't send an Authorization header, so the access token is
// passed as ?token= (the backend validates it and filters events to the tenant).

import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';

// Matches the hardcoded baseURL in src/lib/api.ts.
const API_BASE = 'http://localhost:8080/api/v1';

export interface MitigationAutoCompleted {
  tenant_id: string;
  plan_id: string;
  sub_action_id: string;
  scanner_job_id: string;
  evidence: string;
}

export function useMitigationEvents(onAutoCompleted?: (evt: MitigationAutoCompleted) => void) {
  const qc = useQueryClient();

  useEffect(() => {
    const token = localStorage.getItem('auth_token');
    if (!token || typeof window.EventSource === 'undefined') return;

    const es = new EventSource(`${API_BASE}/mitigations/events?token=${encodeURIComponent(token)}`);

    const handler = (e: MessageEvent) => {
      let evt: MitigationAutoCompleted | null = null;
      try {
        evt = JSON.parse(e.data) as MitigationAutoCompleted;
      } catch {
        evt = null;
      }
      // Refresh anything mitigation-related so the board + drawer pick up the
      // scanner-completed sub-action.
      qc.invalidateQueries({ queryKey: ['mitigations'] });
      toast.success('Mitigation auto-détectée par le scanner ✓');
      if (evt && onAutoCompleted) onAutoCompleted(evt);
    };

    es.addEventListener('mitigation.auto_completed', handler as EventListener);
    // EventSource reconnects on its own; swallow transient errors in dev.
    es.onerror = () => {};

    return () => {
      es.removeEventListener('mitigation.auto_completed', handler as EventListener);
      es.close();
    };
  }, [qc, onAutoCompleted]);
}
