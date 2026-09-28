// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * `RadialGauge`'s arc reveal (#751 phase 3 review, "for consistency" with
 * `ScoreGauge`). No live caller renders this component today, but its
 * `fresh` contract must match `ScoreGauge`'s exactly, since both wrap the
 * same `useArcReveal` — see score.test.tsx's `ScoreGauge` suite for the
 * fuller coverage this mirrors.
 */

import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, render } from '@testing-library/react';

import { RadialGauge } from '../ui';

afterEach(() => {
  vi.useRealTimers();
});

describe('RadialGauge — arc reveal', () => {
  it('fresh: undraws first, then draws to the final value', () => {
    vi.useFakeTimers();
    const { container } = render(<RadialGauge value={25} fresh />);
    const arc = container.querySelector('[data-testid="radial-gauge-arc"]');
    expect(arc).toHaveAttribute('stroke-dashoffset', '1');

    act(() => vi.advanceTimersByTime(0));
    // 25/100 == 0.25
    expect(arc).toHaveAttribute('stroke-dashoffset', '0.75');
  });

  it('cached (no fresh prop): renders the final arc immediately', () => {
    const { container } = render(<RadialGauge value={25} />);
    const arc = container.querySelector('[data-testid="radial-gauge-arc"]');
    expect(arc).toHaveAttribute('stroke-dashoffset', '0.75');
  });

  it('countUp=false ignores fresh entirely — never undrawn', () => {
    const { container } = render(<RadialGauge value={25} countUp={false} fresh />);
    const arc = container.querySelector('[data-testid="radial-gauge-arc"]');
    expect(arc).toHaveAttribute('stroke-dashoffset', '0.75');
  });
});
