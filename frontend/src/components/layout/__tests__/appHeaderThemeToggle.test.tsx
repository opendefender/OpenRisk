// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 5 — required test 7: the theme toggle's accessible name follows
// the theme, naming the result of pressing it rather than a static "toggle
// theme" that never changes.

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

vi.mock('../../../features/notifications/notificationService', () => ({
  notificationService: {
    list: vi.fn().mockResolvedValue([]),
    unreadCount: vi.fn().mockResolvedValue(0),
    markRead: vi.fn(),
    markAllRead: vi.fn(),
  },
}));

import { AppHeader } from '../AppHeader';
import { useUIStore } from '../../../store/uiStore';

function renderHeader() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AppHeader onOpenMobileNav={() => {}} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ lang: 'en', theme: 'dark' });
});

describe('AppHeader theme toggle', () => {
  it('names the theme it will switch TO, and that name flips with the theme', async () => {
    const user = userEvent.setup();
    renderHeader();

    // Starts dark: pressing it goes to light.
    const button = screen.getByRole('button', { name: 'Switch to light theme' });

    await user.click(button);

    // Same button, new name — the real store toggled, not a stand-in.
    expect(screen.getByRole('button', { name: 'Switch to dark theme' })).toBe(button);
  });
});
