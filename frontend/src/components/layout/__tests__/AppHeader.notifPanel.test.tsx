// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — the notification panel used to have no Escape handling, no
// focus trap and no focus return (AppHeader.tsx:218-402, pre-#751): it closed
// only on an invisible backdrop click. These tests are new — every one of
// them fails against the pre-fix panel.

import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';

import type { Notification } from '../../../features/notifications/notificationService';

const unreadCountMock = vi.fn();
const notificationsMock = vi.fn();
const markAllReadMutate = vi.fn();

vi.mock('../../../features/notifications/useNotifications', () => ({
  useUnreadCount: () => unreadCountMock(),
  useNotifications: () => notificationsMock(),
  useNotificationActions: () => ({
    markRead: { mutate: vi.fn() },
    markAllRead: { mutate: markAllReadMutate },
  }),
}));

import { AppHeader } from '../AppHeader';
import { useUIStore } from '../../../store/uiStore';

const notifications: Notification[] = [
  {
    id: 'n1',
    user_id: 'u',
    tenant_id: 't',
    type: 'critical_risk',
    channel: 'in_app',
    status: 'sent',
    subject: 'Critical risk detected',
    message: 'A critical risk was raised on srv-01.',
    read_at: null,
    created_at: new Date().toISOString(),
  },
  {
    id: 'n2',
    user_id: 'u',
    tenant_id: 't',
    type: 'action_assigned',
    channel: 'in_app',
    status: 'sent',
    subject: 'Task assigned',
    message: 'You were assigned a mitigation task.',
    read_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
  },
];

function renderHeader() {
  return render(
    <MemoryRouter>
      <AppHeader onOpenMobileNav={() => {}} />
    </MemoryRouter>,
  );
}

const bell = () => screen.getByRole('button', { name: /notifications/i });

describe('AppHeader — notification panel (#751 phase 4)', () => {
  beforeEach(() => {
    unreadCountMock.mockReset().mockReturnValue({ count: 1, isFetched: true });
    notificationsMock.mockReset().mockReturnValue({
      notifications,
      isLoading: false,
      isError: false,
    });
    markAllReadMutate.mockReset();
    useUIStore.setState({ lang: 'en' });
  });

  it('Escape closes the panel and returns focus to the bell trigger', async () => {
    renderHeader();
    await userEvent.click(bell());
    expect(screen.getByRole('dialog')).toBeInTheDocument();

    await userEvent.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(bell()).toHaveFocus();
  });

  it('opens with aria-expanded true and closes it back to false', async () => {
    renderHeader();
    expect(bell()).toHaveAttribute('aria-expanded', 'false');
    await userEvent.click(bell());
    expect(bell()).toHaveAttribute('aria-expanded', 'true');
    await userEvent.keyboard('{Escape}');
    await waitFor(() => expect(bell()).toHaveAttribute('aria-expanded', 'false'));
  });

  it('keeps every panel control in the tab order for the trap to wrap around', async () => {
    // NOT a tab traversal. useDismissableLayer's trap filters candidates on
    // `offsetParent`, which jsdom always reports as null because it does no
    // layout, so simulating Tab here would prove a jsdom quirk rather than
    // the keyboard contract (see governedBulk.test.tsx for the same call).
    // Real wrap-around traversal is a Playwright concern. What IS provable
    // here: the mark-all-read button, both filter chips and both rows stay
    // reachable — none pulled from the tab order or hidden from the a11y
    // tree, which is the precondition the trap depends on.
    renderHeader();
    await userEvent.click(bell());
    const dialog = screen.getByRole('dialog');

    const focusable = dialog.querySelectorAll<HTMLElement>(
      'button, [tabindex]:not([tabindex="-1"])',
    );
    expect(focusable.length).toBeGreaterThanOrEqual(5); // mark-all-read + 2 chips + 2 rows
    focusable.forEach((el) => {
      expect(el).not.toHaveAttribute('tabindex', '-1');
      expect(el).toBeVisible();
    });
  });

  it('stays mounted through its close animation instead of vanishing instantly', async () => {
    renderHeader();
    await userEvent.click(bell());
    const dialog = screen.getByRole('dialog');

    await userEvent.keyboard('{Escape}');
    // Still in the document immediately after the close request — the
    // exit-timer keeps it mounted for the CSS animation, only data-open flips
    // right away.
    expect(dialog).toHaveAttribute('data-open', 'false');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('reopening mid-exit keeps the same panel node mounted and ends open (review fix)', () => {
    // Regression: .notif-panel used to be a @keyframes animation, which
    // restarts from 0% every time `data-open` flips — so reopening during
    // the 120ms exit snapped the panel to fully closed before replaying the
    // enter, a visible flash. A `transition` (this fix) has no "restart":
    // it just continues interpolating from wherever the value currently is.
    vi.useFakeTimers();
    try {
      renderHeader();

      fireEvent.click(bell());
      // Flush useEnterGate's one-macrotask delay so the panel is genuinely
      // open (data-open="true"), not just mounted-but-still-gated.
      act(() => {
        vi.advanceTimersByTime(0);
      });
      const dialog = screen.getByRole('dialog');
      expect(dialog).toHaveAttribute('data-open', 'true');

      fireEvent.keyDown(document, { key: 'Escape' });
      expect(dialog).toHaveAttribute('data-open', 'false');

      // Reopen well before NOTIF_EXIT_MS (120ms) elapses.
      act(() => {
        vi.advanceTimersByTime(50);
      });
      fireEvent.click(bell());

      // Same node — no remount — and it ends up open, not stuck mid-close.
      expect(screen.getByRole('dialog')).toBe(dialog);
      expect(dialog).toHaveAttribute('data-open', 'true');
    } finally {
      vi.useRealTimers();
    }
  });

  it('.notif-panel animates via transition, never a restarting keyframe (review fix)', () => {
    // jsdom cannot play back a real CSS animation/transition timeline, so
    // the DOM-level reopen test above cannot observe the actual visual bug
    // (a @keyframes animation restarts from 0% every time `data-open` flips,
    // so reopening mid-exit snapped the panel to fully closed before
    // replaying the enter). This reads the authored rule directly instead:
    // it must be a `transition` (which continues interpolating from
    // wherever the value currently sits), never driven by `animation:`
    // (which always restarts).
    const css = readFileSync(path.resolve(__dirname, '../../../index.css'), 'utf8');
    const base = css.match(/\.notif-panel\s*\{[^}]*\}/)?.[0] ?? '';
    const openState = css.match(/\.notif-panel\[data-open='true'\]\s*\{[^}]*\}/)?.[0] ?? '';
    expect(base).toMatch(/transition:/);
    expect(base).not.toMatch(/animation:/);
    expect(openState).toMatch(/transition:/);
    expect(openState).not.toMatch(/animation:/);
  });
});
