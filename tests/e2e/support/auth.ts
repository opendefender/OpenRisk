// API-based authentication for E2E — NO test logs in through the UI except
// auth.login.spec.ts.
//
// This used to mint a storageState of localStorage tokens (auth_token,
// auth_refresh_token, auth_expires_in) because that is what the app once used.
// It no longer does: the durable credential is an HttpOnly cookie the backend
// sets, `lib/api.ts` sends `withCredentials: true`, and the in-memory token is
// only a fallback for a split-origin deployment. A storageState without those
// cookies produces a browser that BELIEVES it is signed in — `auth_user` is
// present, so the route guard lets it through — while every API call 401s. Every
// authenticated spec would then be asserting against a logged-out app.
//
// So the session is established through a Playwright APIRequestContext, which
// captures Set-Cookie, and the storageState carries those cookies plus the
// `auth_user` profile the SPA paints from before its first request returns.
//
// The second change is MFA. It is mandated for every account (W0-03), so a
// password no longer produces a session: /auth/login answers 200 with an
// mfa_token and nothing else, and the old helper threw on the missing
// token_pair. Both second-factor shapes are completed here.

import crypto from 'node:crypto';
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

/** What /auth/login answers when a second factor stands in the way. */
interface MfaChallenge {
  mfa_token?: string;
  mfa_required?: boolean;
  mfa_enrollment_required?: boolean;
}

/**
 * RFC 6238 TOTP from Node's crypto — six digits do not warrant a dependency.
 * Mirrors scripts/seed-e2e.mjs, which enrols the seed admin.
 */
export function totp(base32Secret: string, atSeconds = Date.now() / 1000): string {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const ch of base32Secret.replace(/=+$/, '').toUpperCase()) {
    const idx = alphabet.indexOf(ch);
    if (idx >= 0) bits += idx.toString(2).padStart(5, '0');
  }
  const bytes = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(atSeconds / 30)));
  const hmac = crypto.createHmac('sha1', bytes).update(counter).digest();
  const offset = hmac[hmac.length - 1] & 0x0f;
  return String((hmac.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).padStart(6, '0');
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

/**
 * Signs in through a Playwright APIRequestContext, completing MFA if demanded.
 *
 * The context accumulates the session cookies, so pass the SAME context to
 * `storageStateFor` afterwards.
 *
 * @param mfaSecret the account's TOTP secret; the seed records the admin's in
 *   tests/e2e/.seed-ids.json. Without it an enrolled account cannot be answered.
 */
export async function apiLogin(
  request: APIRequestContext,
  email: string,
  password: string,
  mfaSecret?: string,
): Promise<LoginResult> {
  const res = await request.post(`${API_URL}/auth/login`, { data: { email, password } });
  if (!res.ok()) {
    throw new Error(`login failed for ${email}: ${res.status()} ${await res.text()}`);
  }
  const body = (await res.json()) as LoginResult & MfaChallenge;
  if (body.token_pair?.access_token) return body;

  // 200 without a token_pair means a second factor is in the way.
  const mfaToken = body.mfa_token;
  if (!mfaToken) {
    throw new Error(`login for ${email} produced neither a session nor an mfa_token`);
  }
  const auth = { Authorization: `Bearer ${mfaToken}` };

  if (body.mfa_enrollment_required) {
    const setup = await request.post(`${API_URL}/auth/mfa/setup`, { headers: auth, data: {} });
    if (!setup.ok()) throw new Error(`mfa setup failed for ${email}: ${setup.status()} ${await setup.text()}`);
    const secret = ((await setup.json()) as { secret?: string }).secret;
    if (!secret) throw new Error(`mfa setup for ${email} returned no secret`);
    const verify = await request.post(`${API_URL}/auth/mfa/verify`, {
      headers: auth,
      data: { code: totp(secret) },
    });
    if (!verify.ok()) throw new Error(`mfa verify failed for ${email}: ${verify.status()} ${await verify.text()}`);
    return await withProfile(request, (await verify.json()) as LoginResult);
  }

  if (!mfaSecret) {
    // Say which knob fixes it. A bare "invalid code" sends the next person
    // hunting the wrong thing.
    throw new Error(
      `${email} has MFA enrolled but no TOTP secret was supplied. ` +
        'Pass the secret recorded in tests/e2e/.seed-ids.json (adminMfaSecret), or reset the account\'s MFA.',
    );
  }
  const challenge = await request.post(`${API_URL}/auth/mfa/challenge`, {
    headers: auth,
    data: { code: totp(mfaSecret) },
  });
  if (!challenge.ok()) {
    throw new Error(`mfa challenge failed for ${email}: ${challenge.status()} ${await challenge.text()}`);
  }
  return await withProfile(request, (await challenge.json()) as LoginResult);
}

/**
 * The MFA endpoints answer with a token pair but no profile — exactly like the
 * frontend's adoptSession, which reads /auth/me next. Do the same, so callers
 * always get a LoginResult with a usable `user`.
 */
async function withProfile(request: APIRequestContext, session: LoginResult): Promise<LoginResult> {
  if (session.user) return session;
  const me = await request.get(`${API_URL}/auth/me`, {
    headers: { Authorization: `Bearer ${session.token_pair.access_token}` },
  });
  if (!me.ok()) throw new Error(`/auth/me failed: ${me.status()} ${await me.text()}`);
  const body = (await me.json()) as { user?: Record<string, unknown>; organization_id?: string };
  return { ...session, user: (body.user ?? body) as LoginResult['user'] };
}

/**
 * A storageState carrying the REAL session cookies plus the cached profile.
 *
 * `request.storageState()` holds the cookies the login accumulated — or_access
 * and or_refresh (HttpOnly) and or_csrf (readable, for the double-submit check).
 * Those are the credential. `auth_user` is only what the SPA paints from before
 * its first request returns, and what its route guard reads to decide it has a
 * session at all.
 */
export async function storageStateFor(request: APIRequestContext, login: LoginResult) {
  const state = await request.storageState();
  return {
    cookies: state.cookies,
    origins: [
      {
        origin: FRONTEND_ORIGIN,
        localStorage: [{ name: 'auth_user', value: JSON.stringify(buildAuthUser(login)) }],
      },
    ],
  };
}
