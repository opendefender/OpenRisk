// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Form primitives for the auth screens. Style constants live in formStyles.ts
// so this module exports components only (React Fast Refresh).

import { AlertCircle } from 'lucide-react';

// Promoted into the design system (#751 phase 2): shared/ds/Field now hosts
// the same shake for every form, not only these auth screens. Re-exported
// here so the six existing `import { Shake } from './fields'` call sites keep
// working unchanged.
export { Shake } from '../../shared/ds/Shake';

export function Label({ htmlFor, children }: { htmlFor?: string; children: React.ReactNode }) {
  return (
    <label htmlFor={htmlFor} className="block text-[12.5px] font-medium text-ink-soft mb-[7px]">
      {children}
    </label>
  );
}

/** Inline error banner. `role="alert"` so screen readers announce it. */
export function ErrorBanner({ children }: { children: React.ReactNode }) {
  if (!children) return null;
  return (
    <div
      role="alert"
      data-testid="auth-error"
      className="flex items-start gap-2 mb-4 px-3 py-2.5 rounded-[11px] text-[12.5px] leading-snug"
      style={{
        // 7%, not 10%: at 10% the 12.5px text measured 4.46:1 on the light
        // theme, under the 4.5:1 floor (axe color-contrast, #872). 7% gives
        // 4.68:1 light and 5.20:1 dark.
        background: 'color-mix(in srgb, var(--critical) 7%, transparent)',
        border: '1px solid color-mix(in srgb, var(--critical) 35%, transparent)',
        color: 'var(--critical)',
      }}
    >
      <AlertCircle size={15} className="shrink-0 mt-px" />
      <span>{children}</span>
    </div>
  );
}

/** Success banner, same shape as ErrorBanner. */
export function SuccessBanner({ children }: { children: React.ReactNode }) {
  if (!children) return null;
  return (
    <div
      role="status"
      data-testid="auth-success"
      className="flex items-start gap-2 mb-4 px-3 py-2.5 rounded-[11px] text-[12.5px] leading-snug"
      style={{
        background: 'color-mix(in srgb, var(--low) 10%, transparent)',
        border: '1px solid color-mix(in srgb, var(--low) 35%, transparent)',
        color: 'var(--low)',
      }}
    >
      <span>{children}</span>
    </div>
  );
}
