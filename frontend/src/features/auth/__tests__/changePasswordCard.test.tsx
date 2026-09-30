// #720 — Settings › Security › change password.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render as rtlRender, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';

import { ChangePasswordCard } from '../ChangePasswordCard';
import { useUIStore } from '../../../store/uiStore';
import { getAccessToken, setAccessToken } from '../../../lib/session';

const render = (ui: React.ReactElement) =>
  rtlRender(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );

const changePassword = vi.fn();
const fetchHasLocalPassword = vi.fn();
vi.mock('../authService', async () => {
  const actual = await vi.importActual<typeof import('../authService')>('../authService');
  return {
    ...actual,
    changePassword: (...a: unknown[]) => changePassword(...a),
    fetchHasLocalPassword: (...a: unknown[]) => fetchHasLocalPassword(...a),
  };
});

// The meter talks to the server and loads zxcvbn; here it only reports a verdict.
vi.mock('../PasswordStrength', () => ({
  PasswordStrength: ({ onVerdict }: { onVerdict?: (ok: boolean) => void }) => {
    onVerdict?.(true);
    return null;
  },
}));

const toastSuccess = vi.fn();
vi.mock('sonner', () => ({
  toast: { success: (...a: unknown[]) => toastSuccess(...a), error: vi.fn() },
}));

function axiosError(status: number, data: unknown) {
  return Object.assign(new Error('request failed'), {
    isAxiosError: true,
    response: { status, data },
  });
}

async function fill(current: string, next: string, confirm = next) {
  await userEvent.type(await screen.findByLabelText(/^Current password/), current);
  await userEvent.type(screen.getByLabelText(/^New password/), next);
  await userEvent.type(screen.getByLabelText(/^Confirm the new password/), confirm);
  await userEvent.click(screen.getByTestId('change-password-submit'));
}

beforeEach(() => {
  changePassword.mockReset();
  fetchHasLocalPassword.mockReset();
  fetchHasLocalPassword.mockResolvedValue(true);
  toastSuccess.mockReset();
  useUIStore.getState().setLang('en');
});

describe('ChangePasswordCard', () => {
  it('changes the password and clears the form', async () => {
    setAccessToken('old-access');
    changePassword.mockResolvedValue({
      message: 'Password changed.',
      reauthenticate: false,
      token_pair: { access_token: 'fresh-access', refresh_token: 'r', expires_in: 900 },
    });
    render(<ChangePasswordCard />);
    await fill('Old-Password-1234', 'Violet-Kilimanjaro-Anchor-2026!');
    await waitFor(() =>
      expect(changePassword).toHaveBeenCalledWith(
        'Old-Password-1234',
        'Violet-Kilimanjaro-Anchor-2026!',
        'en',
      ),
    );
    expect(toastSuccess).toHaveBeenCalledWith('Password changed.');
    await waitFor(() => expect(screen.getByLabelText(/^Current password/)).toHaveValue(''));
    expect(getAccessToken()).toBe('fresh-access');
  });

  it('refuses a mismatched confirmation before calling the API', async () => {
    render(<ChangePasswordCard />);
    await fill('Old-Password-1234', 'Violet-Kilimanjaro-Anchor-2026!', 'Something-Else-2026!!');
    expect(await screen.findByText('The two passwords do not match.')).toBeInTheDocument();
    expect(changePassword).not.toHaveBeenCalled();
  });

  it('reports a wrong current password on that field', async () => {
    changePassword.mockRejectedValue(
      axiosError(403, {
        code: 'wrong_current_password',
        error: 'The current password is incorrect.',
      }),
    );
    render(<ChangePasswordCard />);
    await fill('Wrong-Password-1234', 'Violet-Kilimanjaro-Anchor-2026!');
    expect(await screen.findByText('The current password is incorrect.')).toBeInTheDocument();
  });

  it('explains instead of offering a form when the identity provider owns the password', async () => {
    changePassword.mockRejectedValue(
      axiosError(409, {
        code: 'no_local_password',
        error: 'This account signs in through your identity provider.',
      }),
    );
    render(<ChangePasswordCard />);
    await fill('anything-long-enough', 'Violet-Kilimanjaro-Anchor-2026!');
    expect(await screen.findByTestId('password-managed-by-idp')).toHaveTextContent(
      'identity provider',
    );
    expect(screen.queryByTestId('change-password-submit')).not.toBeInTheDocument();
  });

  describe('an account without a password in OpenRisk (#850)', () => {
    it('says the identity provider owns the password and offers no fields', async () => {
      fetchHasLocalPassword.mockResolvedValue(false);
      render(<ChangePasswordCard />);

      expect(await screen.findByTestId('password-managed-by-idp')).toHaveTextContent(
        /identity provider/i,
      );
      expect(screen.queryByLabelText(/^Current password/)).not.toBeInTheDocument();
      expect(screen.queryByTestId('change-password-submit')).not.toBeInTheDocument();
      expect(changePassword).not.toHaveBeenCalled();
    });

    it('shows a skeleton, not the form, while it finds out', () => {
      fetchHasLocalPassword.mockReturnValue(new Promise(() => {}));
      render(<ChangePasswordCard />);

      expect(screen.queryByLabelText(/^Current password/)).not.toBeInTheDocument();
      expect(screen.queryByTestId('password-managed-by-idp')).not.toBeInTheDocument();
    });

    it('falls back to the form when it cannot tell, and the server still corrects it', async () => {
      fetchHasLocalPassword.mockRejectedValue(new Error('offline'));
      changePassword.mockRejectedValue(
        axiosError(409, { code: 'no_local_password', error: 'Managed by your identity provider.' }),
      );
      render(<ChangePasswordCard />);
      // One retry (1 s) before the card gives up and shows the form.
      await screen.findByLabelText(/^Current password/, {}, { timeout: 3000 });
      await fill('anything-at-all', 'Violet-Kilimanjaro-Anchor-2026!');

      expect(await screen.findByTestId('password-managed-by-idp')).toHaveTextContent(
        'Managed by your identity provider.',
      );
    });
  });
});
