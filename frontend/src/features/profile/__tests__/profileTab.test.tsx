// #719 — Settings › My profile.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { ProfileTab } from '../ProfileTab';
import { MY_PROFILE_KEY } from '../useProfile';
import type { MyProfile } from '../profileService';

const getMe = vi.fn();
const updateMe = vi.fn();
const uploadAvatar = vi.fn();
vi.mock('../profileService', () => ({
  profileService: {
    getMe: () => getMe(),
    updateMe: (...a: unknown[]) => updateMe(...a),
    uploadAvatar: (...a: unknown[]) => uploadAvatar(...a),
    deleteAvatar: vi.fn(),
    getAvatarBlob: vi.fn(),
  },
}));

const toastError = vi.fn();
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: (...a: unknown[]) => toastError(...a) },
}));

const tr = (_fr: string, en: string) => en;

function me(over: Partial<MyProfile> = {}): MyProfile {
  return {
    id: 'u1',
    email: 'alice@a.io',
    username: 'alice',
    full_name: 'Alice',
    job_title: '',
    phone: '',
    bio: '',
    timezone: '',
    locale: '',
    date_format: '',
    theme_mode: '',
    has_avatar: false,
    effective: { locale: 'fr', timezone: 'Africa/Douala' },
    updated_at: '2026-09-17T00:00:00Z',
    ...over,
  };
}

function renderTab() {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={qc}>
      <ProfileTab tr={tr} />
    </QueryClientProvider>,
  );
  return qc;
}

beforeEach(() => {
  getMe.mockReset();
  updateMe.mockReset();
  uploadAvatar.mockReset();
  toastError.mockReset();
});

describe('ProfileTab', () => {
  it('shows the error state with a retry when the profile cannot load', async () => {
    getMe.mockRejectedValue(new Error('down'));
    renderTab();
    expect(await screen.findByText('Could not load your profile')).toBeInTheDocument();
  });

  it('saves identity and preferences, and keeps email read-only', async () => {
    getMe.mockResolvedValue(me());
    updateMe.mockResolvedValue(me({ full_name: 'Alice Mbarga', theme_mode: 'dark' }));
    const qc = renderTab();

    const name = await screen.findByLabelText(/^Full name/);
    expect(screen.getByDisplayValue('alice@a.io')).toBeDisabled();
    await userEvent.clear(name);
    await userEvent.type(name, 'Alice Mbarga');
    await userEvent.selectOptions(screen.getByLabelText('Theme'), 'dark');
    await userEvent.click(screen.getByTestId('profile-save'));

    await waitFor(() => expect(updateMe).toHaveBeenCalledTimes(1));
    expect(updateMe.mock.calls[0][0]).toMatchObject({
      full_name: 'Alice Mbarga',
      theme_mode: 'dark',
    });
    expect(updateMe.mock.calls[0][0]).not.toHaveProperty('email');
    await waitFor(() =>
      expect(qc.getQueryData<MyProfile>(MY_PROFILE_KEY)?.full_name).toBe('Alice Mbarga'),
    );
  });

  it('refuses an invalid phone number before calling the API', async () => {
    getMe.mockResolvedValue(me());
    renderTab();
    await userEvent.type(await screen.findByLabelText('Phone'), 'call me');
    await userEvent.click(screen.getByTestId('profile-save'));
    expect(await screen.findByText('Invalid phone number.')).toBeInTheDocument();
    expect(updateMe).not.toHaveBeenCalled();
  });

  it('rolls back and shows the server message when the save is refused', async () => {
    getMe.mockResolvedValue(me());
    updateMe.mockRejectedValue({
      isAxiosError: true,
      response: { status: 400, data: { error: 'timezone: unknown IANA time zone' } },
    });
    const qc = renderTab();
    const name = await screen.findByLabelText(/^Full name/);
    await userEvent.clear(name);
    await userEvent.type(name, 'Renamed');
    await userEvent.click(screen.getByTestId('profile-save'));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(qc.getQueryData<MyProfile>(MY_PROFILE_KEY)?.full_name).toBe('Alice');
  });

  it('refuses an SVG avatar in the browser without uploading it', async () => {
    getMe.mockResolvedValue(me());
    renderTab();
    const input = await screen.findByTestId('avatar-input');
    const svg = new File(['<svg/>'], 'a.svg', { type: 'image/svg+xml' });
    await userEvent.upload(input, svg, { applyAccept: false });
    expect(toastError).toHaveBeenCalled();
    expect(uploadAvatar).not.toHaveBeenCalled();
  });
});
