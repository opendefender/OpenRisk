// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Form primitives for the auth screens. Style constants live in formStyles.ts
// so this module exports components only (React Fast Refresh).

import { AlertCircle } from 'lucide-react';

export function Label({ htmlFor, children }: { htmlFor?: string; children: React.ReactNode }) {
  return (
    <label htmlFor={htmlFor} className="block text-[12.5px] font-medium text-ink-soft mb-[7px]">
      {children}
    </label>
  );
}

/**
 * Shakes its children once whenever `errorKey` changes to a new truthy value.
 *
 * Implemented by using errorKey as the React `key`: a changed key remounts the
 * wrapper, and a fresh mount restarts the CSS animation. That is why callers
 * pass an incrementing nonce rather than the message itself — a SECOND failure
 * with identical wording still has to shake, or someone retyping the same wrong
 * password gets no feedback at all and concludes the button is dead.
 *
 * Deriving the animation from the key rather than from an effect also keeps this
 * render-pure: no state, no effect, nothing to get out of step.
 *
 * The shake is decoration layered on top of the message and the red border.
 * Under `prefers-reduced-motion` the global CSS rule strips the animation and
 * the other two still carry the meaning.
 */
export function Shake({ errorKey, children }: { errorKey: string | number; children: React.ReactNode }) {
  return (
    <div key={String(errorKey)} style={errorKey ? { animation: 'or-shake 300ms ease' } : undefined}>
      {children}
    </div>
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
        background: 'color-mix(in srgb, var(--critical) 10%, transparent)',
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
