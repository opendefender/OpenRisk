// #719 — server preferences reach the interface at sign-in.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { PreferencesSync } from '../PreferencesSync';
import { usePreferenceStore } from '../../../shared/preferences/preferenceStore';
import { useUIStore } from '../../../store/uiStore';
import type { MyProfile } from '../profileService';

const getMe = vi.fn();
vi.mock('../profileService', () => ({
  profileService: { getMe: () => getMe() },
}));

function profile(over: Partial<MyProfile>): MyProfile {
  return {
    id: 'u1',
    email: 'a@a.io',
    username: 'a',
    full_name: 'A',
    job_title: '',
    phone: '',
    bio: '',
    timezone: '',
    locale: '',
    date_format: '',
    theme_mode: '',
    has_avatar: false,
    effective: {},
    updated_at: '2026-09-17T00:00:00Z',
    ...over,
  };
}

function mount() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <PreferencesSync />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  getMe.mockReset();
  usePreferenceStore.getState().clear();
  useUIStore.getState().setLang('fr');
  useUIStore.getState().setThemeMode('light');
});

describe('PreferencesSync', () => {
  it('applies theme, effective language and date preferences from the server', async () => {
    getMe.mockResolvedValue(
      profile({
        theme_mode: 'dark',
        effective: { locale: 'en', timezone: 'Africa/Douala', date_format: 'YYYY-MM-DD' },
      }),
    );
    mount();
    await waitFor(() => expect(useUIStore.getState().themeMode).toBe('dark'));
    expect(useUIStore.getState().lang).toBe('en');
    expect(usePreferenceStore.getState()).toMatchObject({
      timeZone: 'Africa/Douala',
      pattern: 'YYYY-MM-DD',
    });
  });

  it('leaves the device choices alone when the person set nothing', async () => {
    getMe.mockResolvedValue(profile({ effective: {} }));
    mount();
    await waitFor(() => expect(getMe).toHaveBeenCalled());
    expect(useUIStore.getState().themeMode).toBe('light');
    expect(useUIStore.getState().lang).toBe('fr');
  });

  it('never applies a locale that is not enabled', async () => {
    getMe.mockResolvedValue(profile({ effective: { locale: 'ar' } }));
    mount();
    await waitFor(() => expect(getMe).toHaveBeenCalled());
    expect(useUIStore.getState().lang).toBe('fr');
  });
});
