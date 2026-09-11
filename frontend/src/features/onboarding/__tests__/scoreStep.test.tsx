// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Step 4 of the tunnel (#643).
//
// The defect: the step rendered "Évaluez ce risque" over two sliders and no
// risk, its sliders opened on a hard-coded 0.5/6 regardless of the risk, and
// the score was stored as an opaque wizard answer and applied to nothing. A
// user who scored 6.30 found 2 in their register.
//
// These own the two claims that do not need a server: the step names its
// subject, and it opens on that subject's real values.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import type { OnboardingState } from '../../../services/activationService';

const getOnboardingState = vi.fn();
const saveStep = vi.fn();

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
      complete: vi.fn(),
      getSuggestions: vi.fn(),
      getPosture: vi.fn(),
    },
  };
});

import { ScoreStep } from '../wizard/valueSteps';

function baseState(overrides: Partial<OnboardingState> = {}): OnboardingState {
  return {
    current_step: 'score',
    steps: ['organization', 'goal', 'framework', 'score', 'cover'],
    skipped_steps: [],
    step_index: 3,
    completed: false,
    percent: 60,
    answers: {},
    landing: '/dashboard',
    ...overrides,
  } as OnboardingState;
}

function renderStep() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/onboarding/score']}>
        <ScoreStep />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ScoreStep', () => {
  beforeEach(() => {
    getOnboardingState.mockReset();
    saveStep.mockReset();
  });

  it('names the risk it is scoring', async () => {
    getOnboardingState.mockResolvedValue(
      baseState({
        score_target: {
          id: 'r-1',
          title: 'Indisponibilité de la plateforme de sinistres',
          probability: 0.2,
          impact: 10,
        },
      }),
    );

    renderStep();

    await waitFor(() =>
      expect(screen.getByTestId('score-target')).toHaveTextContent(
        'Indisponibilité de la plateforme de sinistres',
      ),
    );
  });

  it('opens the sliders on the risk’s own values, not on a hard-coded default', async () => {
    getOnboardingState.mockResolvedValue(
      baseState({
        score_target: { id: 'r-1', title: 'Un risque', probability: 0.9, impact: 8 },
      }),
    );

    renderStep();

    // 0.9 × 8 = 7.20. The old defaults (0.5 × 6) would read 3.00.
    await waitFor(() => expect(screen.getByTestId('score-readout')).toHaveTextContent('7.20'));
  });

  it('prefers a stored answer over the risk’s values, so a resume shows what was set', async () => {
    getOnboardingState.mockResolvedValue(
      baseState({
        answers: { score: { probability: 0.7, impact: 9 } },
        score_target: { id: 'r-1', title: 'Un risque', probability: 0.2, impact: 10 },
      }),
    );

    renderStep();

    // The number the bug report quotes: scored 6.30, register showed 2.
    await waitFor(() => expect(screen.getByTestId('score-readout')).toHaveTextContent('6.30'));
  });

  it('stays passable when the tenant adopted no risk, and says why', async () => {
    getOnboardingState.mockResolvedValue(baseState());

    renderStep();

    await waitFor(() => expect(screen.getByTestId('score-no-target')).toBeInTheDocument());
    expect(screen.queryByTestId('score-target')).not.toBeInTheDocument();
    // Criterion 5: nobody is trapped on a step they cannot complete.
    expect(screen.getByRole('button', { name: /Continuer|Continue/i })).toBeEnabled();
  });
});
