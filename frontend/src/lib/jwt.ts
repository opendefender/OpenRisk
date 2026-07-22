// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Minimal, dependency-free decoder for the RS256 access token. We do NOT verify
// the signature client-side (the backend is the trust boundary — every request
// is re-validated there); we only read the claims to know what the current user
// can do, so the UI can gate menus/actions the same way the API gates routes.

export interface AccessTokenClaims {
  sub?: string;
  tenant_id?: string;
  org_roles?: Record<string, string>; // { orgId: roleName }
  permissions?: string[]; // canonical "resource:action" strings, or ["*"]
  feature_flags?: string[];
  type?: string;
  exp?: number;
}

/** base64url → JSON, tolerant of padding. Returns {} on any failure. */
export function decodeAccessToken(token: string | null | undefined): AccessTokenClaims {
  if (!token) return {};
  const parts = token.split('.');
  if (parts.length !== 3) return {};
  try {
    const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4);
    const json = decodeURIComponent(
      atob(padded)
        .split('')
        .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    );
    return JSON.parse(json) as AccessTokenClaims;
  } catch {
    return {};
  }
}

/**
 * Mirrors the backend middleware.hasPermission wildcard semantics exactly:
 *   - "*" matches everything (root/admin);
 *   - an exact match;
 *   - a trailing ":*" wildcard matches by prefix, so "compliance:*" grants
 *     "compliance:frameworks:read" and "risks:*" grants "risks:create".
 */
export function permitted(permissions: string[] | undefined, required: string): boolean {
  if (!permissions || permissions.length === 0) return false;
  for (const perm of permissions) {
    if (perm === '*' || perm === required) return true;
    if (perm.endsWith(':*')) {
      const prefix = perm.slice(0, -1); // keep the trailing ":"
      if (required.length > prefix.length && required.startsWith(prefix)) return true;
    }
  }
  return false;
}
