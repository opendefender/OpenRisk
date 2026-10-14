// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #966 — an optional shell piece whose chunk fails to load must vanish quietly
// and leave the rest of the app on screen, not trip the root error page.

import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { lazy, Suspense } from 'react';

vi.mock('../../../lib/observability', () => ({ reportError: vi.fn(() => 'err-1') }));

import { reportError } from '../../../lib/observability';
import { QuietBoundary } from '../QuietBoundary';

describe('QuietBoundary', () => {
  it('renders its children when nothing fails', () => {
    render(
      <QuietBoundary>
        <p>tour</p>
      </QuietBoundary>,
    );
    expect(screen.getByText('tour')).toBeInTheDocument();
  });

  it('renders nothing, keeps its siblings and reports, when a lazy chunk fails to load', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    const Broken = lazy(() =>
      Promise.reject(new Error('Failed to fetch dynamically imported module')),
    );
    render(
      <>
        <main>the app</main>
        <QuietBoundary>
          <Suspense fallback={null}>
            <Broken />
          </Suspense>
        </QuietBoundary>
      </>,
    );
    await vi.waitFor(() => expect(reportError).toHaveBeenCalled());
    expect(screen.getByText('the app')).toBeInTheDocument();
    spy.mockRestore();
  });
});
