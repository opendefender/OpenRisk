// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, it, vi } from 'vitest';
import { useRef, useState } from 'react';
import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Collapse } from '../Collapse';
import { ScrollProgress } from '../ScrollProgress';

/* ---------------------------------------------------------------- Collapse -- */

function CollapseHarness({ initiallyOpen = false }: { initiallyOpen?: boolean }) {
  const [open, setOpen] = useState(initiallyOpen);
  return (
    <>
      <button aria-expanded={open} aria-controls="c" onClick={() => setOpen((o) => !o)}>
        3 decisions
      </button>
      <Collapse id="c" open={open}>
        <a href="#x">Decision detail</a>
      </Collapse>
    </>
  );
}

describe('Collapse', () => {
  it('does not mount its content before the first open', () => {
    // A list of fifty collapsed rows must not render fifty hidden panels.
    render(<CollapseHarness />);
    expect(screen.queryByText('Decision detail')).not.toBeInTheDocument();
  });

  it('opens under the trigger that controls it', async () => {
    const user = userEvent.setup();
    render(<CollapseHarness />);
    await user.click(screen.getByRole('button', { name: '3 decisions' }));

    const panel = document.getElementById('c');
    expect(panel).toHaveAttribute('data-state', 'open');
    expect(panel).toHaveStyle({ gridTemplateRows: '1fr' });
    expect(screen.getByRole('link', { name: 'Decision detail' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '3 decisions' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('keeps closed content in the DOM but out of reach', async () => {
    // Kept so the close can animate; inert so it is neither focusable nor read.
    const user = userEvent.setup();
    render(<CollapseHarness initiallyOpen />);
    await user.click(screen.getByRole('button', { name: '3 decisions' }));

    const panel = document.getElementById('c');
    expect(panel).toHaveAttribute('data-state', 'closed');
    expect(panel).toHaveStyle({ gridTemplateRows: '0fr' });
    expect(panel).toHaveAttribute('inert');
    expect(panel).toHaveAttribute('aria-hidden', 'true');
    expect(screen.queryByRole('link', { name: 'Decision detail' })).not.toBeInTheDocument();
  });
});

/* ---------------------------------------------------------- ScrollProgress -- */

function ScrollHarness() {
  const ref = useRef<HTMLDivElement>(null);
  return (
    <>
      <ScrollProgress target={ref} />
      <div ref={ref} data-testid="body">
        <p>Long form</p>
      </div>
    </>
  );
}

function stubScroll(el: HTMLElement, box: { scrollHeight: number; clientHeight: number }) {
  Object.defineProperty(el, 'scrollHeight', { configurable: true, value: box.scrollHeight });
  Object.defineProperty(el, 'clientHeight', { configurable: true, value: box.clientHeight });
}

describe('ScrollProgress', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('stays hidden when the content does not overflow', () => {
    // jsdom lays nothing out, so every box is 0 tall: no overflow.
    render(<ScrollHarness />);
    expect(screen.getByTestId('scroll-progress').style.opacity).toBe('0');
  });

  it('fills in proportion to how far the content has been scrolled', () => {
    vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      cb(0);
      return 1;
    });
    render(<ScrollHarness />);
    const body = screen.getByTestId('body');
    stubScroll(body, { scrollHeight: 1000, clientHeight: 600 });

    act(() => {
      body.scrollTop = 100;
      body.dispatchEvent(new Event('scroll'));
    });

    const bar = screen.getByTestId('scroll-progress');
    expect(bar.style.opacity).toBe('1');
    expect(bar.style.transform).toBe('scaleX(0.25)');
  });
});
