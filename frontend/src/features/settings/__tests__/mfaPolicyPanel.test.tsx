// OR26-03 — Settings › Security › MFA policy.
//
// The properties that matter here are the ones a settings form gets wrong:
// validating against the server's bounds rather than a hard-coded copy, telling
// "saved" from "using the default", and not offering a control to somebody the
// server would refuse.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { MFAPolicyPanel } from '../MFAPolicyPanel';
import type { MFAPolicy } from '../../auth/mfaPolicyService';

const fetchMFAPolicy = vi.fn();
const saveMFAPolicy = vi.fn();
vi.mock('../../auth/mfaPolicyService', async () => {
  const actual = await vi.importActual<typeof import('../../auth/mfaPolicyService')>(
    '../../auth/mfaPolicyService',
  );
  return {
    ...actual,
    fetchMFAPolicy: (...a: unknown[]) => fetchMFAPolicy(...a),
    saveMFAPolicy: (...a: unknown[]) => saveMFAPolicy(...a),
    fetchMFAStatus: vi.fn().mockResolvedValue(null),
  };
});

const hasPermission = vi.fn();
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (sel: (s: { hasPermission: (p: string) => boolean }) => unknown) =>
    sel({ hasPermission: (p: string) => hasPermission(p) }),
}));

vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function policy(over: Partial<MFAPolicy> = {}): MFAPolicy {
  return {
    grace_days: 7,
    configured: false,
    min_days: 0,
    max_days: 90,
    default_days: 7,
    privileged_org_roles: ['admin', 'root'],
    privileged_business_roles: ['rssi'],
    ...over,
  } as MFAPolicy;
}

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MFAPolicyPanel />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  hasPermission.mockReturnValue(true);
  fetchMFAPolicy.mockResolvedValue(policy());
});

describe('MFAPolicyPanel', () => {
  it('shows the shipped default and says nobody chose it', async () => {
    renderPanel();
    const input = (await screen.findByRole('spinbutton')) as HTMLInputElement;
    // The panel seeds this field from the server in an effect, not at render, so
    // there is exactly one frame where the spinbutton exists holding ''.
    // findByRole resolves on that frame — wait for the VALUE, not the element.
    await waitFor(() => expect(input.value).toBe('7'));
    expect(document.body.textContent).toMatch(/default value|valeur par défaut/i);
  });

  it('names who the window applies to, so the consequence is not left to guesswork', async () => {
    renderPanel();
    await screen.findByRole('spinbutton');
    expect(document.body.textContent).toMatch(/admin, root, rssi/);
    expect(document.body.textContent).toMatch(/recommendation|recommandation/i);
  });

  it('saves a new window', async () => {
    saveMFAPolicy.mockResolvedValue(policy({ grace_days: 3, configured: true }));
    renderPanel();

    const input = await screen.findByRole('spinbutton');
    await userEvent.clear(input);
    await userEvent.type(input, '3');
    await userEvent.click(screen.getByRole('button', { name: /save|enregistrer/i }));

    await waitFor(() => expect(saveMFAPolicy).toHaveBeenCalledWith(3));
  });

  it('refuses a value outside the bounds the SERVER shipped', async () => {
    // Validating against min_days/max_days from the response rather than a
    // hard-coded copy is what stops the form and the API from drifting apart.
    fetchMFAPolicy.mockResolvedValue(policy({ min_days: 0, max_days: 30 }));
    renderPanel();

    const input = await screen.findByRole('spinbutton');
    await userEvent.clear(input);
    await userEvent.type(input, '45');

    expect(screen.getByRole('alert').textContent).toMatch(/between 0 and 30|entre 0 et 30/i);
    expect(screen.getByRole('button', { name: /save|enregistrer/i })).toBeDisabled();
    expect(saveMFAPolicy).not.toHaveBeenCalled();
  });

  it('keeps save disabled until something actually changed', async () => {
    renderPanel();
    await screen.findByRole('spinbutton');
    expect(screen.getByRole('button', { name: /save|enregistrer/i })).toBeDisabled();
  });

  it('explains what zero days means rather than showing a bare number', async () => {
    fetchMFAPolicy.mockResolvedValue(policy({ grace_days: 0, configured: true }));
    renderPanel();
    await screen.findByRole('spinbutton');
    expect(document.body.textContent).toMatch(/first sign-in|première connexion/i);
  });

  it('shows a non-admin the value read-only instead of a control the server would refuse', async () => {
    hasPermission.mockReturnValue(false);
    renderPanel();

    const input = await screen.findByRole('spinbutton');
    expect(input).toBeDisabled();
    expect(screen.queryByRole('button', { name: /save|enregistrer/i })).not.toBeInTheDocument();
    expect(document.body.textContent).toMatch(/administrator|administrateur/i);
  });

  it('offers a retry when the policy cannot be read', async () => {
    fetchMFAPolicy.mockRejectedValue(new Error('boom'));
    renderPanel();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /retry|réessayer/i })).toBeInTheDocument(),
    );
  });
});
