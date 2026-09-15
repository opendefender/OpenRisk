// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The tunnel's exit (#685).
//
// The defect: on the last step, « Voir ma posture » saved the step and asked the
// server to advance. The server clamps the cursor to the last step, so the page
// reloaded itself, POST /onboarding/complete was never sent, and the route guard
// kept every screen of the app pointing back at that step. Every new sign-up was
// locked out.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactElement } from 'react';

import type { OnboardingState } from '../../../services/activationService';
import { useUIStore } from '../../../store/uiStore';

const getOnboardingState = vi.fn();
const saveStep = vi.fn();
const complete = vi.fn();
const getPosture = vi.fn();

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
      saveStep: (...a: unknown[]) => saveStep(...a),
      complete: (...a: unknown[]) => complete(...a),
      getSuggestions: vi.fn(),
      getPosture: (...a: unknown[]) => getPosture(...a),
    },
  };
});

import { CoverStep } from '../wizard/valueSteps';
import { useStepNav } from '../wizard/stepNav';

function state(overrides: Partial<OnboardingState> = {}): OnboardingState {
  return {
    current_step: 'cover',
    steps: ['organization', 'goal', 'framework', 'score', 'cover'],
    skipped_steps: [],
    step_index: 4,
    completed: false,
    percent: 80,
    answers: {},
    landing: '/dashboard',
    ...overrides,
  } as OnboardingState;
}

/** A step that is not `cover`, to prove the exit follows the server's step list. */
function ScoreHarness() {
  const { go, busy, error } = useStepNav('score');
  return (
    <button type="button" onClick={() => go({ probability: 0.4 }, 1)} disabled={busy}>
      {error ? 'score failed' : 'score next'}
    </button>
  );
}

function renderRoutes(path: string, routes: Record<string, ReactElement>) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          {Object.entries(routes).map(([p, el]) => (
            <Route key={p} path={p} element={el} />
          ))}
          <Route path="/posture" element={<p>posture reveal</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

// Not "#685" in the title: the raw-colour lint reads a hash and three hex digits as a colour.
describe('the tunnel exit (issue 685)', () => {
  beforeEach(() => {
    useUIStore.getState().setLang('fr');
    for (const fn of [getOnboardingState, saveStep, complete, getPosture]) fn.mockReset();
    getPosture.mockResolvedValue({ top_risks: [] });
  });

  it('saves the last step, then completes onboarding, then lands on the posture reveal', async () => {
    getOnboardingState.mockResolvedValue(state());
    saveStep.mockResolvedValue(state());
    complete.mockResolvedValue(state({ completed: true }));

    renderRoutes('/onboarding/cover', { '/onboarding/cover': <CoverStep /> });
    fireEvent.click(await screen.findByTestId('wizard-next'));

    expect(await screen.findByText('posture reveal')).toBeInTheDocument();
    expect(saveStep).toHaveBeenCalledWith('cover', expect.any(Object), 'cover');
    expect(complete).toHaveBeenCalledTimes(1);
    expect(saveStep.mock.invocationCallOrder[0]).toBeLessThan(complete.mock.invocationCallOrder[0]);
  });

  it('keeps the user on the step with a retry when completing fails, and the retry exits', async () => {
    getOnboardingState.mockResolvedValue(state());
    saveStep.mockResolvedValue(state());
    complete
      .mockRejectedValueOnce(new Error('503'))
      .mockResolvedValueOnce(state({ completed: true }));

    renderRoutes('/onboarding/cover', { '/onboarding/cover': <CoverStep /> });
    fireEvent.click(await screen.findByTestId('wizard-next'));

    expect(await screen.findByTestId('wizard-save-error')).toBeInTheDocument();
    expect(screen.queryByText('posture reveal')).toBeNull();

    fireEvent.click(screen.getByTestId('wizard-retry'));
    expect(await screen.findByText('posture reveal')).toBeInTheDocument();
    expect(saveStep).toHaveBeenCalledTimes(2);
    expect(complete).toHaveBeenCalledTimes(2);
  });

  it('never completes when saving the last step fails', async () => {
    getOnboardingState.mockResolvedValue(state());
    saveStep.mockRejectedValue(new Error('500'));

    renderRoutes('/onboarding/cover', { '/onboarding/cover': <CoverStep /> });
    fireEvent.click(await screen.findByTestId('wizard-next'));

    expect(await screen.findByTestId('wizard-save-error')).toBeInTheDocument();
    expect(complete).not.toHaveBeenCalled();
  });

  it('exits from whichever step is last when cover is auto-skipped', async () => {
    const skipped = state({
      current_step: 'score',
      steps: ['organization', 'goal', 'framework', 'score'],
      skipped_steps: ['cover'],
      step_index: 3,
    });
    getOnboardingState.mockResolvedValue(skipped);
    saveStep.mockResolvedValue(skipped);
    complete.mockResolvedValue({ ...skipped, completed: true });

    renderRoutes('/onboarding/score', { '/onboarding/score': <ScoreHarness /> });
    await waitFor(() => expect(getOnboardingState).toHaveBeenCalled());
    fireEvent.click(await screen.findByRole('button', { name: 'score next' }));

    expect(await screen.findByText('posture reveal')).toBeInTheDocument();
    expect(complete).toHaveBeenCalledTimes(1);
  });

  it('only advances the cursor from a step that is not the last', async () => {
    getOnboardingState.mockResolvedValue(state({ current_step: 'score', step_index: 3 }));
    saveStep.mockResolvedValue(state({ current_step: 'cover' }));

    renderRoutes('/onboarding/score', {
      '/onboarding/score': <ScoreHarness />,
      '/onboarding/cover': <p>cover step</p>,
    });
    await waitFor(() => expect(getOnboardingState).toHaveBeenCalled());
    fireEvent.click(await screen.findByRole('button', { name: 'score next' }));

    expect(await screen.findByText('cover step')).toBeInTheDocument();
    expect(saveStep).toHaveBeenCalledWith('score', { probability: 0.4 }, 'cover');
    expect(complete).not.toHaveBeenCalled();
  });
});
