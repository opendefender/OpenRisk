// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #296 — the sidebar organization block lists the organizations the account
// belongs to and switches between them. It used to wear a switcher's chevron
// and only open /settings.

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router';

import type { RBACCatalog } from '../../../features/rbac/rbacService';
import type { MembershipSummary } from '../../../features/organization/orgSwitchService';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { useUIStore } from '../../../store/uiStore';

const listMine = vi.fn();
const getCatalog = vi.fn();

vi.mock('../../../features/organization/orgSwitchService', () => ({
  orgSwitchService: { listMine: (...a: unknown[]) => listMine(...a), switchTo: vi.fn() },
}));
vi.mock('../../../features/rbac/rbacService', async () => {
  const actual = await vi.importActual<typeof import('../../../features/rbac/rbacService')>(
    '../../../features/rbac/rbacService',
  );
  return { ...actual, rbacService: { getCatalog: (...a: unknown[]) => getCatalog(...a) } };
});
vi.mock('../../../services/entitlementService', async () => {
  const actual = await vi.importActual<typeof import('../../../services/entitlementService')>(
    '../../../services/entitlementService',
  );
  return {
    ...actual,
    entitlementService: { ...actual.entitlementService, get: vi.fn(() => new Promise(() => {})) },
  };
});

import { OrgSwitcher } from '../OrgSwitcher';

const ORG_A = 'org-a';
const ORG_B = 'org-b';

const catalog: RBACCatalog = {
  permissions: [],
  business_roles: [
    {
      key: 'viewer',
      label_fr: 'Lecteur',
      label_en: 'Viewer',
      description_fr: '',
      description_en: '',
      permissions: [],
      default_landing: '/',
    },
  ],
};

const rows: MembershipSummary[] = [
  {
    organization_id: ORG_A,
    name: 'Org A',
    slug: 'org-a',
    role: 'user',
    business_role: 'viewer',
    is_default: false,
  },
  { organization_id: ORG_B, name: 'Org B', slug: 'org-b', role: 'root', is_default: true },
];

const switchOrganization = vi.fn();

function renderSwitcher(onSwitched = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/risks']}>
        <Routes>
          <Route path="/risks" element={<OrgSwitcher orgName="Org A" onSwitched={onSwitched} />} />
          <Route path="/settings" element={<div>settings page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { onSwitched };
}

const trigger = () => screen.getByRole('button', { name: /Switch organization/ });

describe('OrgSwitcher', () => {
  beforeEach(() => {
    listMine.mockReset();
    getCatalog.mockReset().mockResolvedValue(catalog);
    switchOrganization.mockReset();
    useUIStore.setState({ lang: 'en' });
    useAuthStore.setState({
      user: {
        id: 'u',
        email: 'u@example.test',
        username: 'u',
        full_name: 'U',
        role: '',
        tenant_id: ORG_A,
      },
      switchOrganization,
    });
  });

  it('lists every organization, marks the current one and names the role in each', async () => {
    listMine.mockResolvedValue(rows);
    renderSwitcher();
    await userEvent.click(trigger());

    const menu = await screen.findByRole('menu', { name: 'Your organizations' });
    const a = await within(menu).findByRole('menuitemradio', { name: /Org A/ });
    const b = within(menu).getByRole('menuitemradio', { name: /Org B/ });
    expect(a).toHaveAttribute('aria-checked', 'true');
    expect(b).toHaveAttribute('aria-checked', 'false');
    await waitFor(() => expect(a).toHaveTextContent('Viewer'));
    expect(b).toHaveTextContent('Owner');
  });

  it('switches to another organization, then hands over to the reload', async () => {
    listMine.mockResolvedValue(rows);
    switchOrganization.mockResolvedValue(undefined);
    const { onSwitched } = renderSwitcher();
    await userEvent.click(trigger());
    await userEvent.click(await screen.findByRole('menuitemradio', { name: /Org B/ }));

    await waitFor(() => expect(onSwitched).toHaveBeenCalledTimes(1));
    expect(switchOrganization).toHaveBeenCalledWith(ORG_B, 'Org B');
  });

  it('does not switch when the current organization is picked', async () => {
    listMine.mockResolvedValue(rows);
    const { onSwitched } = renderSwitcher();
    await userEvent.click(trigger());
    await userEvent.click(await screen.findByRole('menuitemradio', { name: /Org A/ }));

    expect(switchOrganization).not.toHaveBeenCalled();
    expect(onSwitched).not.toHaveBeenCalled();
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('states a refused switch and stays put', async () => {
    listMine.mockResolvedValue(rows);
    switchOrganization.mockRejectedValue(new Error('403'));
    const { onSwitched } = renderSwitcher();
    await userEvent.click(trigger());
    await userEvent.click(await screen.findByRole('menuitemradio', { name: /Org B/ }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The switch failed. You are still in Org A.',
    );
    expect(onSwitched).not.toHaveBeenCalled();
  });

  it('shows an error with a retry when the list cannot be loaded', async () => {
    listMine.mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(rows);
    renderSwitcher();
    await userEvent.click(trigger());

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Your organizations could not be loaded.',
    );
    await userEvent.click(screen.getByRole('menuitem', { name: 'Retry' }));
    expect(await screen.findByRole('menuitemradio', { name: /Org B/ })).toBeInTheDocument();
  });

  it('tells a single-organization user how another one gets here', async () => {
    listMine.mockResolvedValue([rows[0]]);
    renderSwitcher();
    await userEvent.click(trigger());
    expect(
      await screen.findByText(/An invitation from another organization will add it here/),
    ).toBeInTheDocument();
  });

  it('keeps the way to the organization settings', async () => {
    listMine.mockResolvedValue(rows);
    renderSwitcher();
    await userEvent.click(trigger());
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Organization settings' }));
    expect(await screen.findByText('settings page')).toBeInTheDocument();
  });

  it('is keyboard-operable: arrows move, Escape closes and returns focus', async () => {
    listMine.mockResolvedValue(rows);
    renderSwitcher();
    trigger().focus();
    await userEvent.keyboard('{Enter}');

    const a = await screen.findByRole('menuitemradio', { name: /Org A/ });
    await waitFor(() => expect(a).toHaveFocus());
    await userEvent.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitemradio', { name: /Org B/ })).toHaveFocus();
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    expect(trigger()).toHaveFocus();
  });
});

describe('OrgSwitcher — badge', () => {
  it('shows the given badge instead of the initials', () => {
    useUIStore.setState({ lang: 'en' });
    const client = new QueryClient();
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter>
          <OrgSwitcher orgName="Org A" badge={<img alt="Org A logo" src="data:," />} />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByRole('img', { name: 'Org A logo' })).toBeInTheDocument();
    expect(screen.queryByText('OA')).toBeNull();
  });
});
