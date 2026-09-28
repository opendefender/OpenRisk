// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * SlotReel (#751 phase 3, D-060).
 *
 * The properties that matter are the ones a screenshot cannot catch: whether
 * the accessible/printable text is EVER an intermediate value (it must not
 * be, mid-roll or otherwise — there is no animated JS number in this
 * component to leak one from), and whether a plain mount or a same-value
 * refetch spuriously rolls (the #823-class trap: gating on mount rather than
 * on a genuine value change).
 *
 * These also stand in for the two deleted `useCountUp` copies' own tests
 * (`features/dashboard/__tests__/useCountUp.test.tsx`): the reduced-motion
 * guarantee they pinned is ported below as "reduced motion gives the final
 * value at once". Their second case — settling correctly when
 * `requestAnimationFrame` never advances — has no equivalent here on purpose:
 * SlotReel has no rAF loop to stall, which is the whole point of building it
 * on a CSS transform instead.
 */

import { describe, expect, it, vi, afterEach } from 'vitest';
import { act, render, screen } from '@testing-library/react';

import { SlotReel } from '../SlotReel';

function setReducedMotion(reduce: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: reduce && query.includes('prefers-reduced-motion'),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

afterEach(() => {
  vi.useRealTimers();
  setReducedMotion(false);
});

describe('SlotReel', () => {
  it('renders a plain, already-settled value on mount with data already present', () => {
    render(<SlotReel value={42} />);
    const reel = screen.getByTestId('slot-reel');
    // No roll was ever started: mount never triggers one.
    expect(reel).not.toHaveAttribute('data-rolling');
    expect(reel).toHaveAttribute('data-settled', 'true');
    expect(screen.getByTestId('slot-reel-value')).toHaveTextContent('42');
  });

  it('does not roll on a refetch that lands on the same value', () => {
    setReducedMotion(false);
    const { rerender } = render(<SlotReel value={7} />);
    expect(screen.getByTestId('slot-reel')).toHaveAttribute('data-settled', 'true');

    rerender(<SlotReel value={7} />);

    expect(screen.getByTestId('slot-reel')).not.toHaveAttribute('data-rolling');
    expect(screen.getByTestId('slot-reel')).toHaveAttribute('data-settled', 'true');
  });

  it('rolls old to new when the value changes, and settles back to true', () => {
    vi.useFakeTimers();
    setReducedMotion(false);
    const { rerender } = render(<SlotReel value={7} />);

    act(() => rerender(<SlotReel value={19} />));

    const reel = screen.getByTestId('slot-reel');
    expect(reel).toHaveAttribute('data-rolling', 'true');
    expect(reel).toHaveAttribute('data-settled', 'false');

    // The motion budget is --dur-panel (400ms fallback) + up to 3 staggered
    // --stagger-step (40ms fallback) = 520ms worst case.
    act(() => vi.advanceTimersByTime(600));

    expect(reel).not.toHaveAttribute('data-rolling');
    expect(reel).toHaveAttribute('data-settled', 'true');
  });

  it('keeps the accessible value final at every point, including mid-roll', () => {
    vi.useFakeTimers();
    setReducedMotion(false);
    const { rerender } = render(<SlotReel value={7} />);

    act(() => rerender(<SlotReel value={19} />));
    // Mid-roll: the reel is still animating, but the truth-carrying node
    // already reads the target, never a counted intermediate.
    expect(screen.getByTestId('slot-reel')).toHaveAttribute('data-rolling', 'true');
    expect(screen.getByTestId('slot-reel-value')).toHaveTextContent('19');

    act(() => vi.advanceTimersByTime(600));
    expect(screen.getByTestId('slot-reel-value')).toHaveTextContent('19');
  });

  it('gives the final value at once under prefers-reduced-motion, never rolling', () => {
    setReducedMotion(true);
    const { rerender } = render(<SlotReel value={7} />);

    rerender(<SlotReel value={19} />);

    const reel = screen.getByTestId('slot-reel');
    expect(reel).not.toHaveAttribute('data-rolling');
    expect(reel).toHaveAttribute('data-settled', 'true');
    expect(screen.getByTestId('slot-reel-value')).toHaveTextContent('19');
  });

  it('formats through Intl.NumberFormat and keeps separators static (not columns)', () => {
    render(<SlotReel value={1234} locale="en-US" />);
    expect(screen.getByTestId('slot-reel-value')).toHaveTextContent('1,234');
  });
});
