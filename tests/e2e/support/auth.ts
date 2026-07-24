// API-based authentication for E2E — NO test logs in through the UI except
// auth.login.spec.ts. We POST /auth/login, then mint a Playwright `storageState`
// that reproduces exactly what useAuthStore.login writes to localStorage
// (frontend/src/hooks/useAuthStore.ts): auth_token / auth_refresh_token /
// auth_user (role flattened to role.name + JWT-derived permissions) / auth_expires_in.

import type { APIRequestContext } from '@playwright/test';
import { API_URL, FRONTEND_ORIGIN } from './env';

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}
export interface LoginResult {
  user: Record<string, unknown> & { role?: unknown };
  token_pair: TokenPair;
  organization?: Record<string, unknown>;
  business_role?: string;
}

/** Decode a JWT payload (base64url) without verifying — test-only. */
export function decodeJwt(token: string): Record<string, unknown> {
  const part = token.split('.')[1] ?? '';
  try {
    return JSON.parse(Buffer.from(part, 'base64url').toString('utf8'));
  } catch {
    return {};
  }
}

/** Reproduce useAuthStore.withTokenClaims: flatten role, fold in JWT claims. */
export function buildAuthUser(login: LoginResult): Record<string, unknown> {
  const claims = decodeJwt(login.token_pair.access_token);
  const roleName =
    login.user.role && typeof login.user.role === 'object'
      ? (login.user.role as { name?: string }).name ?? ''
      : (login.user.role as string) ?? '';
  return {
    ...login.user,
    role: roleName,
    permissions: claims.permissions ?? (login.user as { permissions?: unknown }).permissions ?? [],
    org_roles: claims.org_roles ?? (login.user as { org_roles?: unknown }).org_roles,
    tenant_id: claims.tenant_id ?? (login.user as { tenant_id?: unknown }).tenant_id,
    business_role: login.business_role ?? (login.user as { business_role?: string }).business_role ?? '',
  };
}

/** A Playwright storageState that authenticates the app via localStorage. */
export function buildStorageState(login: LoginResult) {
  const user = buildAuthUser(login);
  return {
    cookies: [],
    origins: [
      {
        origin: FRONTEND_ORIGIN,
        localStorage: [
          { name: 'auth_token', value: login.token_pair.access_token },
          { name: 'auth_refresh_token', value: login.token_pair.refresh_token },
          { name: 'auth_user', value: JSON.stringify(user) },
          { name: 'auth_expires_in', value: String(login.token_pair.expires_in) },
        ],
      },
    ],
  };
}

/** POST /auth/login through a Playwright APIRequestContext (no CORS in Node). */
export async function apiLogin(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<LoginResult> {
  const res = await request.post(`${API_URL}/auth/login`, { data: { email, password } });
  if (!res.ok()) {
    throw new Error(`login failed for ${email}: ${res.status()} ${await res.text()}`);
  }
  return (await res.json()) as LoginResult;
}
