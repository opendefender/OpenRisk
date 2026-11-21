// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, afterEach } from 'vitest';

import { useAuthStore } from '../useAuthStore';
import { activeOrgRole, isOrgAdmin } from '../../lib/orgRole';

// #867 — hasRole read only user.role, which is "" on every account created by
// registration or invitation; the role in the organization lives in the
// token's org_roles. hasRole('admin') was therefore false for everyone and the
// risk-weights page was read-only for owners and admins alike, while the
// server (middleware.RequireRole) accepted their saves.
//
// These users have the shape login actually stores (role "", org_roles from
// the JWT), not a stub of hasRole: a stub is what hid the defect.

type StoredUser = NonNullable<ReturnType<typeof useAuthStore.getState>['user']>;

const base: StoredUser = {
  id: 'u1',
  email: 'a@example.test',
  username: 'a',
  full_name: 'A',
  role: '',
  business_role: '',
  tenant_id: 'org-1',
  org_roles: {},
};

function as(user: Partial<StoredUser>) {
  useAuthStore.setState({ user: { ...base, ...user } });
  return useAuthStore.getState().hasRole;
}

afterEach(() => useAuthStore.setState({ user: null }));

describe('hasRole', () => {
  it('is true for an admin of the active organization', () => {
    expect(as({ org_roles: { 'org-1': 'admin' } })('admin')).toBe(true);
  });

  it('is true for the owner (root), who outranks every role gate as on the server', () => {
    expect(as({ org_roles: { 'org-1': 'root' } })('admin')).toBe(true);
  });

  it('is false for a plain member', () => {
    expect(as({ org_roles: { 'org-1': 'user' } })('admin')).toBe(false);
  });

  it('ignores a role held in another organization', () => {
    expect(as({ org_roles: { 'org-2': 'admin', 'org-1': 'user' } })('admin')).toBe(false);
    expect(as({ org_roles: { 'org-2': 'root' } })('admin')).toBe(false);
  });

  it('still matches a business role preset', () => {
    expect(as({ org_roles: { 'org-1': 'user' }, business_role: 'rssi' })('rssi')).toBe(true);
  });

  it('is false when signed out', () => {
    useAuthStore.setState({ user: null });
    expect(useAuthStore.getState().hasRole('admin')).toBe(false);
  });
});

// #702 — the org role is read in one place; hasRole and the saved-views gate
// both go through it.
describe('activeOrgRole / isOrgAdmin', () => {
  it('reads the role of the active organization only', () => {
    expect(activeOrgRole({ tenant_id: 'org-1', org_roles: { 'org-1': 'Admin' } })).toBe('admin');
    expect(activeOrgRole({ tenant_id: 'org-1', org_roles: { 'org-2': 'root' } })).toBe('');
    expect(activeOrgRole({ org_roles: { 'org-1': 'root' } })).toBe('');
    expect(activeOrgRole(null)).toBe('');
  });

  it('holds for root and admin, not for a member', () => {
    expect(isOrgAdmin({ ...base, org_roles: { 'org-1': 'root' } })).toBe(true);
    expect(isOrgAdmin({ ...base, org_roles: { 'org-1': 'admin' } })).toBe(true);
    expect(isOrgAdmin({ ...base, org_roles: { 'org-1': 'user' } })).toBe(false);
  });

  it('ignores the legacy user.role, which the server does not read either', () => {
    expect(isOrgAdmin({ ...base, role: 'admin', org_roles: { 'org-1': 'user' } })).toBe(false);
  });
});
