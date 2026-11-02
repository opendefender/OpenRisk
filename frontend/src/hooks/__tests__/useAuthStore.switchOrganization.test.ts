// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #296 — switchOrganization moves the session to another organization the user
// belongs to. A switch is a tenant boundary: the previous organization's
// permissions, persona and caches must not survive it, and a refused switch
// must leave the current session untouched.

import { vi, describe, it, expect, beforeEach } from 'vitest';

vi.mock('../../lib/sessionScope', () => ({ clearSessionScope: vi.fn() }));
vi.mock('../../lib/session', () => ({ setAccessToken: vi.fn() }));

import { useAuthStore } from '../useAuthStore';
import { api } from '../../lib/api';
import { clearSessionScope } from '../../lib/sessionScope';
import { setAccessToken } from '../../lib/session';

function tokenWith(payload: Record<string, unknown>): string {
  const b64 = (o: unknown) =>
    btoa(JSON.stringify(o)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `${b64({ alg: 'RS256', typ: 'JWT' })}.${b64(payload)}.sig`;
}

const ORG_A = 'org-a';
const ORG_B = 'org-b';

type StoreUser = NonNullable<ReturnType<typeof useAuthStore.getState>['user']>;

// Signed into org A as a viewer: read-only, with the viewer persona.
const viewerInA: StoreUser = {
  id: 'user-1',
  email: 'u@example.test',
  username: 'u',
  full_name: 'U',
  role: '',
  tenant_id: ORG_A,
  org_name: 'Org A',
  business_role: 'viewer',
  permissions: ['risks:read'],
  org_roles: { [ORG_A]: 'user' },
};

beforeEach(() => {
  vi.restoreAllMocks();
  vi.mocked(clearSessionScope).mockClear();
  vi.mocked(setAccessToken).mockClear();
  localStorage.clear();
  useAuthStore.setState({ user: viewerInA, token: 'old', isAuthenticated: true });
});

describe('switchOrganization', () => {
  it('TestSwitchOrganization_Success — adopts the new organization and wipes the old one', async () => {
    const token = tokenWith({
      sub: 'user-1',
      tenant_id: ORG_B,
      permissions: ['*'],
      org_roles: { [ORG_B]: 'root' },
    });
    const post = vi.spyOn(api, 'post').mockResolvedValueOnce({
      data: {
        token_pair: { access_token: token, refresh_token: 'r', expires_in: 900 },
        organization: { id: ORG_B, name: 'Org B' },
        role: 'root',
        // Owner of org B: no business role. The field is omitted, as the API does.
      },
    });

    const outcome = await useAuthStore.getState().switchOrganization(ORG_B, 'Org B');

    expect(outcome).toEqual({ status: 'switched' });
    expect(post).toHaveBeenCalledWith('/auth/switch-org', { organization_id: ORG_B });
    expect(clearSessionScope).toHaveBeenCalledTimes(1);
    expect(setAccessToken).toHaveBeenCalledWith(token);
    const user = useAuthStore.getState().user;
    expect(user?.tenant_id).toBe(ORG_B);
    expect(user?.org_name).toBe('Org B');
    expect(user?.permissions).toEqual(['*']);
    // The viewer persona belonged to org A; it must not follow the user out.
    expect(user?.business_role).toBe('');
    expect(JSON.parse(localStorage.getItem('auth_user') ?? '{}').tenant_id).toBe(ORG_B);
  });

  it('names the organization from the switcher row when the response omits it', async () => {
    const token = tokenWith({ sub: 'user-1', tenant_id: ORG_B, permissions: ['*'] });
    vi.spyOn(api, 'post').mockResolvedValueOnce({
      data: {
        token_pair: { access_token: token, refresh_token: 'r', expires_in: 900 },
        organization: null,
        role: 'root',
      },
    });

    await useAuthStore.getState().switchOrganization(ORG_B, 'Org B');

    expect(useAuthStore.getState().user?.org_name).toBe('Org B');
  });

  // GHSA-45mr-j2w6-m4jv — the target organization keeps its own MFA posture.
  it('TestSwitchOrganization_Unauthorized — a second factor owed in the target changes nothing yet', async () => {
    vi.spyOn(api, 'post').mockResolvedValueOnce({
      data: {
        mfa_required: true,
        mfa_token: 'mfa-for-b',
        organization: { id: ORG_B, name: 'Org B' },
      },
    });

    const outcome = await useAuthStore.getState().switchOrganization(ORG_B, 'Org B');

    expect(outcome).toEqual({ status: 'mfa_required', mfa_token: 'mfa-for-b' });
    expect(clearSessionScope).not.toHaveBeenCalled();
    expect(setAccessToken).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user).toEqual(viewerInA);
    expect(useAuthStore.getState().token).toBe('old');
  });

  it('reports an enrolment owed in the target as one', async () => {
    vi.spyOn(api, 'post').mockResolvedValueOnce({
      data: { mfa_enrollment_required: true, mfa_token: 'enrol-in-b', organization: null },
    });

    const outcome = await useAuthStore.getState().switchOrganization(ORG_B, 'Org B');

    expect(outcome).toEqual({ status: 'mfa_enrollment_required', mfa_token: 'enrol-in-b' });
    expect(setAccessToken).not.toHaveBeenCalled();
  });

  it('TestSwitchOrganization_Unauthorized — a refused switch changes nothing', async () => {
    vi.spyOn(api, 'post').mockRejectedValueOnce(
      Object.assign(new Error('Request failed with status code 403'), {
        response: { status: 403 },
      }),
    );

    await expect(useAuthStore.getState().switchOrganization(ORG_B, 'Org B')).rejects.toThrow();

    expect(clearSessionScope).not.toHaveBeenCalled();
    expect(setAccessToken).not.toHaveBeenCalled();
    expect(useAuthStore.getState().user).toEqual(viewerInA);
    expect(useAuthStore.getState().token).toBe('old');
  });
});
