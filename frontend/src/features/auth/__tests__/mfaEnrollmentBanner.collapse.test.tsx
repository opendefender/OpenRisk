// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #751 phase 4 — dismissing the soft MFA recommendation used to unmount the
// banner outright. It now collapses first (.or-collapse, index.css) and only
// leaves the DOM once useExitTimer's timer fires — immediately under
// prefers-reduced-motion, after --dur-fast otherwise. This file owns that
// timing; mfaEnrollmentBanner.test.tsx owns the "who may dismiss what".

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { MFAEnrollmentBanner } from '../MFAEnrollmentBanner';
import type { MFAStatus } from '../mfaPolicyService';

const fetchMFAStatus = vi.fn();
vi.mock('../mfaPolicyService', async () => {
  const actual = await vi.importActual<typeof import('../mfaPolicyService')>('../mfaPolicyService');
  return { ...actual, fetchMFAStatus: (...a: unknown[]) => fetchMFAStatus(...a) };
});

function status(over: Partial<MFAStatus> = {}): MFAStatus {
  return {
    state: 'recommended',
    configured: false,
    required: false,
    privileged: false,
    grace_period_active: false,
    grace_days: 7,
    ...over,
  } as MFAStatus;
}

function renderBanner() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MFAEnrollmentBanner />
    </QueryClientProvider>,
  );
}

const originalMatchMedia = window.matchMedia;

describe('MFAEnrollmentBanner — dismiss collapse (#751 phase 4)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
  });

  afterEach(() => {
    window.matchMedia = originalMatchMedia;
  });

  it('collapses first, then is removed on a timer (normal motion)', async () => {
    fetchMFAStatus.mockResolvedValue(status());
    renderBanner();
    const banner = await screen.findByTestId('mfa-enrollment-banner');
    const shell = banner.closest('.or-collapse') as HTMLElement;
    expect(shell).toHaveAttribute('data-open', 'true');

    await userEvent.click(screen.getByRole('button', { name: /dismiss|masquer/i }));

    // The testid/role vanish this instant (existing contract), but the shell
    // is still mounted, now collapsing.
    expect(screen.queryByTestId('mfa-enrollment-banner')).not.toBeInTheDocument();
    expect(shell).toHaveAttribute('data-open', 'false');
    expect(shell).toBeInTheDocument();

    // Then, after its exit timer, it is gone for real.
    await waitFor(() => expect(shell).not.toBeInTheDocument());
  });

  it('is removed immediately under prefers-reduced-motion', async () => {
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('prefers-reduced-motion'),
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
      onchange: null,
    }));

    fetchMFAStatus.mockResolvedValue(status());
    renderBanner();
    const banner = await screen.findByTestId('mfa-enrollment-banner');
    const shell = banner.closest('.or-collapse') as HTMLElement;

    await userEvent.click(screen.getByRole('button', { name: /dismiss|masquer/i }));

    // No visible "still collapsing" window to observe under reduced motion —
    // the exit timer resolves on the next macrotask regardless.
    await waitFor(() => expect(shell).not.toBeInTheDocument());
  });
});
