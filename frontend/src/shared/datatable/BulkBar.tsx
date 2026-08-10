// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Floating bulk-action bar.
//
// Two things it is strict about:
//  1. Scope is explicit. "The 50 on this page" and "all 1 284 results" are not
//     the same action, and a security tool must never let the user guess which
//     one it just ran. Ticking the header box selects the page; a banner then
//     offers the whole result set as a separate, deliberate choice.
//  2. Every action shows its own state. Pending → spinner + disabled, resolved →
//     tick, rejected → error label that stays until dismissed (rule (b)).

import { useState } from 'react';
import { Check, Loader2, TriangleAlert, X } from 'lucide-react';
import type { BulkAction, BulkScope } from './types';

type Phase = { kind: 'idle' } | { kind: 'running'; key: string } | { kind: 'done'; key: string } | { kind: 'error'; key: string };

interface BulkBarProps<T> {
  count: number;
  actions: BulkAction<T>[];
  buildScope: (action: BulkAction<T>) => BulkScope<T>;
  onClear: () => void;
  labels: { selected: (n: number) => string; clear: string; failed: string };
}

export function BulkBar<T>({ count, actions, buildScope, onClear, labels }: BulkBarProps<T>) {
  const [phase, setPhase] = useState<Phase>({ kind: 'idle' });
  const visible = actions.filter((a) => !a.hidden);
  if (count === 0 || visible.length === 0) return null;

  const run = async (action: BulkAction<T>) => {
    setPhase({ kind: 'running', key: action.key });
    try {
      await action.run(buildScope(action));
      setPhase({ kind: 'done', key: action.key });
      window.setTimeout(() => setPhase((p) => (p.kind === 'done' && p.key === action.key ? { kind: 'idle' } : p)), 1600);
    } catch {
      setPhase({ kind: 'error', key: action.key });
    }
  };

  return (
    <div
      role="toolbar"
      aria-label={labels.selected(count)}
      data-testid="bulk-bar"
      className="fixed bottom-6 left-1/2 z-[60] glass-strong rounded-[14px] shadow-card-lg px-3.5 py-2.5 flex items-center gap-3 flex-wrap"
      style={{ transform: 'translateX(-50%)', animation: 'or-fadeup .2s ease' }}
    >
      <span className="text-[13px] font-semibold text-ink" data-testid="bulk-count">{labels.selected(count)}</span>
      <span className="w-px h-5" style={{ background: 'var(--border-strong)' }} />

      {visible.map((action) => {
        const Icon = action.icon;
        const running = phase.kind === 'running' && phase.key === action.key;
        const done = phase.kind === 'done' && phase.key === action.key;
        const failed = phase.kind === 'error' && phase.key === action.key;
        return (
          <button
            key={action.key}
            type="button"
            onClick={() => run(action)}
            disabled={phase.kind === 'running'}
            data-testid={`bulk-action-${action.key}`}
            className="h-8 px-3 rounded-lg text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60"
            style={
              failed
                ? { background: 'color-mix(in srgb,var(--critical) 18%,transparent)', color: 'var(--critical)' }
                : action.danger
                  ? { background: 'color-mix(in srgb,var(--critical) 14%,transparent)', color: 'var(--critical)' }
                  : { background: 'var(--bg-hover)', color: 'var(--text-primary)' }
            }
          >
            {running ? <Loader2 size={14} className="animate-spin" /> : done ? <Check size={14} /> : failed ? <TriangleAlert size={14} /> : Icon ? <Icon size={14} /> : null}
            {failed ? labels.failed : action.label}
          </button>
        );
      })}

      <button
        type="button"
        onClick={onClear}
        aria-label={labels.clear}
        data-testid="bulk-clear"
        className="w-7 h-7 rounded-lg inline-flex items-center justify-center text-ink-muted hover:bg-hover"
      >
        <X size={15} />
      </button>
    </div>
  );
}
