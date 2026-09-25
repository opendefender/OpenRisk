// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Profiler, useRef } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useNavGlide } from '../useNavGlide';

let renders = 0;

function Nav() {
  const backdropRef = useRef<HTMLDivElement>(null);
  const glide = useNavGlide(backdropRef);
  return (
    <nav data-testid="nav" {...glide.navProps}>
      {glide.enabled && <div ref={backdropRef} data-testid="nav-glide" data-visible="false" />}
      <a href="#risks" data-nav-entry="">
        Risks
      </a>
      <a href="#assets" data-nav-entry="" aria-current="page">
        Assets
      </a>
      <a href="#audit" data-nav-entry="">
        Audit
      </a>
    </nav>
  );
}

// jsdom lays nothing out: stack the entries 38px apart, in document order.
function stubLayout() {
  const entryIndex = (el: HTMLElement) =>
    Array.from(el.parentElement?.querySelectorAll('[data-nav-entry]') ?? []).indexOf(el);
  vi.spyOn(HTMLElement.prototype, 'offsetTop', 'get').mockImplementation(function (
    this: HTMLElement,
  ) {
    return Math.max(0, entryIndex(this)) * 38;
  });
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(36);
  vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(220);
}

function mockMedia(matching: string[]) {
  vi.mocked(window.matchMedia).mockImplementation(
    (query: string) =>
      ({
        matches: matching.includes(query),
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }) as unknown as MediaQueryList,
  );
}

describe('useNavGlide', () => {
  beforeEach(() => {
    renders = 0;
    mockMedia([]);
    stubLayout();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('is not rendered at all under reduced motion', () => {
    mockMedia(['(prefers-reduced-motion: reduce)']);
    render(<Nav />);
    expect(screen.queryByTestId('nav-glide')).not.toBeInTheDocument();
  });

  it('is not rendered on a touch screen, where there is no hover to follow', () => {
    mockMedia(['(pointer: coarse)']);
    render(<Nav />);
    expect(screen.queryByTestId('nav-glide')).not.toBeInTheDocument();
  });

  it('moves under the entry the pointer is on, and leaves with the pointer', () => {
    render(<Nav />);
    const backdrop = screen.getByTestId('nav-glide');

    fireEvent.pointerOver(screen.getByText('Audit'));
    expect(backdrop.dataset.visible).toBe('true');
    expect(backdrop.style.transform).toBe('translateY(76px)');
    expect(backdrop.style.height).toBe('36px');

    fireEvent.pointerOver(screen.getByText('Risks'));
    expect(backdrop.style.transform).toBe('translateY(0px)');

    fireEvent.pointerLeave(screen.getByTestId('nav'));
    expect(backdrop.dataset.visible).toBe('false');
  });

  it('steps aside on the current page, which has its own accent background', () => {
    render(<Nav />);
    fireEvent.pointerOver(screen.getByText('Risks'));
    fireEvent.pointerOver(screen.getByText('Assets'));
    expect(screen.getByTestId('nav-glide').dataset.visible).toBe('false');
  });

  // jsdom does not compute :focus-visible, so the test decides what the
  // browser would: true after a Tab, false after a click.
  function stubFocusVisible(visible: boolean) {
    const matches = Element.prototype.matches;
    vi.spyOn(Element.prototype, 'matches').mockImplementation(function (
      this: Element,
      selector: string,
    ) {
      return selector === ':focus-visible' ? visible : matches.call(this, selector);
    });
  }

  it('follows keyboard focus the way it follows the pointer', async () => {
    stubFocusVisible(true);
    const user = userEvent.setup();
    render(<Nav />);
    await user.tab();
    const backdrop = screen.getByTestId('nav-glide');
    expect(backdrop.dataset.visible).toBe('true');
    expect(backdrop.style.transform).toBe('translateY(0px)');

    await user.tab(); // Assets, the current page
    await user.tab(); // Audit
    expect(backdrop.style.transform).toBe('translateY(76px)');

    await user.tab(); // out of the nav
    expect(backdrop.dataset.visible).toBe('false');
  });

  it('ignores the focus a click leaves behind', () => {
    stubFocusVisible(false);
    render(<Nav />);
    screen.getByText('Audit').focus();
    expect(screen.getByTestId('nav-glide').dataset.visible).toBe('false');
  });

  it('never re-renders while the pointer sweeps the nav', () => {
    // Profiler counts commits without the component having to count itself.
    render(
      <Profiler id="nav" onRender={() => (renders += 1)}>
        <Nav />
      </Profiler>,
    );
    const before = renders;
    for (const label of ['Risks', 'Audit', 'Risks', 'Audit']) {
      fireEvent.pointerOver(screen.getByText(label));
    }
    fireEvent.pointerLeave(screen.getByTestId('nav'));
    expect(renders).toBe(before);
  });
});
