// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * `useAnimeScope` (#445, D-028).
 *
 * The three properties worth testing are the three that go wrong silently: a
 * scope that is never reverted leaks across StrictMode's double mount, a
 * reduced-motion user who still gets animation, and — the one a screenshot would
 * never catch — a reduced-motion user who gets NO animation and therefore an
 * empty chart, because the component hid its content waiting for a draw that was
 * never going to run.
 */

import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { render, act } from '@testing-library/react';
import { useAnimeScope } from '../useAnimeScope';

const revert = vi.fn();
const scopeAdd = vi.fn((cb: () => void) => {
  cb();
  return { revert };
});
const createScope = vi.fn(() => ({ add: scopeAdd, revert }));

vi.mock('animejs', () => ({
  createScope: (...args: unknown[]) => createScope(...(args as [])),
  animate: vi.fn(),
  stagger: vi.fn(() => 0),
}));

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

function Probe({ onSetup }: { onSetup: () => void }) {
  const { ref, willAnimate } = useAnimeScope<SVGSVGElement>(onSetup);
  return (
    <svg ref={ref} data-testid="root">
      <rect data-testid="mark" width={willAnimate ? 0 : 100} />
    </svg>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  setReducedMotion(false);
});
afterEach(() => setReducedMotion(false));

describe('useAnimeScope', () => {
  it('creates a scope and runs setup when motion is allowed', () => {
    const onSetup = vi.fn();
    render(<Probe onSetup={onSetup} />);
    expect(createScope).toHaveBeenCalledTimes(1);
    expect(onSetup).toHaveBeenCalledTimes(1);
  });

  it('reverts the scope on unmount, so StrictMode cannot leak it', () => {
    const { unmount } = render(<Probe onSetup={vi.fn()} />);
    expect(revert).not.toHaveBeenCalled();
    act(() => unmount());
    expect(revert).toHaveBeenCalled();
  });

  it('does not run at all under prefers-reduced-motion', () => {
    setReducedMotion(true);
    const onSetup = vi.fn();
    render(<Probe onSetup={onSetup} />);
    // Not "a shorter animation" — no anime.js code runs.
    expect(createScope).not.toHaveBeenCalled();
    expect(onSetup).not.toHaveBeenCalled();
  });

  it('reports willAnimate=false under reduced motion so callers render the FINAL state', () => {
    setReducedMotion(true);
    const { getByTestId } = render(<Probe onSetup={vi.fn()} />);
    // The guarantee that stops "reduced motion" meaning "empty canvas".
    expect(getByTestId('mark').getAttribute('width')).toBe('100');
  });

  it('reports willAnimate=true normally, so callers render the initial state', () => {
    const { getByTestId } = render(<Probe onSetup={vi.fn()} />);
    expect(getByTestId('mark').getAttribute('width')).toBe('0');
  });
});
