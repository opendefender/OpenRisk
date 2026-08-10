// Unit coverage for the activation panel's contract.
//
// The e2e suite (tests/e2e/activation.spec.ts) owns the claims that need a real
// server: the journey, its duration, and the state surviving a reload. These
// tests own the one thing the component is still allowed to do — render the
// server's answer faithfully — and prove the properties behind the reported
// bugs:
//
//   • the checklist ticks exactly what the server says is complete;
//   • one completed step strikes through one row (not two);
//   • the celebration fires once per step, driven by the server's flag, and is
//     acknowledged so it cannot fire again;
//   • prefers-reduced-motion gets a toast instead of confetti, never nothing.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { OnboardingChecklist } from '../OnboardingChecklist';
import { nextActionableStep } from '../useActivation';
import { i18n, type ActivationState, type ActivationStep } from '../../../services/activationService';

// --- mocks -----------------------------------------------------------------

const getState = vi.fn();
const markCelebrated = vi.fn();
vi.mock('../../../services/activationService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/activationService')>(
    '../../../services/activationService',
  );
  return {
    ...actual,
    activationService: {
      getState: (...args: unknown[]) => getState(...args),
      markCelebrated: (...args: unknown[]) => markCelebrated(...args),
      getOnboardingState: vi.fn(),
      saveStep: vi.fn(),
      complete: vi.fn(),
      getSuggestions: vi.fn(),
    },
  };
});

const confetti = vi.fn();
vi.mock('../../../shared/celebrate', () => ({ confetti: (...a: unknown[]) => confetti(...a) }));

const toastSuccess = vi.fn();
vi.mock('sonner', () => ({ toast: { success: (...a: unknown[]) => toastSuccess(...a) } }));

// --- fixtures --------------------------------------------------------------

function step(over: Partial<ActivationStep> = {}): ActivationStep {
  return {
    key: 'first_risk',
    event_key: 'risk.created',
    label_i18n: { fr: 'Créez votre premier risque', en: 'Create your first risk' },
    hint_i18n: { fr: 'Le cœur du produit.', en: 'The heart of the product.' },
    completed: false,
    completed_at: null,
    deep_link: '/risks',
    order: 1,
    primary: true,
    celebrate: false,
    ...over,
  };
}

function state(steps: ActivationStep[]): ActivationState {
  const done = steps.filter((s) => s.completed).length;
  return {
    steps,
    percent: steps.length ? Math.round((done / steps.length) * 100) : 0,
    aha_reached_at: null,
  };
}

function renderChecklist() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <OnboardingChecklist />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  getState.mockReset();
  markCelebrated.mockReset().mockResolvedValue(undefined);
  confetti.mockReset();
  toastSuccess.mockReset();
  // Default: motion allowed.
  window.matchMedia = vi.fn().mockImplementation((q: string) => ({
    matches: false,
    media: q,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })) as unknown as typeof window.matchMedia;
});

afterEach(() => vi.clearAllMocks());

// --- tests -----------------------------------------------------------------

describe('OnboardingChecklist — renders server state only', () => {
  it('strikes through exactly the steps the server reports complete', async () => {
    getState.mockResolvedValue(
      state([
        step({ key: 'first_risk', completed: true, completed_at: '2026-08-10T09:00:00Z' }),
        step({ key: 'framework', primary: false, order: 2, label_i18n: { fr: 'Importez', en: 'Import' }, hint_i18n: {} }),
        step({ key: 'team', primary: false, order: 3, label_i18n: { fr: 'Invitez', en: 'Invite' }, hint_i18n: {} }),
      ]),
    );

    renderChecklist();

    const done = await screen.findByTestId('activation-step-first_risk');
    expect(done).toHaveAttribute('data-completed', 'true');

    // THE regression: completing one step must not tick a second row.
    expect(screen.getByTestId('activation-step-framework')).toHaveAttribute(
      'data-completed',
      'false',
    );
    expect(screen.getByTestId('activation-step-team')).toHaveAttribute('data-completed', 'false');
  });

  it('hides itself when everything is done, without storing a dismissal', async () => {
    getState.mockResolvedValue(state([step({ completed: true, completed_at: 'x' })]));
    const { container } = renderChecklist();
    await waitFor(() => expect(container.querySelector('section')).toBeNull());
    // Nothing client-side decides visibility — completion is a server fact.
    expect(localStorage.getItem('openrisk-onboarding')).toBeNull();
  });

  it('stays silent when the endpoint fails rather than showing an error banner', async () => {
    getState.mockRejectedValue(new Error('boom'));
    const { container } = renderChecklist();
    await waitFor(() => expect(container.querySelector('section')).toBeNull());
    expect(screen.queryByRole('alert')).toBeNull();
  });
});

describe('celebration — server-driven and idempotent', () => {
  it('fires once for a step the server flags, then acknowledges it', async () => {
    getState.mockResolvedValue(
      state([
        step({ completed: true, completed_at: 'x', celebrate: true }),
        step({ key: 'framework', primary: false, order: 2, label_i18n: { fr: 'a', en: 'a' }, hint_i18n: {} }),
      ]),
    );

    renderChecklist();

    await waitFor(() => expect(confetti).toHaveBeenCalledTimes(1));
    expect(markCelebrated).toHaveBeenCalledExactlyOnceWith('first_risk');
  });

  it('does not fire for a completed step the server has already acknowledged', async () => {
    getState.mockResolvedValue(
      state([
        step({ completed: true, completed_at: 'x', celebrate: false }),
        step({ key: 'framework', primary: false, order: 2, label_i18n: { fr: 'a', en: 'a' }, hint_i18n: {} }),
      ]),
    );

    renderChecklist();

    await screen.findByTestId('activation-step-first_risk');
    expect(confetti).not.toHaveBeenCalled();
    expect(markCelebrated).not.toHaveBeenCalled();
  });

  it('falls back to a toast under prefers-reduced-motion', async () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: true,
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })) as unknown as typeof window.matchMedia;

    getState.mockResolvedValue(
      state([
        step({ completed: true, completed_at: 'x', celebrate: true }),
        step({ key: 'framework', primary: false, order: 2, label_i18n: { fr: 'a', en: 'a' }, hint_i18n: {} }),
      ]),
    );

    renderChecklist();

    await waitFor(() => expect(toastSuccess).toHaveBeenCalledTimes(1));
    expect(confetti, 'no motion under reduced-motion').not.toHaveBeenCalled();
    // The milestone is still acknowledged, so it does not re-announce forever.
    expect(markCelebrated).toHaveBeenCalledWith('first_risk');
  });
});

describe('helpers', () => {
  it('nextActionableStep prefers the primary step, then order', () => {
    const s = state([
      step({ key: 'profile', primary: false, order: 1, completed: false }),
      step({ key: 'first_risk', primary: true, order: 2, completed: false }),
    ]);
    expect(nextActionableStep(s)?.key).toBe('first_risk');

    const s2 = state([
      step({ key: 'profile', primary: false, order: 1, completed: false }),
      step({ key: 'first_risk', primary: true, order: 2, completed: true, completed_at: 'x' }),
    ]);
    expect(nextActionableStep(s2)?.key).toBe('profile');

    expect(nextActionableStep(undefined)).toBeUndefined();
  });

  it('i18n falls back rather than rendering an empty label', () => {
    expect(i18n({ fr: 'Bonjour', en: 'Hello' }, 'en')).toBe('Hello');
    expect(i18n({ fr: 'Bonjour' }, 'en')).toBe('Bonjour');
    expect(i18n(undefined, 'fr')).toBe('');
  });
});
