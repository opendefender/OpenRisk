// OR26-03 — deferrable MFA enrolment, end to end.
//
// This spec holds the claims that unit tests cannot:
//
//   1. A BRAND-NEW ACCOUNT REACHES THE PRODUCT. The reported bug was that the
//      very first login answered `mfa_enrollment_required: true` and handed back
//      no session, so an evaluator met a QR code before a single screen. The
//      test registers a REAL account and asserts it gets a session, reaches the
//      dashboard, walks onboarding, and creates a risk — all without MFA.
//
//   2. THE BANNER IS THERE INSTEAD OF THE WALL. Non-blocking means the product
//      renders AND the prompt is visible; either alone would be a regression.
//
//   3. THE SERVER, NOT THE CLIENT, ENFORCES. Every bypass here is attempted at
//      the API, with no browser involved: forging the mfa block, calling a
//      protected route directly, and reading another tenant's policy. A test
//      that only hid a page in the frontend would prove nothing.
//
// It signs up a fresh tenant rather than reusing a seeded persona: the seeded
// admin already has MFA enrolled, which is precisely the state this is not about.

import { test, expect, request as pwRequest, type APIRequestContext } from '@playwright/test';
import { API_URL } from './support/env';
import { apiLogin, storageStateFor, decodeJwt } from './support/auth';

interface LoginBody {
  token_pair?: { access_token: string };
  mfa_enrollment_required?: boolean;
  mfa_required?: boolean;
  mfa?: { state?: string; required?: boolean; privileged?: boolean; grace_days?: number; deadline?: string };
}

interface Account {
  email: string;
  password: string;
  token: string;
  login: Awaited<ReturnType<typeof apiLogin>>;
}

/** Registers a brand-new tenant. The registering user becomes its root/admin. */
async function signUp(ctx: APIRequestContext, tag: string): Promise<{ account: Account; body: LoginBody }> {
  const stamp = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  const email = `e2e.mfa.${tag}.${stamp}@openrisk.test`;
  const password = 'DeferredMfaE2E!2026';

  const res = await ctx.post(`${API_URL}/auth/register`, {
    data: {
      email,
      username: `e2emfa${tag}${stamp}`.slice(0, 28),
      password,
      full_name: 'Awa Evaluator',
      company_name: `Awa MFA Org ${stamp}`,
    },
  });
  expect(res.status(), `registration should succeed: ${await res.text()}`).toBe(201);

  // The RAW login response, before any helper completes a factor for us — this
  // is the exact payload the reported bug was about.
  const raw = await ctx.post(`${API_URL}/auth/login`, { data: { email, password } });
  expect(raw.ok()).toBeTruthy();
  const body = (await raw.json()) as LoginBody;

  const login = await apiLogin(ctx, email, password);
  return { account: { email, password, token: login.token_pair.access_token, login }, body };
}

function authed(ctx: APIRequestContext, token: string) {
  const headers = { Authorization: `Bearer ${token}` };
  return {
    get: (path: string) => ctx.get(`${API_URL}${path}`, { headers }),
    put: (path: string, data: unknown) => ctx.put(`${API_URL}${path}`, { headers, data }),
    post: (path: string, data: unknown) => ctx.post(`${API_URL}${path}`, { headers, data }),
  };
}

// ---------------------------------------------------------------------------

test.describe('OR26-03 — MFA is deferrable before the Aha moment', () => {
  // These tests own their identity; the shared persona storageState must not leak in.
  test.use({ storageState: { cookies: [], origins: [] } });

  test('a brand-new account signs in without MFA and is told so honestly', async () => {
    const ctx = await pwRequest.newContext();
    const { body } = await signUp(ctx, 'first');

    // THE REGRESSION FENCE. Before OR26-03 this was `true` with no token pair.
    expect(body.mfa_enrollment_required ?? false).toBe(false);
    expect(body.mfa_required ?? false).toBe(false);
    expect(body.token_pair?.access_token, 'first login must produce a real session').toBeTruthy();

    // The session contract says what state the account is in and by when.
    expect(body.mfa, 'login must report the resolved MFA state').toBeTruthy();
    expect(body.mfa?.required).toBe(false);
    expect(['recommended', 'grace_active', 'grace_expiring']).toContain(body.mfa?.state);
    expect(body.mfa?.grace_days).toBe(7);

    // And it carries no secret material of any kind.
    const serialised = JSON.stringify(body.mfa);
    expect(serialised).not.toMatch(/secret|qr_code|backup/i);

    await ctx.dispose();
  });

  test('the registering admin is privileged, and gets a deadline rather than a wall', async () => {
    const ctx = await pwRequest.newContext();
    const { account, body } = await signUp(ctx, 'admin');

    // Whoever registers a tenant owns it, so this is the "Admin" half of the
    // issue's "Admin/RSSI": privileged, deferred, not blocked.
    expect(body.mfa?.privileged).toBe(true);
    expect(body.mfa?.state).toBe('grace_active');
    expect(body.mfa?.deadline, 'a privileged account must be told when the window closes').toBeTruthy();

    const deadline = new Date(body.mfa!.deadline!).getTime();
    const days = (deadline - Date.now()) / 86_400_000;
    expect(days).toBeGreaterThan(6);
    expect(days).toBeLessThanOrEqual(7.01);

    // /auth/me reports the same resolved state as login — one decision, two readers.
    const me = await authed(ctx, account.token).get('/auth/me');
    expect(me.ok()).toBeTruthy();
    const meBody = (await me.json()) as { mfa?: { state?: string; required?: boolean } };
    expect(meBody.mfa?.state).toBe('grace_active');
    expect(meBody.mfa?.required).toBe(false);

    await ctx.dispose();
  });

  test('onboarding and the first risk are reachable with no authenticator', async () => {
    const ctx = await pwRequest.newContext();
    const { account } = await signUp(ctx, 'journey');
    const api = authed(ctx, account.token);

    // Onboarding — the wizard the guard would otherwise hold them in front of.
    const onboarding = await api.get('/onboarding/state');
    expect(onboarding.status(), 'onboarding must be reachable without MFA').toBe(200);

    // The dashboard's data, and the activation checklist behind it.
    expect((await api.get('/activation/state')).status()).toBe(200);
    expect((await api.get('/stats')).status()).toBeLessThan(400);

    // THE AHA PATH: create the first risk. This is the value the wall used to
    // stand in front of.
    const created = await api.post('/risks', {
      name: 'Exposed admin panel on web-prod-01',
      description: 'Deferred-MFA e2e: the first risk a new tenant creates.',
      probability: 0.6,
      impact: 7,
      category: 'Cybersécurité',
    });
    expect(created.status(), `first risk must be creatable without MFA: ${await created.text()}`).toBe(201);

    // And the activation event actually landed, so the Aha path is intact.
    const state = (await (await api.get('/activation/state')).json()) as {
      steps: { key: string; completed: boolean }[];
    };
    expect(state.steps.find((s) => s.key === 'first_risk')?.completed).toBe(true);

    await ctx.dispose();
  });

  test('the dashboard renders the product, with the prompt beside it — not instead of it', async ({ browser }) => {
    const ctx = await pwRequest.newContext();
    const { account } = await signUp(ctx, 'ui');
    const storageState = await storageStateFor(ctx, account.login);

    const page = await (await browser.newContext({ storageState })).newPage();
    await page.goto('/dashboard');

    // Non-blocking means BOTH: the product is there…
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible({ timeout: 15_000 });
    expect(page.url()).not.toContain('/login');

    // …and the prompt is there too, saying which state the server resolved.
    const banner = page.getByTestId('mfa-enrollment-banner');
    await expect(banner).toBeVisible({ timeout: 15_000 });
    await expect(banner).toHaveAttribute('data-mfa-state', /grace_active|grace_expiring|recommended/);
    await expect(banner.getByRole('button', { name: /enable mfa|activer le mfa/i })).toBeVisible();

    await ctx.dispose();
  });

  test('the MFA policy is readable, admin-writable, bounded and tenant-scoped', async () => {
    const ctx = await pwRequest.newContext();
    const { account: a } = await signUp(ctx, 'polA');
    const apiA = authed(ctx, a.token);

    const initial = await apiA.get('/security/mfa-policy');
    expect(initial.status()).toBe(200);
    const policy = (await initial.json()) as {
      grace_days: number; configured: boolean; min_days: number; max_days: number; default_days: number;
      privileged_org_roles: string[]; privileged_business_roles: string[];
    };
    expect(policy.default_days).toBe(7);
    expect(policy.grace_days).toBe(7);
    expect(policy.configured).toBe(false);
    expect(policy.privileged_org_roles).toEqual(expect.arrayContaining(['admin', 'root']));
    expect(policy.privileged_business_roles).toContain('rssi');

    // Bounds are enforced by the server, not only by the form.
    expect((await apiA.put('/security/mfa-policy', { grace_days: 91 })).status()).toBe(400);
    expect((await apiA.put('/security/mfa-policy', { grace_days: -1 })).status()).toBe(400);
    expect((await apiA.put('/security/mfa-policy', {})).status()).toBe(400);

    const saved = await apiA.put('/security/mfa-policy', { grace_days: 3 });
    expect(saved.status()).toBe(200);
    expect((await saved.json()).grace_days).toBe(3);

    // A SECOND tenant must neither see nor be moved by the first one's window.
    const ctxB = await pwRequest.newContext();
    const { account: b } = await signUp(ctxB, 'polB');
    const apiB = authed(ctxB, b.token);

    const policyB = await apiB.get('/security/mfa-policy');
    expect(policyB.status()).toBe(200);
    expect((await policyB.json()).grace_days, "tenant B must not inherit tenant A's window").toBe(7);

    // And tenant A's stays where A put it.
    expect((await (await apiA.get('/security/mfa-policy')).json()).grace_days).toBe(3);

    await ctx.dispose();
    await ctxB.dispose();
  });

  test('a zero-day policy takes effect on the next login, and enforcement is server-side', async () => {
    // This is the enforcement half of the feature, exercised without a browser:
    // shrink the window to zero and watch the SAME account go from working to
    // refused, with the refusal coming from the API.
    const ctx = await pwRequest.newContext();
    const { account } = await signUp(ctx, 'enforce');
    const api = authed(ctx, account.token);

    expect((await api.get('/risks')).status(), 'inside the window the account works').toBe(200);

    expect((await api.put('/security/mfa-policy', { grace_days: 0 })).status()).toBe(200);

    // The request-time guard, on a session that was minted while the window was
    // open. Enforcing only at login would let this keep working.
    const blocked = await api.get('/risks');
    expect(blocked.status(), 'a live session must be refused once the window is zero').toBe(403);
    const err = (await blocked.json()) as { code?: string; mfa?: { required?: boolean } };
    expect(err.code).toBe('MFA_ENROLLMENT_REQUIRED');
    expect(err.mfa?.required).toBe(true);

    // The remedy stays reachable — a requirement you cannot satisfy is a
    // lockout, not a control.
    expect((await api.get('/auth/me')).status()).toBe(200);
    const setup = await api.post('/auth/mfa/setup', {});
    expect(setup.status(), 'enrolment must remain reachable while blocked').toBe(200);

    // Logging in again now stops at enrolment, exactly as before OR26-03 —
    // which is the behaviour a zero-day policy is asking for.
    const relogin = await ctx.post(`${API_URL}/auth/login`, {
      data: { email: account.email, password: account.password },
    });
    const reBody = (await relogin.json()) as LoginBody;
    expect(reBody.mfa_enrollment_required).toBe(true);
    expect(reBody.token_pair?.access_token).toBeFalsy();

    await ctx.dispose();
  });

  test('the client cannot talk its way out of the requirement', async () => {
    const ctx = await pwRequest.newContext();
    const { account } = await signUp(ctx, 'bypass');
    const api = authed(ctx, account.token);
    expect((await api.put('/security/mfa-policy', { grace_days: 0 })).status()).toBe(200);
    expect((await api.get('/risks')).status()).toBe(403);

    // 1. Forging the session contract in the request. The server resolves the
    //    state itself; nothing it reads comes from the client.
    const forged = await ctx.get(`${API_URL}/risks`, {
      headers: {
        Authorization: `Bearer ${account.token}`,
        'X-MFA-Configured': 'true',
        'X-MFA-Required': 'false',
      },
    });
    expect(forged.status()).toBe(403);

    // 2. Sending a body that claims compliance on a write route.
    const claimed = await api.post('/risks', {
      name: 'Bypass attempt',
      probability: 0.5,
      impact: 5,
      mfa: { state: 'configured', configured: true, required: false },
      mfa_configured: true,
    });
    expect(claimed.status()).toBe(403);

    // 3. Widening the window back out is itself a protected write, so a blocked
    //    account cannot restore its own access.
    expect((await api.put('/security/mfa-policy', { grace_days: 90 })).status()).toBe(403);

    // 4. Minting a personal access token — the standing-exemption route — is
    //    refused too.
    expect((await api.post('/auth/pat', { name: 'bypass', scopes: ['*'] })).status()).toBe(403);

    // 5. The token itself carries no MFA claim to tamper with in the first place.
    const claims = decodeJwt(account.token);
    expect(Object.keys(claims).join(',')).not.toMatch(/mfa/i);

    await ctx.dispose();
  });
});
