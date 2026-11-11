// #1032 — regenerating MFA backup codes from Settings › Security.
//
// What matters: the button is offered whenever MFA is on, including to a role
// that may not turn it off; nothing is sent without a six-digit code (and the
// password, for an account that has one); a refusal lands on the right field
// and is sent once; the new codes are shown once with copy and download; and
// an empty answer is said, not shown as an empty grid.

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

const regenerateBackupCodes = vi.fn();
const fetchHasLocalPassword = vi.fn();
vi.mock('../../auth/authService', async () => {
  const actual =
    await vi.importActual<typeof import('../../auth/authService')>('../../auth/authService');
  return {
    ...actual,
    regenerateBackupCodes: (...a: unknown[]) => regenerateBackupCodes(...a),
    fetchHasLocalPassword: (...a: unknown[]) => fetchHasLocalPassword(...a),
  };
});

vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (sel: (s: { user: { email: string } }) => unknown) =>
    sel({ user: { email: 'rssi@banque.cm' } }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock('sonner', () => ({
  toast: {
    success: (...a: unknown[]) => toastSuccess(...a),
    error: (...a: unknown[]) => toastError(...a),
  },
}));

const CODES = [
  'AAAA-1111',
  'BBBB-2222',
  'CCCC-3333',
  'DDDD-4444',
  'EEEE-5555',
  'FFFF-6666',
  'GGGG-7777',
  'HHHH-8888',
];

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
  // Mutation retries as in src/main.tsx, so a replayed guess shows up.
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
  await userEvent.click(await screen.findByTestId('mfa-regenerate-open'));
}

async function fill(password: string | null, code: string) {
  if (password !== null) {
    await userEvent.type(await screen.findByTestId('mfa-regenerate-password'), password);
  }
  await userEvent.type(await screen.findByTestId('mfa-regenerate-code'), code);
  await userEvent.click(screen.getByTestId('mfa-regenerate-submit'));
}

beforeEach(() => {
  vi.clearAllMocks();
  fetchMFAStatus.mockResolvedValue(status());
  fetchHasLocalPassword.mockResolvedValue(true);
});

describe('regenerating backup codes', () => {
  it('is offered to a role that may not turn MFA off', async () => {
    fetchMFAStatus.mockResolvedValue(status({ privileged: true }));
    renderPanel();

    expect(await screen.findByTestId('mfa-regenerate-open')).toBeInTheDocument();
    expect(screen.getByTestId('mfa-disable-locked')).toBeInTheDocument();
  });

  it('is not offered while MFA is off', async () => {
    fetchMFAStatus.mockResolvedValue(status({ state: 'recommended', configured: false }));
    renderPanel();

    expect(
      await screen.findByRole('button', { name: /enable mfa|activer le mfa/i }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-regenerate-open')).not.toBeInTheDocument();
  });

  it('shows a skeleton while it learns which proofs to ask for', async () => {
    fetchHasLocalPassword.mockReturnValue(new Promise(() => {}));
    renderPanel();
    await openDialog();

    expect(await screen.findByTestId('mfa-regenerate-submit')).toBeDisabled();
    expect(screen.queryByTestId('mfa-regenerate-code')).not.toBeInTheDocument();
  });

  it('sends nothing without the password and a six-digit code', async () => {
    renderPanel();
    await openDialog();
    await fill(null, '123');

    expect(
      await screen.findByText(/enter your password|saisissez votre mot de passe/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/^(Enter the 6-digit code\.|Saisissez les 6 chiffres du code\.)$/),
    ).toBeInTheDocument();
    expect(regenerateBackupCodes).not.toHaveBeenCalled();
  });

  it('sends the password and the code, then shows the codes once with copy and download', async () => {
    regenerateBackupCodes.mockResolvedValue({ backup_codes: CODES });
    renderPanel();
    await openDialog();
    await fill('Ancre-Vitrail7-Cobalt', '123456');

    await waitFor(() =>
      expect(regenerateBackupCodes).toHaveBeenCalledWith(
        { password: 'Ancre-Vitrail7-Cobalt', code: '123456' },
        expect.any(String),
      ),
    );
    const list = await screen.findByTestId('mfa-regenerate-codes');
    for (const c of CODES) expect(list.textContent).toContain(c);
    expect(screen.getByTestId('mfa-regenerate-copy')).toBeInTheDocument();
    expect(screen.getByTestId('mfa-regenerate-download')).toBeInTheDocument();

    await userEvent.click(screen.getByTestId('mfa-regenerate-done'));
    await waitFor(() =>
      expect(screen.queryByTestId('mfa-regenerate-codes')).not.toBeInTheDocument(),
    );
  });

  it('copies the codes to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    regenerateBackupCodes.mockResolvedValue({ backup_codes: CODES });
    renderPanel();
    await openDialog();
    await fill('Ancre-Vitrail7-Cobalt', '123456');
    await screen.findByTestId('mfa-regenerate-codes');
    // user-event installs its own clipboard stub on setup; replace it after.
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });

    await userEvent.click(screen.getByTestId('mfa-regenerate-copy'));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith(CODES.join('\n')));
  });

  it('says so when the server returns no codes', async () => {
    regenerateBackupCodes.mockResolvedValue({ backup_codes: [] });
    renderPanel();
    await openDialog();
    await fill('Ancre-Vitrail7-Cobalt', '123456');

    expect(await screen.findByTestId('mfa-regenerate-empty')).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-regenerate-copy')).not.toBeInTheDocument();
  });

  it('reports a wrong password on the field, sent once', async () => {
    regenerateBackupCodes.mockRejectedValue(refusal('wrong_password', 'Incorrect password.', 401));
    renderPanel();
    await openDialog();
    await fill('guess-1234', '123456');

    expect(await screen.findByText('Incorrect password.')).toBeInTheDocument();
    expect(screen.getByTestId('mfa-regenerate-password')).toHaveAttribute('aria-invalid', 'true');
    expect(regenerateBackupCodes).toHaveBeenCalledTimes(1);
  });

  it('reports a wrong code on the code field', async () => {
    regenerateBackupCodes.mockRejectedValue(refusal('wrong_code', 'Incorrect code.', 401));
    renderPanel();
    await openDialog();
    await fill('Ancre-Vitrail7-Cobalt', '000000');

    expect(await screen.findByText('Incorrect code.')).toBeInTheDocument();
    expect(regenerateBackupCodes).toHaveBeenCalledTimes(1);
  });

  it('says plainly when the attempt budget is spent', async () => {
    regenerateBackupCodes.mockRejectedValue(
      refusal('too_many_attempts', 'Too many attempts. Try again in a few minutes.', 429),
    );
    renderPanel();
    await openDialog();
    await fill('Ancre-Vitrail7-Cobalt', '123456');

    expect((await screen.findByTestId('mfa-regenerate-blocked')).textContent).toMatch(
      /too many attempts/i,
    );
    expect(screen.getByTestId('mfa-regenerate-submit')).toBeDisabled();
  });

  it('says the old codes still work when the server fails', async () => {
    regenerateBackupCodes.mockRejectedValue(new Error('network'));
    renderPanel();
    await openDialog();
    await fill('Ancre-Vitrail7-Cobalt', '123456');

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        expect.stringMatching(/old codes still work|anciens codes restent/i),
      ),
    );
  });

  it('asks only for the code on an account without a password', async () => {
    fetchHasLocalPassword.mockResolvedValue(false);
    regenerateBackupCodes.mockResolvedValue({ backup_codes: CODES });
    renderPanel();
    await openDialog();
    await screen.findByTestId('mfa-regenerate-code');
    expect(screen.queryByTestId('mfa-regenerate-password')).not.toBeInTheDocument();

    await fill(null, '123456');

    await waitFor(() =>
      expect(regenerateBackupCodes).toHaveBeenCalledWith({ code: '123456' }, expect.any(String)),
    );
  });
});
