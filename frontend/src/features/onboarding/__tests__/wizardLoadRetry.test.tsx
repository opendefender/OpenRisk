// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The tunnel's load-failure screen.
//
// The defect: when GET /onboarding/state failed (a 500 while the database was
// unavailable), the wizard and its redirect guard fell into a remount loop —
// TanStack Query v5 resets a never-succeeded query to `pending` on refetch, so
// the guard swapped the wizard for its spinner on every refetch-on-mount — and
// the "Try again" button, a full reload, could never stay on screen long
// enough to be pressed. It must hold still and refetch in place.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider, focusManager } from '@tanstack/react-query';

import type { OnboardingState } from '../../../services/activationService';

const getOnboardingState = vi.fn();

vi.mock('../../../services/activationService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/activationService')>(
    '../../../services/activationService',
  );
  return {
    ...actual,
    activationService: {
      getState: vi.fn(),
      markCelebrated: vi.fn(),
      getOnboardingState: (...a: unknown[]) => getOnboardingState(...a),
      saveStep: vi.fn(),
      complete: vi.fn(),
      getSuggestions: vi.fn(),
      getPosture: vi.fn(),
    },
  };
});

import { OnboardingWizard } from '../wizard/OnboardingWizard';
import { OnboardingCompletedRedirect } from '../OnboardingGuard';

const okState = {
  current_step: 'organization',
  steps: ['organization', 'goal', 'framework', 'score', 'cover'],
  skipped_steps: [],
  step_index: 0,
  completed: false,
  percent: 0,
  answers: {},
  landing: '/',
} as OnboardingState;

function renderWizard() {
  // `retry: 1` lives on the hook itself, so the client default cannot switch it
  // off; the mock below fails twice to exhaust it.
  const client = new QueryClient();
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/onboarding/organization']}>
        <Routes>
          <Route
            path="/onboarding"
            element={
              <OnboardingCompletedRedirect>
                <OnboardingWizard />
              </OnboardingCompletedRedirect>
            }
          >
            <Route path="organization" element={<div data-testid="org-step">org</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('OnboardingWizard load failure', () => {
  beforeEach(() => {
    getOnboardingState.mockReset();
    // TanStack pauses retries while the document is unfocused, and jsdom never
    // reports focus — without this the hook's `retry: 1` waits forever.
    focusManager.setFocused(true);
  });

  it('retries in place and renders the step once the server recovers', async () => {
    const boom = Object.assign(new Error('Request failed with status code 500'), { status: 500 });
    // Every call fails until the test says otherwise: the server stays down.
    getOnboardingState.mockRejectedValue(boom);

    renderWizard();

    const retry = await screen.findByTestId('wizard-load-retry', {}, { timeout: 8000 });
    expect(screen.queryByTestId('org-step')).toBeNull();

    // No remount loop: with the server still down the screen stays put and the
    // query is not re-fired behind it. (The loop was the guard swapping the
    // wizard for its placeholder on every refetch-on-mount.)
    // The wizard's own refetch-on-mount (and its one retry) is expected; wait
    // for it to finish, then nothing further may fire.
    await waitFor(() => expect(screen.getByTestId('wizard-load-retry')).toBeEnabled(), {
      timeout: 5000,
    });
    const settled = getOnboardingState.mock.calls.length;
    await new Promise((r) => setTimeout(r, 2500));
    expect(getOnboardingState.mock.calls.length).toBe(settled);
    expect(screen.getByTestId('wizard-load-retry')).toBeEnabled();

    let resolve: (s: OnboardingState) => void = () => {};
    getOnboardingState.mockReturnValueOnce(
      new Promise<OnboardingState>((r) => {
        resolve = r;
      }),
    );
    fireEvent.click(retry);

    // The button stays mounted and says it is working — the guard must not
    // swap the whole wizard for its placeholder mid-retry.
    await waitFor(() => expect(screen.getByTestId('wizard-load-retry')).toBeDisabled());

    const callsBeforeRecovery = getOnboardingState.mock.calls.length;
    resolve(okState);
    await waitFor(() => expect(screen.getByTestId('org-step')).toBeInTheDocument());
    // Recovered by the one refetch, not by a reload or a refetch storm.
    expect(getOnboardingState.mock.calls.length).toBe(callsBeforeRecovery);
  }, 15_000);
});
