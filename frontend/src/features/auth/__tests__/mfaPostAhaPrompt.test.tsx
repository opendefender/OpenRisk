// OR26-03 step 3 — the post-Aha ask.
//
// The property under test is that the TRIGGER IS THE SERVER'S. The prompt fires
// from activation.aha_reached_at, which the executive dashboard records once per
// tenant from real data; it must not fire because the component decided the
// moment had arrived, and it must not repeat on every navigation.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { MFAPostAhaPrompt } from '../MFAPostAhaPrompt';
import type { MFAStatus } from '../mfaPolicyService';

const fetchMFAStatus = vi.fn();
vi.mock('../mfaPolicyService', async () => {
  const actual = await vi.importActual<typeof import('../mfaPolicyService')>('../mfaPolicyService');
  return { ...actual, fetchMFAStatus: (...a: unknown[]) => fetchMFAStatus(...a) };
});

const useActivationState = vi.fn();
vi.mock('../../onboarding/useActivation', () => ({
  useActivationState: () => useActivationState(),
}));

vi.mock('../MFAEnrollmentDialog', () => ({
  MFAEnrollmentDialog: () => <div data-testid="mfa-dialog" />,
}));

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

function renderPrompt() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MFAPostAhaPrompt />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  fetchMFAStatus.mockResolvedValue(status());
  useActivationState.mockReturnValue({ data: { aha_reached_at: '2026-08-24T10:00:00Z' } });
});

describe('MFAPostAhaPrompt', () => {
  it('asks again once the Aha moment has been reached', async () => {
    renderPrompt();
    expect(await screen.findByTestId('mfa-post-aha-prompt')).toBeInTheDocument();
  });

  it('stays silent until the server says the Aha moment happened', async () => {
    useActivationState.mockReturnValue({ data: { aha_reached_at: null } });
    renderPrompt();

    await waitFor(() => expect(fetchMFAStatus).toHaveBeenCalled());
    expect(screen.queryByTestId('mfa-post-aha-prompt')).not.toBeInTheDocument();
  });

  it('stays silent when MFA is already configured', async () => {
    fetchMFAStatus.mockResolvedValue(status({ state: 'configured', configured: true }));
    renderPrompt();

    await waitFor(() => expect(fetchMFAStatus).toHaveBeenCalled());
    expect(screen.queryByTestId('mfa-post-aha-prompt')).not.toBeInTheDocument();
  });

  it('stays silent once enrolment is already mandatory', async () => {
    // The banner is already an alert and the server is already refusing
    // requests. A modal on top of that is noise stacked on a blocker.
    fetchMFAStatus.mockResolvedValue(status({ state: 'required', required: true, privileged: true }));
    renderPrompt();

    await waitFor(() => expect(fetchMFAStatus).toHaveBeenCalled());
    expect(screen.queryByTestId('mfa-post-aha-prompt')).not.toBeInTheDocument();
  });

  it('does not re-ask within the cooldown', async () => {
    const first = renderPrompt();
    await screen.findByTestId('mfa-post-aha-prompt');
    first.unmount();

    renderPrompt();
    await waitFor(() => expect(fetchMFAStatus).toHaveBeenCalledTimes(2));
    expect(screen.queryByTestId('mfa-post-aha-prompt')).not.toBeInTheDocument();
  });

  it('asks again once the cooldown has elapsed', async () => {
    localStorage.setItem('openrisk_mfa_post_aha_prompt_at', String(Date.now() - 25 * 60 * 60 * 1000));
    renderPrompt();
    expect(await screen.findByTestId('mfa-post-aha-prompt')).toBeInTheDocument();
  });

  it('dismissing it changes nothing about the requirement', async () => {
    renderPrompt();
    await screen.findByTestId('mfa-post-aha-prompt');
    await userEvent.click(screen.getByRole('button', { name: /later|plus tard/i }));

    expect(screen.queryByTestId('mfa-post-aha-prompt')).not.toBeInTheDocument();
    // The only durable artefact is the cooldown timestamp. Nothing here says
    // "this user may skip MFA" — that decision is not the client's to record.
    expect(localStorage.getItem('openrisk_mfa_post_aha_prompt_at')).not.toBeNull();
    const durableKeys = Array.from({ length: localStorage.length }, (_, i) => localStorage.key(i));
    expect(durableKeys).toEqual(['openrisk_mfa_post_aha_prompt_at']);
  });

  it('hands off to the real enrolment flow', async () => {
    renderPrompt();
    await screen.findByTestId('mfa-post-aha-prompt');
    await userEvent.click(screen.getByRole('button', { name: /enable mfa|activer le mfa/i }));
    expect(screen.getByTestId('mfa-dialog')).toBeInTheDocument();
  });
});
