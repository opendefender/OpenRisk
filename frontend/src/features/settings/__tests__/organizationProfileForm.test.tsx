// #299 — Settings › General › organization profile.
//
// What a profile form gets wrong: offering a form to somebody the server would
// refuse, accepting a value the API rejects, and leaving the optimistic value on
// screen after the server said no.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { OrganizationProfileForm } from '../OrganizationProfileForm';
import { ORG_KEY } from '../../organization/useOrganization';
import type { OrganizationView } from '../../organization/organizationService';

const updateOrganization = vi.fn();
vi.mock('../../organization/organizationService', async () => {
  const actual = await vi.importActual<typeof import('../../organization/organizationService')>(
    '../../organization/organizationService',
  );
  return {
    ...actual,
    organizationService: {
      ...actual.organizationService,
      updateOrganization: (...a: unknown[]) => updateOrganization(...a),
    },
  };
});

const toastError = vi.fn();
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: (...a: unknown[]) => toastError(...a) },
}));

const tr = (_fr: string, en: string) => en;

function org(over: Partial<OrganizationView> = {}): OrganizationView {
  return {
    id: 'o1',
    name: 'Banque Atlantique',
    slug: 'banque-atlantique',
    plan: 'professional',
    is_active: true,
    owner_id: 'u1',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    counts: {
      total_members: 3,
      active_members: 3,
      deactivated_members: 0,
      revoked_members: 0,
      admins: 1,
      pending_invitations: 0,
    },
    can_edit: true,
    ...over,
  };
}

function renderForm(view: OrganizationView) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  qc.setQueryData(ORG_KEY, view);
  render(
    <QueryClientProvider client={qc}>
      <OrganizationProfileForm org={view} tr={tr} />
    </QueryClientProvider>,
  );
  return qc;
}

beforeEach(() => {
  updateOrganization.mockReset();
  toastError.mockReset();
});

describe('OrganizationProfileForm', () => {
  it('saves the edited profile and keeps the server answer', async () => {
    const saved = org({ name: 'Banque Atlantique Cameroun', website: 'https://ba.cm' });
    updateOrganization.mockResolvedValue(saved);
    const qc = renderForm(org());

    const name = screen.getByLabelText(/^Name/);
    await userEvent.clear(name);
    await userEvent.type(name, 'Banque Atlantique Cameroun');
    await userEvent.type(screen.getByLabelText('Website'), 'https://ba.cm');
    await userEvent.click(screen.getByTestId('org-profile-save'));

    await waitFor(() => expect(updateOrganization).toHaveBeenCalledTimes(1));
    expect(updateOrganization.mock.calls[0][0]).toMatchObject({
      name: 'Banque Atlantique Cameroun',
      website: 'https://ba.cm',
    });
    await waitFor(() =>
      expect(qc.getQueryData<OrganizationView>(ORG_KEY)?.name).toBe('Banque Atlantique Cameroun'),
    );
  });

  it('refuses an insecure website before calling the API', async () => {
    renderForm(org());
    await userEvent.type(screen.getByLabelText('Website'), 'http://ba.cm');
    await userEvent.click(screen.getByTestId('org-profile-save'));

    expect(await screen.findByText('A full https:// address is expected.')).toBeInTheDocument();
    expect(updateOrganization).not.toHaveBeenCalled();
  });

  it('rolls the cached profile back and shows the server message when the save fails', async () => {
    updateOrganization.mockRejectedValue({
      isAxiosError: true,
      response: { status: 400, data: { error: 'timezone: unknown IANA time zone' } },
    });
    const qc = renderForm(org());

    const name = screen.getByLabelText(/^Name/);
    await userEvent.clear(name);
    await userEvent.type(name, 'Renamed');
    await userEvent.click(screen.getByTestId('org-profile-save'));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(qc.getQueryData<OrganizationView>(ORG_KEY)?.name).toBe('Banque Atlantique');
  });
});
