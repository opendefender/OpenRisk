// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0
//
// #751 phase 4, review fix (motion-designer) — NOTIF_EXIT_MS and
// MFA_DISMISS_EXIT_MS are hand-copied ms literals that claim to match
// --dur-fast (src/styles/primitives.css). Nothing enforces that at compile
// time, so a later change to the token would silently desync the two exit
// timers from it. This reads the real token from disk and pins both
// constants to it, rather than to another hardcoded 120.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, it, expect } from 'vitest';

import { NOTIF_EXIT_MS } from '../../../components/layout/AppHeader';
import { MFA_DISMISS_EXIT_MS } from '../../../features/auth/MFAEnrollmentBanner';

function durFastMs(): number {
  const css = readFileSync(path.resolve(__dirname, '../../../styles/primitives.css'), 'utf8');
  const match = css.match(/--dur-fast:\s*([\d.]+)ms/);
  if (!match)
    throw new Error('--dur-fast not found in primitives.css — did the token get renamed?');
  return Number(match[1]);
}

describe('exit-timer constants track --dur-fast (#751 phase 4 review fix)', () => {
  it('NOTIF_EXIT_MS equals the real --dur-fast token', () => {
    expect(NOTIF_EXIT_MS).toBe(durFastMs());
  });

  it('MFA_DISMISS_EXIT_MS equals the real --dur-fast token', () => {
    expect(MFA_DISMISS_EXIT_MS).toBe(durFastMs());
  });
});
