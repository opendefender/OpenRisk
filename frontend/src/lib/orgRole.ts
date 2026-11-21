// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

/** The two fields of the signed-in user that carry the role in an organization. */
export interface OrgScopedUser {
  tenant_id?: string;
  org_roles?: Record<string, string>; // { orgId: roleName } from the JWT
}

/**
 * The caller's role in the ACTIVE organization: root | admin | user, or "".
 *
 * This is the one place the frontend reads it (#702). It comes from the signed
 * token's org_roles, keyed by the active tenant. user.role is the legacy
 * relation from /auth/me and is empty on every account created by sign-up or
 * invitation, so a screen that read it showed owners a member's interface.
 */
export function activeOrgRole(user: OrgScopedUser | null | undefined): string {
  if (!user?.tenant_id) return '';
  return (user.org_roles?.[user.tenant_id] ?? '').toLowerCase();
}

/**
 * Whether the caller administers the active organization: admin, or root
 * (the owner), who outranks every role gate as middleware.RequireRole does.
 * It matches the server's own admin test for shared resources, such as
 * savedViewCaller in saved_view_handler.go.
 */
export function isOrgAdmin(user: OrgScopedUser | null | undefined): boolean {
  const role = activeOrgRole(user);
  return role === 'admin' || role === 'root';
}
