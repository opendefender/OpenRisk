// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — before BannerStack, App.tsx rendered <DemoBanner /> then
// <OfflineBanner /> as two independent divs; "stacking" was only ever mount
// order. This proves the governed container renders both, in the fixed
// order (Demo permanent and on top, Offline closest to the header), without
// one clobbering the other's data-testid.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

vi.mock('../../lib/api', () => ({
  api: { get: vi.fn().mockResolvedValue({ data: { status: 'ok', demo_mode: true } }) },
}));

import { BannerStack } from '../BannerStack';
import { DemoBanner } from '../DemoBanner';
import { OfflineBanner } from '../OfflineBanner';
import { useUIStore } from '../../store/uiStore';

describe('BannerStack (#751 phase 4)', () => {
  beforeEach(() => {
    useUIStore.setState({ lang: 'en' });
  });

  afterEach(() => {
    act(() => {
      window.dispatchEvent(new Event('online'));
    });
  });

  it('renders Demo and Offline together, Demo first', async () => {
    const client = new QueryClient();
    render(
      <QueryClientProvider client={client}>
        <BannerStack>
          <DemoBanner />
          <OfflineBanner />
        </BannerStack>
      </QueryClientProvider>,
    );

    act(() => {
      window.dispatchEvent(new Event('offline'));
    });

    const demo = await waitFor(() => screen.getByTestId('demo-banner'));
    const offline = screen.getByTestId('offline-banner');
    expect(demo).toBeInTheDocument();
    expect(offline).toBeInTheDocument();
    // Demo is permanent and sits on top; Offline is closest to the header.
    expect(demo.compareDocumentPosition(offline) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
});
