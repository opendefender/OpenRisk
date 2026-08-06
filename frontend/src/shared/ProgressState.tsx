// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Informative waiting state (UX-09): for operations that naturally take > 1.5s
// (a scan, a PDF/AI report, a CRQ computation), don't show a bare spinner — show
// the step in progress and a useful stat, turning dead time into anticipation and
// perceived value.

import { type ReactNode } from 'react';

interface ProgressStateProps {
  title: string;
  /** Ordered step labels; `activeStep` (0-based) is highlighted, earlier = done. */
  steps?: string[];
  activeStep?: number;
  /** A useful figure to surface during the wait (e.g. "142 actifs analysés"). */
  stat?: ReactNode;
  /** 0–100 determinate progress; omit for indeterminate shimmer. */
  percent?: number;
}

export function ProgressState({ title, steps = [], activeStep = 0, stat, percent }: ProgressStateProps) {
  return (
    <div className="flex flex-col items-center justify-center text-center py-12 px-6" role="status" aria-live="polite">
      <div className="w-full max-w-[360px]">
        <div className="text-[15px] font-semibold text-ink mb-1">{title}</div>
        {stat != null && <div className="text-[13px] text-ink-soft mb-4">{stat}</div>}

        <div className="h-1.5 rounded-full overflow-hidden mb-4" style={{ background: 'var(--bg-hover)' }}>
          {percent == null ? (
            <div className="h-full or-skeleton" style={{ width: '100%' }} />
          ) : (
            <div
              className="h-full rounded-full"
              style={{ width: `${Math.min(100, Math.max(0, percent))}%`, background: 'linear-gradient(90deg,var(--accent),var(--accent-2))', transition: 'width .4s var(--ease-out, ease)' }}
            />
          )}
        </div>

        {steps.length > 0 && (
          <div className="flex flex-col gap-1.5 text-left">
            {steps.map((s, i) => {
              const done = i < activeStep;
              const active = i === activeStep;
              return (
                <div key={i} className="flex items-center gap-2.5 text-[12.5px]" style={{ color: active ? 'var(--text-primary)' : done ? 'var(--text-secondary)' : 'var(--text-muted)' }}>
                  <span
                    className="w-4 h-4 rounded-full flex items-center justify-center text-[9px] shrink-0"
                    style={{ background: done ? 'var(--low)' : active ? 'var(--accent)' : 'var(--bg-hover)', color: done || active ? 'var(--text-inverse)' : 'var(--text-muted)' }}
                  >
                    {done ? '✓' : i + 1}
                  </span>
                  <span style={{ fontWeight: active ? 600 : 400 }}>{s}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
