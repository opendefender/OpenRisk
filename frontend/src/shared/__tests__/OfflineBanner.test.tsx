// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — OfflineBanner used to return null outright while healthy,
// which meant the header jumped with no transition the instant a tenant went
// offline or came back, and a flapping connection could flicker the banner
// open/closed on every retry. Neither is true anymore: the banner is always
// mounted (collapsed via .or-collapse) and, once shown, is held for at least
// 2000ms regardless of how quickly the connection recovers.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { OfflineBanner } from '../OfflineBanner';
import { useUIStore } from '../../store/uiStore';

function renderBanner() {
  const client = new QueryClient();
  return render(
    <QueryClientProvider client={client}>
      <OfflineBanner />
    </QueryClientProvider>,
  );
}

const wrapper = () => screen.getByTestId('offline-banner');

describe('OfflineBanner (#751 phase 4)', () => {
  beforeEach(() => {
    useUIStore.setState({ lang: 'en' });
    vi.useFakeTimers();
    act(() => {
      window.dispatchEvent(new Event('online'));
    });
  });

  afterEach(() => {
    act(() => {
      window.dispatchEvent(new Event('online'));
    });
    vi.useRealTimers();
  });

  it('is always mounted, collapsed (data-open=false) while the connection is healthy', () => {
    renderBanner();
    expect(wrapper()).toHaveAttribute('data-open', 'false');
    expect(wrapper()).toHaveAttribute('aria-hidden', 'true');
  });

  it('opens on going offline', () => {
    renderBanner();
    act(() => {
      window.dispatchEvent(new Event('offline'));
    });
    expect(wrapper()).toHaveAttribute('data-open', 'true');
    expect(wrapper()).toHaveTextContent('Offline');
  });

  it('stays open at least 2000ms even once the connection recovers immediately', () => {
    renderBanner();
    act(() => {
      window.dispatchEvent(new Event('offline'));
    });
    expect(wrapper()).toHaveAttribute('data-open', 'true');

    // Reconnects almost immediately — a flapping link, not a real outage.
    act(() => {
      window.dispatchEvent(new Event('online'));
      vi.advanceTimersByTime(500);
    });
    expect(wrapper()).toHaveAttribute('data-open', 'true');

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(wrapper()).toHaveAttribute('data-open', 'true');

    act(() => {
      vi.advanceTimersByTime(600); // total 2100ms since it opened
    });
    expect(wrapper()).toHaveAttribute('data-open', 'false');
  });
});
