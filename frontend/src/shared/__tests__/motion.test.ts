// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, describe, expect, it, vi } from 'vitest';
// eslint-disable-next-line no-restricted-imports -- the test asserts on framer-motion's own global
import { MotionGlobalConfig } from 'framer-motion';
import { syncReducedMotion } from '../motion';

type Listener = (e: MediaQueryListEvent) => void;

function mockPreference(matches: boolean): { flip: (next: boolean) => void } {
  let listener: Listener | undefined;
  vi.mocked(window.matchMedia).mockImplementation(
    (query: string) =>
      ({
        matches,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: (_: string, cb: Listener) => {
          listener = cb;
        },
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }) as unknown as MediaQueryList,
  );
  return {
    flip: (next) => listener?.({ matches: next } as MediaQueryListEvent),
  };
}

describe('shared/motion reduced-motion policy', () => {
  afterEach(() => {
    MotionGlobalConfig.skipAnimations = false;
  });

  it('skips every framer-motion animation when the OS asks for reduced motion', () => {
    mockPreference(true);
    syncReducedMotion();
    expect(MotionGlobalConfig.skipAnimations).toBe(true);
  });

  it('leaves animations on when the preference is not set', () => {
    mockPreference(false);
    syncReducedMotion();
    expect(MotionGlobalConfig.skipAnimations).toBe(false);
  });

  it('follows the preference when it changes mid-session', () => {
    const pref = mockPreference(false);
    syncReducedMotion();
    pref.flip(true);
    expect(MotionGlobalConfig.skipAnimations).toBe(true);
    pref.flip(false);
    expect(MotionGlobalConfig.skipAnimations).toBe(false);
  });
});
