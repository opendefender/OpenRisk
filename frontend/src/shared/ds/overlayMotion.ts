// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Modal and Drawer's exit-timer durations (#751 phase 5).
 *
 * `useExitTimer` keeps a panel mounted until a plain `setTimeout` fires, and
 * has no way to read a CSS custom property — so these hand-copied literals
 * have to match, by value, the exit transition each panel's own class
 * declares in index.css (`.or-modal-panel` / `.or-drawer-panel`). A
 * drift-guard test (`__tests__/overlayMotionTokens.test.ts`) reads the real
 * tokens from disk and fails if either literal falls out of sync — the same
 * convention `notifMotion.ts` established in phase 4.
 *
 * Kept out of Modal.tsx/Drawer.tsx rather than exported alongside the
 * component: `react-refresh/only-export-components` requires a component
 * file to export only the component (plus types, which are erased), so a
 * runtime constant needs its own module.
 */

/** Matches --dur-fast (src/styles/primitives.css) — Modal's exit, --motion-exit. */
export const MODAL_EXIT_MS = 120;

/**
 * Matches --dur-base (src/styles/primitives.css) — Drawer's exit. Slower than
 * the modal's --motion-exit (--dur-fast) on purpose: a panel that has just
 * travelled the width of the viewport reads as yanked off screen if it
 * disappears on the same fast timing as a small dialog's fade.
 */
export const DRAWER_EXIT_MS = 180;
