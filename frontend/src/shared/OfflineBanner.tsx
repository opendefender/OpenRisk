// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Offline / unstable-connection indicator (task §2). Surfaces the connection
// state from lib/connection.ts and, when writes are queued (React Query pauses
// mutations while offline and replays them on reconnect), how many are waiting —
// so a user on a flaky African mobile link knows their changes aren't lost.
//
// #751 phase 4 — always mounted now, open/closed via a height (grid-template-
// rows) + opacity collapse (.or-collapse, index.css) instead of returning
// null: the header used to jump when this banner mounted or unmounted with no
// transition at all. Held for a minimum of 2000ms once shown so a flapping
// connection cannot make the header strobe.

import { useEffect, useRef, useState } from 'react';
import { useMutationState } from '@tanstack/react-query';
import { CloudOff, RefreshCw } from 'lucide-react';
import { subscribeConnection, getConnectionStatus, type ConnectionState } from '../lib/connection';
import { useUIStore } from '../store/uiStore';

/** Once shown, stays up at least this long — matches the ux spec's "a
 *  flapping connection doesn't flicker" requirement. */
const MIN_VISIBLE_MS = 2000;

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
  const wants = state === 'offline' || (state === 'degraded' && pausedCount > 0);
  const offline = state === 'offline';

  // What to render is frozen the instant `wants` goes false, so the collapse
  // animates away the LAST real message rather than an empty shell.
  const [shown, setShown] = useState({ offline, pausedCount });
  if (wants && (shown.offline !== offline || shown.pausedCount !== pausedCount)) {
    setShown({ offline, pausedCount });
  }

  // Held open for MIN_VISIBLE_MS from the moment it first opens; re-opening
  // before that timer fires does not reset the clock, it just cancels the
  // pending close (open/held kept in sync at render time whenever `wants` is
  // true — see feedback_set-state-in-effect-lint.md for why the settle-back
  // timer itself lives in an effect and not here). The open timestamp is a
  // ref, not state: reading Date.now() during render is an impure call the
  // React Compiler's purity rule rejects, so it is captured in an effect
  // instead, which is exactly where an impure read belongs.
  const [held, setHeld] = useState(wants);
  if (wants && !held) setHeld(true);
  const openedAtRef = useRef(0);

  useEffect(() => {
    if (wants) openedAtRef.current = Date.now();
  }, [wants]);

  useEffect(() => {
    if (wants || !held) return;
    const remaining = Math.max(0, MIN_VISIBLE_MS - (Date.now() - openedAtRef.current));
    const timer = setTimeout(() => setHeld(false), remaining);
    return () => clearTimeout(timer);
  }, [wants, held]);

  return (
    <div data-open={held} aria-hidden={!held} className="or-collapse" data-testid="offline-banner">
      <div
        role="status"
        aria-live="polite"
        className="flex items-center justify-center gap-2 px-4 py-1.5 text-[12.5px] font-medium overflow-hidden"
        style={{
          background: shown.offline
            ? 'color-mix(in srgb,var(--critical) 16%,transparent)'
            : 'color-mix(in srgb,var(--medium) 16%,transparent)',
          color: shown.offline ? 'var(--critical)' : 'var(--medium)',
          borderBottom: '1px solid var(--border)',
        }}
      >
        {shown.offline ? <CloudOff size={14} /> : <RefreshCw size={14} />}
        <span>
          {shown.offline
            ? tr('Hors ligne', 'Offline')
            : tr('Connexion instable', 'Unstable connection')}
          {shown.pausedCount > 0 && (
            <>
              {' — '}
              {tr(
                `${shown.pausedCount} modification(s) en attente d'envoi`,
                `${shown.pausedCount} change(s) waiting to sync`,
              )}
            </>
          )}
          {shown.offline && shown.pausedCount === 0 && (
            <> — {tr('vos données restent consultables', 'your data stays viewable')}</>
          )}
        </span>
      </div>
    </div>
  );
}
