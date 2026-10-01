// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — verified against sonner's REAL rendered DOM (v2.0.7, no
// mock), never assumed. The ux spec explicitly named two library claims to
// verify rather than take on faith: role/aria-live per toast type, and pause
// on keyboard focus. Both are checked here, and both come back different
// from what the spec assumed — see the last two tests, which document the
// verified (not the hoped-for) behaviour. This phase restyles sonner rather
// than forking it, so neither gap is fixed here; it is reported.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { toast } from 'sonner';

import ThemedToaster from '../ThemedToaster';
import { useUIStore } from '../../../store/uiStore';

function toastRoot(text: string) {
  return screen.getByText(text).closest('[data-sonner-toast]') as HTMLElement;
}

describe('ThemedToaster (#751 phase 4)', () => {
  beforeEach(() => {
    useUIStore.setState({ theme: 'dark' });
  });

  afterEach(() => {
    act(() => {
      toast.dismiss();
    });
    vi.useRealTimers();
  });

  it('drops richColors: no toast carries data-rich-colors="true"', async () => {
    render(<ThemedToaster />);
    act(() => {
      toast.success('done');
    });
    expect(await screen.findByText('done')).toBeInTheDocument();
    expect(toastRoot('done')).not.toHaveAttribute('data-rich-colors', 'true');
  });

  it('positions bottom-right, 16px offset', async () => {
    render(<ThemedToaster />);
    // The per-position <ol data-sonner-toaster> only renders once there is a
    // toast to show, so fire one rather than asserting on an empty toaster.
    act(() => {
      toast.info('where am i');
    });
    await screen.findByText('where am i');
    const toaster = document.querySelector('[data-sonner-toaster]') as HTMLElement;
    expect(toaster).toHaveAttribute('data-x-position', 'right');
    expect(toaster).toHaveAttribute('data-y-position', 'bottom');
    expect(toaster.style.getPropertyValue('--offset-bottom')).toBe('16px');
    expect(toaster.style.getPropertyValue('--offset-right')).toBe('16px');
  });

  it('applies the token classNames to the toast surface, title and description', async () => {
    render(<ThemedToaster />);
    act(() => {
      toast.success('styled', { description: 'the detail' });
    });
    await screen.findByText('styled');
    const root = toastRoot('styled');
    expect(root).toHaveClass('or-toast');
    expect(root.querySelector('[data-title]')).toHaveClass('or-toast-title');
    expect(root.querySelector('[data-description]')).toHaveClass('or-toast-description');
  });

  it('gives success and error their own distinct icon — never colour alone', async () => {
    render(<ThemedToaster />);
    act(() => {
      toast.success('ok');
      toast.error('bad');
    });
    await screen.findByText('ok');
    await screen.findByText('bad');
    const okIcon = toastRoot('ok').querySelector('[data-icon] svg');
    const badIcon = toastRoot('bad').querySelector('[data-icon] svg');
    expect(okIcon).toBeTruthy();
    expect(badIcon).toBeTruthy();
    expect(okIcon?.getAttribute('class')).not.toEqual(badIcon?.getAttribute('class'));
  });

  it('caps simultaneous toasts at 3 visible', async () => {
    render(<ThemedToaster />);
    act(() => {
      toast.info('one');
      toast.info('two');
      toast.info('three');
      toast.info('four');
    });
    await screen.findByText('four');
    const visible = document.querySelectorAll('[data-sonner-toast][data-visible="true"]');
    expect(visible.length).toBe(3);
  });

  it('VERIFIED (not as the spec assumed): one shared polite region, no per-type role or assertive error', async () => {
    // p4-ux.md: "sonner assigns role=status/polite to success/info/warning
    // and role=alert/assertive to error by default. Verify this in the
    // rendered DOM before shipping." Read from node_modules/sonner/dist/
    // index.mjs (2.0.7): the outer <section> carries a single fixed
    // aria-live="polite" for every toast type, and no toast — including
    // error — ever gets role="alert". The assumption was wrong; this test
    // pins down what is actually there instead.
    render(<ThemedToaster />);
    act(() => {
      toast.error('urgent problem');
    });
    await screen.findByText('urgent problem');

    const liveRegion = document.querySelector('[aria-live]') as HTMLElement;
    expect(liveRegion).toHaveAttribute('aria-live', 'polite');
    expect(toastRoot('urgent problem')).not.toHaveAttribute('role', 'alert');
    expect(document.querySelector('[role="alert"]')).toBeNull();
  });

  it('VERIFIED (not as the spec assumed): hover pauses the auto-dismiss timer, keyboard focus on the action does not', async () => {
    // p4-ux.md: "Pause on hover/focus: built into sonner's toast primitive;
    // verify with a manual keyboard-focus test... do not report this 'done'
    // without that check." Read from the same source: onMouseEnter/
    // onPointerDown flip the state that pauses the close timer; the
    // toaster's onFocus handler only remembers where to return focus later
    // (its own Escape-close feature) and never touches that state. No
    // focusin listener exists anywhere in the package. This is a real,
    // pre-existing gap in the dependency (issue-worthy for #751's own
    // backlog), not something this "restyle, don't replace" phase forks
    // sonner to fix.
    vi.useFakeTimers();
    render(<ThemedToaster />);
    act(() => {
      toast.success('hover test', { duration: 1000 });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const hoverRoot = toastRoot('hover test');

    fireEvent.mouseEnter(hoverRoot);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    // Positive control: hover DOES pause it — proves the fake-timer harness
    // itself is exercising the real dismiss timer, not a no-op.
    expect(screen.queryByText('hover test')).toBeInTheDocument();
    fireEvent.mouseLeave(hoverRoot);

    act(() => {
      toast.success('focus test', { duration: 1000 });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const focusRoot = toastRoot('focus test');
    const actionable = focusRoot.querySelector('[data-close-button], button') as HTMLElement;
    fireEvent.focus(actionable);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    // The verified gap: keyboard focus alone did not keep it up.
    expect(screen.queryByText('focus test')).not.toBeInTheDocument();
  });
});
