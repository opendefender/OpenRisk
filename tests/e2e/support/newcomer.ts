// Registering a brand-new account, for the specs that must exercise a tenant
// which has NOT been onboarded.
//
// The seeded personas (admin / analyst / auditor) are all past the wizard, and
// OnboardingCompletedRedirect sends a completed user away from /onboarding/*.
// A spec that reused one of them would silently be redirected to the dashboard
// and would assert against the wrong screen — green, and meaningless. Anything
// touching onboarding therefore signs up its own tenant.

import { expect, type APIRequestContext } from '@playwright/test';
import { API_URL } from './env';
import { apiLogin, storageStateFor } from './auth';

export interface Newcomer {
  email: string;
  password: string;
  storageState: Awaited<ReturnType<typeof storageStateFor>>;
  token: string;
}

/**
 * Register a brand-new account through the real API and mint its storageState.
 * Unique per call so parallel runs and retries never collide.
 */
export async function signUp(ctx: APIRequestContext, tag: string): Promise<Newcomer> {
  const stamp = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  const email = `e2e.activation.${tag}.${stamp}@openrisk.test`;
  const password = 'ActivationE2E!2026';
  const username = `e2eact${tag}${stamp}`.slice(0, 28);

  const res = await ctx.post(`${API_URL}/auth/register`, {
    data: {
      email,
      username,
      password,
      full_name: 'Awa Newcomer',
      company_name: `Awa Test Org ${stamp}`,
    },
  });
  expect(res.status(), `registration should succeed: ${await res.text()}`).toBe(201);

  const login = await apiLogin(ctx, email, password);
  return {
    email,
    password,
    // From the SAME context that logged in, so the session cookies come with
    // it — they, not a localStorage token, are the credential now.
    storageState: await storageStateFor(ctx, login),
    token: login.token_pair.access_token,
  };
}

/** Authenticated API helper for a newcomer's token. */
export function authed(ctx: APIRequestContext, token: string) {
  const headers = { Authorization: `Bearer ${token}` };
  return {
    get: (path: string) => ctx.get(`${API_URL}${path}`, { headers }),
    put: (path: string, data: unknown) => ctx.put(`${API_URL}${path}`, { headers, data }),
    post: (path: string, data: unknown) => ctx.post(`${API_URL}${path}`, { headers, data }),
  };
}
