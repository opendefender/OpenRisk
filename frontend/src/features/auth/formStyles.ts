// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Shared style constants for the auth forms.
//
// Split out of fields.tsx so that file exports only components: mixing
// components and constants in one module breaks React Fast Refresh, which the
// repo enforces via react-refresh/only-export-components.

export const inputCls =
  'w-full h-11 px-3.5 rounded-[11px] text-[14px] text-ink outline-none transition-colors focus:border-(--accent)';

/** Field chrome. `invalid` swaps the border to the critical token. */
export function inputStyle(invalid = false): React.CSSProperties {
  return {
    border: `1px solid ${invalid ? 'var(--critical)' : 'var(--border-strong)'}`,
    background: 'var(--bg-elevated)',
    // Well under the 400 ms ceiling: a field border that eases slowly feels
    // broken rather than smooth.
    transitionDuration: '150ms',
  };
}

export const primaryBtn =
  'w-full h-[46px] rounded-xl text-[14px] font-semibold text-fg-primary transition-opacity';

export const primaryStyle: React.CSSProperties = {
  background: 'var(--accent-solid)',
  color: 'var(--fg-on-solid)',
};
