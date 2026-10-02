// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { useRef } from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  TILT_MAX_DEG,
  TILT_QUERY,
  tiltFor,
  tiltTransform,
  useScoreCardTilt,
} from '../useScoreCardTilt';

const RECT = { left: 100, top: 50, width: 340, height: 300 };

function Card() {
  const ref = useRef<HTMLDivElement>(null);
  useScoreCardTilt(ref);
  return <div ref={ref} data-testid="card" />;
}

/** Matches TILT_QUERY only when the device is a hovering mouse with motion allowed. */
function setDevice({ fine, reduce }: { fine: boolean; reduce: boolean }) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query === TILT_QUERY && fine && !reduce,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

function mountCard() {
  render(<Card />);
  const card = screen.getByTestId('card');
  card.getBoundingClientRect = () =>
    ({ ...RECT, right: 440, bottom: 350, x: 100, y: 50 }) as DOMRect;
  return card;
}

function move(card: HTMLElement, clientX: number, clientY: number, pointerType = 'mouse') {
  fireEvent.pointerMove(card, { clientX, clientY, pointerType });
}

let frames: FrameRequestCallback[] = [];
beforeEach(() => {
  frames = [];
  vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
    frames.push(cb);
    return frames.length;
  });
  vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {});
});
afterEach(() => {
  vi.restoreAllMocks();
});
const flushFrame = () =>
  act(() => {
    const pending = frames;
    frames = [];
    pending.forEach((cb) => cb(0));
  });

describe('tiltFor — the 2° clamp', () => {
  it('is flat at the centre', () => {
    expect(tiltFor(270, 200, RECT)).toEqual({ rotateX: 0, rotateY: 0 });
  });

  it('reaches exactly 2° per axis at each corner', () => {
    expect(tiltFor(100, 50, RECT)).toEqual({ rotateX: TILT_MAX_DEG, rotateY: -TILT_MAX_DEG });
    expect(tiltFor(440, 50, RECT)).toEqual({ rotateX: TILT_MAX_DEG, rotateY: TILT_MAX_DEG });
    expect(tiltFor(100, 350, RECT)).toEqual({ rotateX: -TILT_MAX_DEG, rotateY: -TILT_MAX_DEG });
    expect(tiltFor(440, 350, RECT)).toEqual({ rotateX: -TILT_MAX_DEG, rotateY: TILT_MAX_DEG });
  });

  it('never exceeds 2° for a pointer beyond the edges', () => {
    const far = tiltFor(10_000, -10_000, RECT);
    expect(far).toEqual({ rotateX: TILT_MAX_DEG, rotateY: TILT_MAX_DEG });
  });

  it('is flat for a card with no size', () => {
    expect(tiltFor(5, 5, { left: 0, top: 0, width: 0, height: 0 })).toEqual({
      rotateX: 0,
      rotateY: 0,
    });
  });

  it('prints an 800px perspective', () => {
    expect(tiltTransform({ rotateX: 2, rotateY: -2 })).toBe(
      'perspective(800px) rotateX(2.00deg) rotateY(-2.00deg)',
    );
  });
});

describe('useScoreCardTilt', () => {
  it('tilts under a hovering mouse, once per frame, and goes flat on leave', () => {
    setDevice({ fine: true, reduce: false });
    const card = mountCard();

    move(card, 440, 50);
    move(card, 430, 60);
    move(card, 440, 50);
    expect(frames).toHaveLength(1); // three events, one frame
    flushFrame();
    expect(card.style.transform).toBe('perspective(800px) rotateX(2.00deg) rotateY(2.00deg)');

    fireEvent.pointerLeave(card);
    expect(card.style.transform).toBe('');
  });

  it('has no transform and no listener under reduced motion', () => {
    setDevice({ fine: true, reduce: true });
    const card = mountCard();
    move(card, 440, 50);
    flushFrame();
    expect(frames).toHaveLength(0);
    expect(card.style.transform).toBe('');
    expect(getComputedStyle(card).transform).not.toMatch(/rotate|matrix/);
  });

  it('has no transform under a coarse pointer', () => {
    setDevice({ fine: false, reduce: false });
    const card = mountCard();
    move(card, 440, 50);
    flushFrame();
    expect(card.style.transform).toBe('');
  });

  it('ignores touch input on a device that also has a mouse', () => {
    setDevice({ fine: true, reduce: false });
    const card = mountCard();
    move(card, 440, 50, 'touch');
    flushFrame();
    expect(card.style.transform).toBe('');
  });

  it('does not tilt on keyboard focus', () => {
    setDevice({ fine: true, reduce: false });
    const card = mountCard();
    fireEvent.focus(card);
    fireEvent.keyDown(card, { key: 'Tab' });
    expect(card.style.transform).toBe('');
  });

  it('removes its listeners on unmount', () => {
    setDevice({ fine: true, reduce: false });
    const { unmount } = render(<Card />);
    const card = screen.getByTestId('card');
    const remove = vi.spyOn(card, 'removeEventListener');
    unmount();
    expect(remove).toHaveBeenCalledWith('pointermove', expect.any(Function));
    expect(remove).toHaveBeenCalledWith('pointerleave', expect.any(Function));
  });
});
