// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — the bell badge shows the real unread count instead of a
// dot, and its entrance is gated by "armed" (the query's first resolved
// fetch), never by this component's own mount. These three behaviours were
// all absent before: the bell threw the number away and rendered a static
// dot (AppHeader.tsx:192, pre-#751).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';

const unreadCountMock = vi.fn();

vi.mock('../../../features/notifications/useNotifications', () => ({
  useUnreadCount: () => unreadCountMock(),
  useNotifications: () => ({ notifications: [], isLoading: false, isError: false }),
  useNotificationActions: () => ({
    markRead: { mutate: vi.fn() },
    markAllRead: { mutate: vi.fn() },
  }),
}));

import { AppHeader } from '../AppHeader';
import { useUIStore } from '../../../store/uiStore';

function renderHeader() {
  return render(
    <MemoryRouter>
      <AppHeader onOpenMobileNav={() => {}} />
    </MemoryRouter>,
  );
}

const bell = () => screen.getByRole('button', { name: /notifications/i });
const badge = () => document.querySelector('.notif-badge') as HTMLElement;

describe('AppHeader — notification badge (#751 phase 4)', () => {
  beforeEach(() => {
    unreadCountMock.mockReset();
    useUIStore.setState({ lang: 'en' });
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('carries the real unread count in the bell aria-label, not the 9+ cap', () => {
    unreadCountMock.mockReturnValue({ count: 142, isFetched: true });
    renderHeader();
    expect(bell()).toHaveAccessibleName('Notifications, 142 unread');
    // The glyph itself still caps — 142 characters would break the geometry.
    expect(badge()).toHaveTextContent('9+');
  });

  it('reads the plain title, with no count, when there is nothing unread', () => {
    unreadCountMock.mockReturnValue({ count: 0, isFetched: true });
    renderHeader();
    expect(bell()).toHaveAccessibleName('Notifications');
  });

  it('never animates on mount: the badge is unarmed on the very first paint', () => {
    // Even a cache hit (isFetched already true on render 1) must not arm
    // instantly — arming is deferred a macrotask so mount and "armed" never
    // land in the same paint.
    unreadCountMock.mockReturnValue({ count: 3, isFetched: true });
    renderHeader();
    expect(badge()).toHaveAttribute('data-armed', 'false');
  });

  it('arms one macrotask after the first fetch resolves, then sets data-visible on 0→n', () => {
    unreadCountMock.mockReturnValue({ count: 0, isFetched: false });
    const { rerender } = renderHeader();
    expect(badge()).toHaveAttribute('data-armed', 'false');
    expect(badge()).toHaveAttribute('data-visible', 'false');

    const rerenderHeader = () =>
      rerender(
        <MemoryRouter>
          <AppHeader onOpenMobileNav={() => {}} />
        </MemoryRouter>,
      );

    // The query resolves with nothing unread — arms, but stays invisible.
    unreadCountMock.mockReturnValue({ count: 0, isFetched: true });
    rerenderHeader();
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(badge()).toHaveAttribute('data-armed', 'true');
    expect(badge()).toHaveAttribute('data-visible', 'false');

    // A later poll reports unread items: 0→n, now that arming has already
    // happened — this is the only transition allowed to animate.
    unreadCountMock.mockReturnValue({ count: 3, isFetched: true });
    rerenderHeader();
    expect(badge()).toHaveAttribute('data-armed', 'true');
    expect(badge()).toHaveAttribute('data-visible', 'true');
  });

  it('is structurally silent on a same-count poll: nothing about the badge changes', () => {
    unreadCountMock.mockReturnValue({ count: 5, isFetched: true });
    const { rerender } = renderHeader();
    act(() => {
      vi.advanceTimersByTime(0);
    });
    const before = badge();
    expect(before).toHaveAttribute('data-visible', 'true');

    // A poll that repeats the same count — the common case every 60s.
    unreadCountMock.mockReturnValue({ count: 5, isFetched: true });
    rerender(
      <MemoryRouter>
        <AppHeader onOpenMobileNav={() => {}} />
      </MemoryRouter>,
    );

    const after = badge();
    expect(after).toBe(before); // same node: no remount, nothing to retrigger
    expect(after).toHaveAttribute('data-armed', 'true');
    expect(after).toHaveAttribute('data-visible', 'true');
    expect(after).toHaveTextContent('5');
  });
});
