// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// useCountUp is decoration over a real number. These tests pin the guarantee
// that matters (audit-2026 #242): the figure it lands on is the DATA, and it
// must be shown even when the count-up animation cannot or should not run —
// reduced-motion, or an environment where requestAnimationFrame does not fire.

import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useCountUp } from '../shared';

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('useCountUp', () => {
  it('shows the target immediately under prefers-reduced-motion (no animation from 0)', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: true,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    );
    const { result } = renderHook(() => useCountUp(12));
    expect(result.current).toBe(12);
  });

  it('settles on the target when requestAnimationFrame never advances', () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: false,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }),
    );
    // rAF that never invokes its callback (models a non-composited / background
    // page). The displayed value must still be the real number, not a frozen 0.
    vi.stubGlobal('requestAnimationFrame', undefined as unknown as typeof requestAnimationFrame);
    const { result } = renderHook(() => useCountUp(57));
    expect(result.current).toBe(57);
  });
});
