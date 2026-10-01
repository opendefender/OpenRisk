// #754 — turning MFA off from Settings › Security.
//
// What matters: the button is only offered to an account the server would let
// through, the password is required before anything is sent, a wrong password
// lands on the field, a refusal the field cannot fix is said plainly, and the
// panel only shows "off" once the server agreed.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';

import { MFAAccountPanel } from '../MFAPolicyPanel';
import type { MFAStatus } from '../../auth/mfaPolicyService';

const fetchMFAStatus = vi.fn();
vi.mock('../../auth/mfaPolicyService', async () => {
  const actual = await vi.importActual<typeof import('../../auth/mfaPolicyService')>(
    '../../auth/mfaPolicyService',
  );
  return { ...actual, fetchMFAStatus: (...a: unknown[]) => fetchMFAStatus(...a) };
});

const disableMFA = vi.fn();
const fetchDisableMFAProof = vi.fn();
vi.mock('../../auth/authService', async () => {
  const actual =
    await vi.importActual<typeof import('../../auth/authService')>('../../auth/authService');
  return {
    ...actual,
    disableMFA: (...a: unknown[]) => disableMFA(...a),
    fetchDisableMFAProof: (...a: unknown[]) => fetchDisableMFAProof(...a),
  };
});

vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (sel: (s: { user: { email: string } }) => unknown) =>
    sel({ user: { email: 'rssi@banque.cm' } }),
}));

const toastSuccess = vi.fn();
vi.mock('sonner', () => ({
  toast: { success: (...a: unknown[]) => toastSuccess(...a), error: vi.fn() },
}));

function status(over: Partial<MFAStatus> = {}): MFAStatus {
  return {
    state: 'configured',
    configured: true,
    required: false,
    privileged: false,
    grace_period_active: false,
    grace_days: 7,
    ...over,
  } as MFAStatus;
}

function refusal(code: string, error: string, httpStatus: number): AxiosError {
  const response = {
    status: httpStatus,
    data: { code, error },
    statusText: '',
    headers: {},
    config: { headers: new AxiosHeaders() },
  } as AxiosResponse;
  return new AxiosError(error, String(httpStatus), undefined, undefined, response);
}

function renderPanel() {
  // Mutation retries as in src/main.tsx, so a replayed password guess shows up.
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: 3, retryDelay: 0 } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <MFAAccountPanel />
    </QueryClientProvider>,
  );
}

async function openDialog() {
  await userEvent.click(await screen.findByTestId('mfa-disable-open'));
  return screen.findByTestId('mfa-disable-password');
}

beforeEach(() => {
  vi.clearAllMocks();
  fetchMFAStatus.mockResolvedValue(status());
  fetchDisableMFAProof.mockResolvedValue('password');
});

describe('turning MFA off', () => {
  it('offers no button to a role the server would refuse', async () => {
    fetchMFAStatus.mockResolvedValue(status({ privileged: true }));
    renderPanel();

    expect(await screen.findByTestId('mfa-disable-locked')).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-disable-open')).not.toBeInTheDocument();
  });

  it('sends nothing without a password', async () => {
    renderPanel();
    await openDialog();

    await userEvent.click(screen.getByTestId('mfa-disable-submit'));

    expect(
      await screen.findByText(/enter your password|saisissez votre mot de passe/i),
    ).toBeInTheDocument();
    expect(disableMFA).not.toHaveBeenCalled();
  });

  it('reports a wrong password on the field and keeps MFA shown as on', async () => {
    disableMFA.mockRejectedValue(refusal('wrong_password', 'Incorrect password.', 401));
    renderPanel();
    await userEvent.type(await openDialog(), 'guess-1234');

    await userEvent.click(screen.getByTestId('mfa-disable-submit'));

    expect(await screen.findByText('Incorrect password.')).toBeInTheDocument();
    expect(screen.getByTestId('mfa-disable-password')).toHaveAttribute('aria-invalid', 'true');
    expect(document.body.textContent).toMatch(/MFA is enabled|Le MFA est activé/);
  });

  it('sends a wrong password once, never replays it against the attempt budget', async () => {
    disableMFA.mockRejectedValue(refusal('wrong_password', 'Incorrect password.', 401));
    renderPanel();
    await userEvent.type(await openDialog(), 'guess-1234');

    await userEvent.click(screen.getByTestId('mfa-disable-submit'));

    expect(await screen.findByText('Incorrect password.')).toBeInTheDocument();
    expect(disableMFA).toHaveBeenCalledTimes(1);
  });

  it('says plainly when the server refuses for a reason the password cannot fix', async () => {
    disableMFA.mockRejectedValue(
      refusal('too_many_attempts', 'Too many attempts. Try again in a few minutes.', 429),
    );
    renderPanel();
    await userEvent.type(await openDialog(), 'Ancre-Vitrail7-Cobalt');

    await userEvent.click(screen.getByTestId('mfa-disable-submit'));

    expect((await screen.findByTestId('mfa-disable-blocked')).textContent).toMatch(
      /too many attempts/i,
    );
    expect(screen.getByTestId('mfa-disable-submit')).toBeDisabled();
  });

  it('sends the password and flips the panel once the server agrees', async () => {
    disableMFA.mockResolvedValue({ message: 'Two-factor authentication turned off.' });
    fetchMFAStatus
      .mockResolvedValueOnce(status())
      .mockResolvedValue(status({ state: 'recommended', configured: false }));
    renderPanel();
    await userEvent.type(await openDialog(), 'Ancre-Vitrail7-Cobalt');

    await userEvent.click(screen.getByTestId('mfa-disable-submit'));

    await waitFor(() =>
      expect(disableMFA).toHaveBeenCalledWith(
        { password: 'Ancre-Vitrail7-Cobalt' },
        expect.any(String),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByTestId('mfa-disable-password')).not.toBeInTheDocument(),
    );
    expect(toastSuccess).toHaveBeenCalledWith('Two-factor authentication turned off.');
    expect(
      await screen.findByRole('button', { name: /enable mfa|activer le mfa/i }),
    ).toBeInTheDocument();
  });

  describe('an account that signs in through an identity provider', () => {
    it('asks for an authenticator code, not a password, and sends the code', async () => {
      fetchDisableMFAProof.mockResolvedValue('code');
      disableMFA.mockResolvedValue({ message: 'Two-factor authentication turned off.' });
      renderPanel();
      await userEvent.click(await screen.findByTestId('mfa-disable-open'));

      const code = await screen.findByTestId('mfa-disable-code');
      expect(screen.queryByTestId('mfa-disable-password')).not.toBeInTheDocument();
      await userEvent.type(code, '123456');
      await userEvent.click(screen.getByTestId('mfa-disable-submit'));

      await waitFor(() =>
        expect(disableMFA).toHaveBeenCalledWith({ code: '123456' }, expect.any(String)),
      );
    });

    it('sends nothing until the code has six digits', async () => {
      fetchDisableMFAProof.mockResolvedValue('code');
      renderPanel();
      await userEvent.click(await screen.findByTestId('mfa-disable-open'));
      await userEvent.type(await screen.findByTestId('mfa-disable-code'), '123');

      await userEvent.click(screen.getByTestId('mfa-disable-submit'));

      expect((await screen.findByRole('alert')).textContent).toMatch(/6-digit code|6 chiffres/i);
      expect(disableMFA).not.toHaveBeenCalled();
    });

    it('reports a wrong code on the code field', async () => {
      fetchDisableMFAProof.mockResolvedValue('code');
      disableMFA.mockRejectedValue(refusal('wrong_code', 'Incorrect code.', 401));
      renderPanel();
      await userEvent.click(await screen.findByTestId('mfa-disable-open'));
      await userEvent.type(await screen.findByTestId('mfa-disable-code'), '000000');

      await userEvent.click(screen.getByTestId('mfa-disable-submit'));

      expect(await screen.findByText('Incorrect code.')).toBeInTheDocument();
      expect(disableMFA).toHaveBeenCalledTimes(1);
    });

    it('switches to the code when the server says the account has no password', async () => {
      // /auth/me could not be read, so the dialog guessed "password".
      fetchDisableMFAProof.mockRejectedValue(new Error('offline'));
      disableMFA.mockRejectedValue(refusal('wrong_code', 'Incorrect code.', 401));
      renderPanel();
      await userEvent.click(await screen.findByTestId('mfa-disable-open'));
      // One retry (1 s) before the dialog falls back to asking for a password.
      await userEvent.type(
        await screen.findByTestId('mfa-disable-password', {}, { timeout: 3000 }),
        'not-mine',
      );

      await userEvent.click(screen.getByTestId('mfa-disable-submit'));

      expect(await screen.findByTestId('mfa-disable-code')).toBeInTheDocument();
      expect(screen.queryByTestId('mfa-disable-password')).not.toBeInTheDocument();
    });
  });
});
