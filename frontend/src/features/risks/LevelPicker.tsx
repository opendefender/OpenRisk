// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The 5-level picker of probability and impact (drawer and new-risk modal, #913).
// Level k writes the midpoint of its bucket (see riskLevels.ts); the pressed
// level is the bucket the CURRENT value falls in, and the exact value is always
// printed beside it, so a level never hides what was stored.

import type { ReactNode } from 'react';
import { LEVELS } from './riskLevels';
import type { MatrixBucket } from '../../shared/ds';

interface LevelPickerProps {
  /** Visible label, also the group's accessible name. */
  label: string;
  /** The bucket the current value falls in. */
  level: MatrixBucket;
  /** Names of levels 1..5, index 0 = level 1. */
  levelNames: readonly string[];
  /** The exact stored value, already formatted ("0,62"). */
  exact: string;
  onPick: (level: MatrixBucket) => void;
  /** Sits right of the label — the field's contextual help, where there is one. */
  help?: ReactNode;
  disabled?: boolean;
  testId?: string;
}

export function LevelPicker({
  label,
  level,
  levelNames,
  exact,
  onPick,
  help,
  disabled = false,
  testId,
}: LevelPickerProps) {
  return (
    <div role="group" aria-label={label} className="grid gap-1.5" data-testid={testId}>
      <span className="flex items-center gap-1.5 text-[12px] font-semibold text-ink-soft">
        {label}
        {help}
      </span>
      <div className="flex gap-1">
        {LEVELS.map((k) => {
          const on = k === level;
          const name = levelNames[k - 1] ?? String(k);
          return (
            <button
              key={k}
              type="button"
              aria-pressed={on}
              aria-label={`${label} ${k} — ${name}`}
              title={name}
              disabled={disabled}
              onClick={() => {
                if (!on) onPick(k);
              }}
              data-testid={testId ? `${testId}-${k}` : undefined}
              className={`mono h-8 flex-1 rounded-lg border text-[13px] font-semibold transition-colors disabled:opacity-60 motion-safe:active:translate-y-px ${
                on
                  ? 'border-accent bg-accent-soft text-accent-strong'
                  : 'border-border-default bg-surface-2 text-ink-soft hover:border-border-strong'
              }`}
            >
              {k}
            </button>
          );
        })}
      </div>
      <span className="text-[11.5px] text-ink-muted">
        {levelNames[level - 1] ?? ''} · <span className="mono">{exact}</span>
      </span>
    </div>
  );
}
