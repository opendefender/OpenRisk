// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The theme toggle in the tunnel header (#666).
//
// The tunnel cannot be left until it is finished, and it was the one screen
// between sign-in and the app with no way to change the theme. These own the
// three claims that need no server: the toggle is there before the state has
// loaded, it flips the theme on <html>, and its name says what it will do.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

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
import { useUIStore } from '../../../store/uiStore';

function state(): OnboardingState {
  return {
    current_step: 'organization',
    steps: ['organization', 'goal', 'framework', 'score', 'cover'],
    skipped_steps: [],
    step_index: 0,
    completed: false,
    percent: 0,
    answers: {},
    landing: '/dashboard',
  } as OnboardingState;
}

function renderWizard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/onboarding/organization']}>
        <OnboardingWizard />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('OnboardingWizard theme toggle', () => {
  beforeEach(() => {
    getOnboardingState.mockReset();
    useUIStore.getState().setTheme('dark');
  });

  it('is available while the tunnel state is still loading', () => {
    getOnboardingState.mockReturnValue(new Promise(() => {}));

    renderWizard();

    expect(screen.getByTestId('wizard-theme-toggle')).toBeInTheDocument();
  });

  it('flips the theme on <html> and names the theme it switches to', async () => {
    getOnboardingState.mockResolvedValue(state());

    renderWizard();

    const toggle = screen.getByTestId('wizard-theme-toggle');
    expect(document.documentElement).toHaveAttribute('data-theme', 'dark');
    expect(toggle).toHaveAccessibleName(/clair|light/i);

    fireEvent.click(toggle);

    await waitFor(() => expect(document.documentElement).toHaveAttribute('data-theme', 'light'));
    expect(useUIStore.getState().themeMode).toBe('light');
    expect(toggle).toHaveAccessibleName(/sombre|dark/i);
  });
});
