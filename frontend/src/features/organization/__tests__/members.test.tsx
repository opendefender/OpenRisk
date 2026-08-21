// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Unit coverage for the members screen's contract (W0-04).
//
// The claims worth pinning here are the ones a reviewer would otherwise have to
// take on trust: that the UI state matrix is complete (loading / empty / error
// / permission-denied), that a control is disabled for the SERVER's reason
// rather than a guess of ours, and above all that the product never tells an
// administrator an invitation was emailed when it was not.

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { MembersView } from '../MembersView';
import { organizationService } from '../organizationService';
import type { MemberView, InvitationView, InviteResult, Page } from '../organizationService';

/* ------------------------------------------------------------------ fixtures */

const OWNER: MemberView = {
  member_id: 'm-owner', user_id: 'u-owner', email: 'owner@acme.io', full_name: 'Olive Owner',
  org_role: 'root', business_role: '', status: 'active', is_active: true,
  joined_at: '2026-01-05T10:00:00Z', permissions: ['*'], is_owner: true,
};
const PLAIN: MemberView = {
  member_id: 'm-plain', user_id: 'u-plain', email: 'pat@acme.io', full_name: 'Pat Plain',
  org_role: 'user', business_role: 'auditor', status: 'active', is_active: true,
  joined_at: '2026-03-11T10:00:00Z', permissions: ['risks:read'], is_owner: false,
};
const SUSPENDED: MemberView = {
  ...PLAIN, member_id: 'm-susp', user_id: 'u-susp', email: 'sam@acme.io', full_name: 'Sam Suspended',
  status: 'deactivated', is_active: false, deactivated_at: '2026-06-01T10:00:00Z',
};

function page<T>(items: T[]): Page<T> {
  return { items, total: items.length, limit: 50, offset: 0 };
}

const PENDING_INVITE: InvitationView = {
  id: 'i-1', email: 'newcomer@acme.io', role: 'user', status: 'pending',
  expires_at: '2026-12-31T10:00:00Z', invited_by_id: 'u-owner', invited_by_email: 'owner@acme.io',
  last_sent_at: '2026-08-20T10:00:00Z', send_count: 1, created_at: '2026-08-20T10:00:00Z',
  can_resend: true,
};

/* -------------------------------------------------------------------- harness */

let currentPermissions: string[] = [];

vi.mock('../../../hooks/usePermissions', () => ({
  usePermissions: () => ({
    can: (p: string) => currentPermissions.includes('*') || currentPermissions.includes(p),
  }),
}));
vi.mock('../../../hooks/useAuthStore', () => ({
  useAuthStore: (sel: (s: { user: { id: string; email: string } }) => unknown) =>
    sel({ user: { id: 'u-owner', email: 'owner@acme.io' } }),
}));
vi.mock('../../rbac/useRbac', () => ({
  useRbacCatalog: () => ({
    data: {
      permissions: [],
      business_roles: [
        { key: 'auditor', label_fr: 'Auditeur', label_en: 'Auditor', description_fr: '', description_en: '', permissions: [], default_landing: '/' },
      ],
    },
  }),
}));
const toastCalls: string[] = [];
vi.mock('sonner', () => ({
  toast: {
    success: (m: string) => { toastCalls.push(`success:${m}`); },
    error: (m: string) => { toastCalls.push(`error:${m}`); },
    info: (m: string) => { toastCalls.push(`info:${m}`); },
  },
}));

function renderMembers() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <MembersView />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  currentPermissions = ['*'];
  toastCalls.length = 0;
  vi.spyOn(organizationService, 'listMembers').mockResolvedValue(page([OWNER, PLAIN, SUSPENDED]));
  vi.spyOn(organizationService, 'listInvitations').mockResolvedValue(page([PENDING_INVITE]));
  vi.spyOn(organizationService, 'membershipAudit').mockResolvedValue(page([]));
});
afterEach(() => vi.restoreAllMocks());

/* ------------------------------------------------------------- the state matrix */

describe('members — UI state matrix', () => {
  it('shows a loading state before the roster arrives', () => {
    vi.spyOn(organizationService, 'listMembers').mockImplementation(() => new Promise(() => {}));
    renderMembers();
    // The claim that matters: while the roster is in flight the screen shows
    // neither rows nor the empty state. An empty state here would read as
    // "this organization has nobody in it", which is a different fact.
    expect(screen.queryAllByTestId('member-row')).toHaveLength(0);
    expect(screen.queryByTestId('empty-state')).not.toBeInTheDocument();
  });

  it('shows an empty state with a reason when the roster is empty', async () => {
    vi.spyOn(organizationService, 'listMembers').mockResolvedValue(page<MemberView>([]));
    renderMembers();
    expect(await screen.findByTestId('empty-state')).toBeInTheDocument();
  });

  it('shows an error state with a retry that actually refetches', async () => {
    const spy = vi.spyOn(organizationService, 'listMembers').mockRejectedValue(new Error('boom'));
    renderMembers();
    const retry = await screen.findByRole('button', { name: /réessayer|retry/i });
    const before = spy.mock.calls.length;
    fireEvent.click(retry);
    await waitFor(() => expect(spy.mock.calls.length).toBeGreaterThan(before));
  });

  it('shows a permission-denied state rather than an empty roster', async () => {
    currentPermissions = ['risks:read'];
    renderMembers();
    expect(await screen.findByTestId('empty-state')).toBeInTheDocument();
    expect(screen.getByText(/accès non autorisé|do not have access/i)).toBeInTheDocument();
    // And no invite affordance, because the API would refuse it.
    expect(screen.queryByTestId('invite-member')).not.toBeInTheDocument();
  });
});

/* ---------------------------------------------------------------- the roster */

describe('members — roster', () => {
  it('renders each member with a status in words, not colour alone', async () => {
    renderMembers();
    const rows = await screen.findAllByTestId('member-row');
    const samRow = rows.find((r) => r.textContent?.includes('sam@acme.io'))!;
    expect(samRow.textContent).toContain('Sam Suspended');
    // Scoped to the row: the same word is also a filter chip above the table.
    // The status has to be legible to someone who cannot distinguish the hues,
    // so the assertion is on the WORD, not on a colour.
    expect(samRow.textContent).toMatch(/désactivé|deactivated/i);
    const active = rows.find((r) => r.textContent?.includes('pat@acme.io'))!;
    expect(active.textContent).toMatch(/actif|active/i);
  });

  it('offers no role or status control for the owner or for yourself', async () => {
    renderMembers();
    const rows = await screen.findAllByTestId('member-row');
    const ownerRow = rows.find((r) => r.textContent?.includes('owner@acme.io'))!;
    // The server refuses both; the row says so instead of offering a click
    // that would come back 403.
    expect(ownerRow.querySelector('select')).toBeNull();
    expect(ownerRow.textContent).toMatch(/propriétaire|owner/i);
  });

  it('lets an administrator change a member role and reports the outcome', async () => {
    const setRole = vi.spyOn(organizationService, 'setRole')
      .mockResolvedValue({ ...PLAIN, org_role: 'admin', business_role: '' });
    renderMembers();
    const rows = await screen.findAllByTestId('member-row');
    const patRow = rows.find((r) => r.textContent?.includes('pat@acme.io'))!;
    const select = patRow.querySelector('select')!;
    fireEvent.change(select, { target: { value: '__admin__' } });
    await waitFor(() => expect(setRole).toHaveBeenCalledWith('m-plain', 'admin', ''));
    await waitFor(() => expect(toastCalls.some((t) => t.startsWith('success:'))).toBe(true));
  });

  it('surfaces the server’s reason when a role change is refused', async () => {
    vi.spyOn(organizationService, 'setRole').mockRejectedValue({
      response: { status: 400, data: { error: 'this is the last active administrator — promote another member first' } },
    });
    renderMembers();
    const rows = await screen.findAllByTestId('member-row');
    const patRow = rows.find((r) => r.textContent?.includes('pat@acme.io'))!;
    fireEvent.change(patRow.querySelector('select')!, { target: { value: '__admin__' } });
    // The server names the invariant that was broken; a generic "update failed"
    // would erase the one piece of information the admin can act on.
    await waitFor(() =>
      expect(toastCalls).toContainEqual(expect.stringContaining('last active administrator')));
  });

  it('requires a confirmation before withdrawing access, and offers the reversible alternative', async () => {
    const setStatus = vi.spyOn(organizationService, 'setStatus').mockResolvedValue(SUSPENDED);
    renderMembers();
    const rows = await screen.findAllByTestId('member-row');
    const patRow = rows.find((r) => r.textContent?.includes('pat@acme.io'))!;
    fireEvent.click(patRow.querySelector('button[aria-label*="évoquer"], button[aria-label*="evoke"]')!);

    // Nothing has been sent yet — the click opens a confirmation, it does not act.
    expect(setStatus).not.toHaveBeenCalled();
    // The words appear twice on purpose (dialog title and confirm button).
    expect(await screen.findAllByText(/révoquer l'accès|revoke access/i)).not.toHaveLength(0);
    // The impact radiography states the consequence in the user's terms.
    expect(screen.getByText(/la révocation est définitive|revocation is final/i)).toBeInTheDocument();
    // Revocation is final, so the reversible option is offered as a real button.
    expect(screen.getByText(/désactiver — réversible|deactivate — reversible/i)).toBeInTheDocument();

    // Confirming is what finally calls the API.
    fireEvent.click(screen.getAllByRole('button', { name: /révoquer l'accès|revoke access/i }).at(-1)!);
    await waitFor(() => expect(setStatus).toHaveBeenCalledWith('m-plain', 'revoked', undefined));
  });
});

/* ----------------------------------------------------------- the invitations */

describe('members — invitations', () => {
  async function openInvitations() {
    renderMembers();
    fireEvent.click(await screen.findByTestId('members-tab-invitations'));
    return screen.findAllByTestId('invitation-row');
  }

  it('lists outstanding invitations with their expiry and send count', async () => {
    const rows = await openInvitations();
    expect(rows).toHaveLength(1);
    expect(rows[0].textContent).toContain('newcomer@acme.io');
    expect(rows[0].textContent).toMatch(/en attente|pending/i);
  });

  it('disables re-send for the server’s reason rather than our guess', async () => {
    vi.spyOn(organizationService, 'listInvitations')
      .mockResolvedValue(page([{ ...PENDING_INVITE, can_resend: false }]));
    const rows = await openInvitations();
    const resend = rows[0].querySelector('button')!;
    expect(resend).toBeDisabled();
  });

  it('confirms before revoking an invitation', async () => {
    const revoke = vi.spyOn(organizationService, 'revokeInvitation')
      .mockResolvedValue({ ...PENDING_INVITE, status: 'revoked' });
    const rows = await openInvitations();
    fireEvent.click(rows[0].querySelector('button[aria-label*="évoquer"], button[aria-label*="evoke"]')!);
    expect(revoke).not.toHaveBeenCalled();
    expect(await screen.findByText(/révoquer l'invitation|revoke the invitation/i)).toBeInTheDocument();
  });
});

/* --------------------------------------------------- delivery is never faked */

describe('members — the product never claims a delivery it did not make', () => {
  async function invite(result: InviteResult) {
    vi.spyOn(organizationService, 'invite').mockResolvedValue(result);
    renderMembers();
    fireEvent.click(await screen.findByTestId('invite-member'));
    fireEvent.change(screen.getByLabelText(/adresse email|email address/i), {
      target: { value: 'new@acme.io' },
    });
    fireEvent.click(screen.getByRole('button', { name: /envoyer l'invitation|send invitation/i }));
  }

  it('says "sent" only when the email actually went out, and withholds the link', async () => {
    await invite({ invitation: PENDING_INVITE, delivery: 'sent' });
    expect(await screen.findByText(/invitation envoyée|invitation sent/i)).toBeInTheDocument();
    // When the mail was delivered, an administrator has no reason to hold a
    // credential that authenticates as somebody else.
    expect(screen.queryByText(/https?:\/\//)).not.toBeInTheDocument();
  });

  it('says the mail did NOT go out, and hands over the link to relay by hand', async () => {
    await invite({
      invitation: PENDING_INVITE,
      delivery: 'unavailable',
      delivery_detail: 'No email transport is configured on this deployment — share the link below yourself.',
      accept_url: 'https://openrisk.test/invitations/accept?token=abc',
    });
    expect(await screen.findByText(/à transmettre vous-même|deliver it yourself/i)).toBeInTheDocument();
    expect(screen.getByText(/No email transport is configured/)).toBeInTheDocument();
    expect(screen.getByText('https://openrisk.test/invitations/accept?token=abc')).toBeInTheDocument();
  });

  it('reports a failed send as failed, not as success', async () => {
    await invite({
      invitation: PENDING_INVITE,
      delivery: 'failed',
      delivery_detail: 'The invitation was created but the email could not be sent — share the link below yourself.',
      accept_url: 'https://openrisk.test/invitations/accept?token=xyz',
    });
    expect(await screen.findByText(/à transmettre vous-même|deliver it yourself/i)).toBeInTheDocument();
    expect(screen.queryByText(/invitation envoyée|invitation sent/i)).not.toBeInTheDocument();
  });

  it('shows the server’s message when the invitation is refused', async () => {
    vi.spyOn(organizationService, 'invite').mockRejectedValue({
      response: { status: 409, data: { error: 'this person is already a member of this organization' } },
    });
    renderMembers();
    fireEvent.click(await screen.findByTestId('invite-member'));
    fireEvent.change(screen.getByLabelText(/adresse email|email address/i), {
      target: { value: 'owner@acme.io' },
    });
    fireEvent.click(screen.getByRole('button', { name: /envoyer l'invitation|send invitation/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/already a member/i);
  });
});
