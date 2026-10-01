// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0
//
// #751 phase 5 — MODAL_EXIT_MS and DRAWER_EXIT_MS are hand-copied ms literals
// that claim to match --dur-fast / --dur-base (src/styles/primitives.css).
// Nothing enforces that at compile time, so a later change to either token
// would silently desync the exit timer from the CSS transition it is meant
// to outlast. This reads the real tokens from disk and pins both constants to
// them — the same convention exitTimerTokens.test.ts established in phase 4.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, it, expect } from 'vitest';

import { MODAL_EXIT_MS, DRAWER_EXIT_MS } from '../overlayMotion';

function tokenMs(name: string): number {
  const css = readFileSync(path.resolve(__dirname, '../../../styles/primitives.css'), 'utf8');
  const match = css.match(new RegExp(`--${name}:\\s*([\\d.]+)ms`));
  if (!match) throw new Error(`--${name} not found in primitives.css — did the token get renamed?`);
  return Number(match[1]);
}

describe('overlay exit-timer constants track their CSS tokens (#751 phase 5)', () => {
  it('MODAL_EXIT_MS equals --dur-fast, which .or-modal-panel exits on', () => {
    expect(MODAL_EXIT_MS).toBe(tokenMs('dur-fast'));
  });

  it('DRAWER_EXIT_MS equals --dur-base, which .or-drawer-panel exits on', () => {
    expect(DRAWER_EXIT_MS).toBe(tokenMs('dur-base'));
  });
});
