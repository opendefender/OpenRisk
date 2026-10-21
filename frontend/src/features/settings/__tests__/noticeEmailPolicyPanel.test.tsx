// #997 — Settings › Notifications › "E-mails d'avis".
//
// The role gate is the real useAuthStore.hasRole, fed a signed-in user: a
// stubbed gate is how a panel ships that nobody can use (#867). Only the HTTP
// service is mocked.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { NoticeEmailPolicyPanel } from '../NoticeEmailPolicyPanel';
import { windowSummary } from '../noticeEmailWindow';
import { useAuthStore } from '../../../hooks/useAuthStore';
import type { NoticeEmailPolicy } from '../../notifications/notificationService';

const getEmailPolicy = vi.fn();
const updateEmailPolicy = vi.fn();
vi.mock('../../notifications/notificationService', async () => {
  const actual = await vi.importActual<typeof import('../../notifications/notificationService')>(
    '../../notifications/notificationService',
  );
  return {
    ...actual,
    notificationService: {
      ...actual.notificationService,
      getEmailPolicy: () => getEmailPolicy(),
      updateEmailPolicy: (m: number) => updateEmailPolicy(m),
    },
  };
});

const toastError = vi.fn();
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: (m: string) => toastError(m) } }));

const fr = (f: string) => f;

function policy(over: Partial<NoticeEmailPolicy> = {}): NoticeEmailPolicy {
  return { window_minutes: 60, configured: false, options: [15, 30, 60, 240, 1440], default_minutes: 60, ...over };
}

function signInAs(orgRole: string) {
  useAuthStore.setState({
    user: {
      id: 'u',
      email: 'u@example.test',
      username: 'u',
      full_name: 'U',
      role: '',
      tenant_id: 'org-a',
      org_roles: { 'org-a': orgRole },
      permissions: [],
    },
  });
}

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  render(
    <QueryClientProvider client={qc}>
      <NoticeEmailPolicyPanel tr={fr} />
    </QueryClientProvider>,
  );
}

const radio = (name: string) => screen.getByRole('radio', { name: new RegExp(`^${name}`) });

beforeEach(() => {
  vi.clearAllMocks();
  getEmailPolicy.mockResolvedValue(policy());
});

describe('NoticeEmailPolicyPanel', () => {
  it('shows the default and says nobody chose it', async () => {
    signInAs('admin');
    renderPanel();
    await screen.findByText('Valeur par défaut');
    expect((radio('1 h') as HTMLInputElement).checked).toBe(true);
    expect(screen.getAllByRole('radio')).toHaveLength(5);
    expect(document.body.textContent).toContain('Au plus un e-mail par élément et par personne toutes les heures.');
  });

  it('saves a choice at once and keeps it when the server agrees', async () => {
    signInAs('admin');
    updateEmailPolicy.mockResolvedValue(policy({ window_minutes: 240, configured: true }));
    renderPanel();
    await screen.findByText('Valeur par défaut');

    await userEvent.click(radio('4 h'));
    await waitFor(() => expect(updateEmailPolicy).toHaveBeenCalledWith(240));
    await screen.findByText("Réglage de l'organisation");
    expect((radio('4 h') as HTMLInputElement).checked).toBe(true);
    expect(document.body.textContent).toContain('toutes les 4 heures');
  });

  it('shows the choice before the server answers, and puts the old one back when it refuses', async () => {
    signInAs('root');
    let refuse: (e: Error) => void = () => {};
    updateEmailPolicy.mockReturnValue(new Promise((_, reject) => (refuse = reject)));
    renderPanel();
    await screen.findByText('Valeur par défaut');

    await userEvent.click(radio('15 min'));
    await waitFor(() => expect((radio('15 min') as HTMLInputElement).checked).toBe(true));
    refuse(new Error('403'));
    await waitFor(() => expect((radio('1 h') as HTMLInputElement).checked).toBe(true));
    expect(toastError).toHaveBeenCalledWith(expect.stringContaining('le réglage précédent est rétabli'));
  });

  it('is read-only for a member who is not an administrator', async () => {
    signInAs('user');
    renderPanel();
    await screen.findByText('Valeur par défaut');
    for (const r of screen.getAllByRole('radio')) expect(r).toBeDisabled();
    expect(document.body.textContent).toContain('Seul un administrateur');
    await userEvent.click(radio('4 h'));
    expect(updateEmailPolicy).not.toHaveBeenCalled();
  });

  it('offers a retry when the setting cannot be read', async () => {
    signInAs('admin');
    getEmailPolicy.mockRejectedValueOnce(new Error('500')).mockResolvedValueOnce(policy({ configured: true }));
    renderPanel();
    await userEvent.click(await screen.findByRole('button', { name: 'Réessayer' }));
    await screen.findByText("Réglage de l'organisation");
  });

  it('words each window plainly', () => {
    const en = (_f: string, e: string) => e;
    expect(windowSummary(15, fr)).toContain('toutes les 15 minutes');
    expect(windowSummary(1440, fr)).toContain('par jour');
    expect(windowSummary(30, en)).toContain('every 30 minutes');
    expect(windowSummary(60, en)).toContain('every hour');
  });
});
