// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Offline / unstable-connection indicator (task §2). Surfaces the connection
// state from lib/connection.ts and, when writes are queued (React Query pauses
// mutations while offline and replays them on reconnect), how many are waiting —
// so a user on a flaky African mobile link knows their changes aren't lost.

import { useEffect, useState } from 'react';
import { useMutationState } from '@tanstack/react-query';
import { CloudOff, RefreshCw } from 'lucide-react';
import { subscribeConnection, getConnectionStatus, type ConnectionState } from '../lib/connection';
import { useUIStore } from '../store/uiStore';

export function OfflineBanner() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const [state, setState] = useState<ConnectionState>(getConnectionStatus().state);
  useEffect(() => subscribeConnection((s) => setState(s.state)), []);

  // Count paused (queued) mutations — the writes waiting for the connection.
  const pausedCount = useMutationState({
    filters: { status: 'pending' },
    select: (m) => (m.state.isPaused ? 1 : 0),
  }).reduce((a: number, b: number) => a + b, 0);

  // Quiet while healthy; degraded shows only when there is queued work to explain.
  if (state === 'online') return null;
  if (state === 'degraded' && pausedCount === 0) return null;

  const offline = state === 'offline';
  return (
    <div
      role="status"
      aria-live="polite"
      className="flex items-center justify-center gap-2 px-4 py-1.5 text-[12.5px] font-medium"
      style={{
        background: offline ? 'color-mix(in srgb,var(--critical) 16%,transparent)' : 'color-mix(in srgb,var(--medium) 16%,transparent)',
        color: offline ? 'var(--critical)' : 'var(--medium)',
        borderBottom: '1px solid var(--border)',
      }}
    >
      {offline ? <CloudOff size={14} /> : <RefreshCw size={14} />}
      <span>
        {offline
          ? tr('Hors ligne', 'Offline')
          : tr('Connexion instable', 'Unstable connection')}
        {pausedCount > 0 && (
          <>
            {' — '}
            {tr(
              `${pausedCount} modification(s) en attente d'envoi`,
              `${pausedCount} change(s) waiting to sync`,
            )}
          </>
        )}
        {offline && pausedCount === 0 && (
          <> — {tr('vos données restent consultables', 'your data stays viewable')}</>
        )}
      </span>
    </div>
  );
}
