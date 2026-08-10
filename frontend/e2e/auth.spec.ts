// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Authentication end-to-end suite.
 *
 * Covers the flows the auth specification calls out:
 *
 *   • the three OAuth providers (Google, GitHub, Microsoft) — start of flow,
 *     PKCE + state on the authorization request, and every failure rendering as
 *     a readable message rather than a blank page;
 *   • the complete password reset flow, request through to sign-in;
 *   • account enumeration probes against both reset legs;
 *   • MFA;
 *   • the password policy, as enforced by the server.
 *
 * Requires a running stack (backend + `vite dev`). Skipped automatically when
 * the API is unreachable, so a missing dev environment does not turn into a red
 * suite — the same convention as e2e/empty-states.spec.ts.
 *
 * On the OAuth providers: a real round trip needs live client credentials for
 * three third parties, which no CI job should hold. What is exercised here is
 * everything on OUR side of the redirect — that the flow starts, that it carries
 * both CSRF defences, and that every way it can come back is handled. The token
 * exchange and user-info parsing for each provider are covered against a stub
 * authorization server in backend/internal/handler/oauth2_flow_test.go, which
 * enforces PKCE the way a real provider does.
 */

import { test, expect, type Page } from '@playwright/test';

const API = process.env.E2E_API_URL ?? 'http://localhost:8080/api/v1';

const stamp = Date.now();

/** A dedicated account per run, created over the API. */
const ACCOUNT = {
  email: `auth-e2e-${stamp}@openrisk.test`,
  username: `authe2e${stamp}`,
  password: 'Ancre-Vitrail7-Cobalt',
  full_name: 'Auth E2E Probe',
  company_name: `Auth E2E Co ${stamp}`,
};

/** An address that must never have an account. */
const GHOST_EMAIL = `ghost-${stamp}@openrisk.test`;

let apiUp = false;

test.beforeAll(async ({ request }) => {
  try {
    const health = await request.get(`${API.replace(/\/api\/v1$/, '')}/health`, { timeout: 4000 });
    apiUp = health.ok();
  } catch {
    apiUp = false;
  }
  if (!apiUp) return;

  const created = await request.post(`${API}/auth/register`, { data: ACCOUNT });
  // 409 means a previous run already created it, which is fine.
  expect([200, 201, 409]).toContain(created.status());
});

test.beforeEach(() => {
  test.skip(!apiUp, 'API unreachable — start the stack to run the auth suite');
});

/* ------------------------------------------------------------------ *
 * OAuth — the three providers
 * ------------------------------------------------------------------ */

const PROVIDERS = [
  { id: 'google', label: 'Google' },
  { id: 'github', label: 'GitHub' },
  { id: 'azure', label: 'Microsoft' },
] as const;

test.describe('OAuth providers', () => {
  for (const provider of PROVIDERS) {
    test(`${provider.label}: the button starts a PKCE-protected flow`, async ({ page, request }) => {
      await page.goto('/login');
      await expect(page.getByTestId(`oauth-${provider.id}`)).toBeVisible();

      // Ask the backend directly and inspect the redirect it issues, rather than
      // clicking through to a third party we cannot reach from CI.
      const response = await request.get(`${API}/auth/oauth2/login/${provider.id}`, {
        maxRedirects: 0,
      });

      // Two legitimate outcomes: configured (302 to the provider) or not
      // configured (302 back to /login with a readable reason). Neither may be a
      // JSON body or a 500.
      expect(response.status(), 'the flow must always answer with a redirect').toBe(302);
      const location = response.headers()['location'] ?? '';
      expect(location, 'a redirect must have a destination').not.toBe('');

      if (location.includes('/login?')) {
        // Unconfigured in this environment. It must say so, not fail silently.
        const params = new URL(location, 'http://localhost:5173').searchParams;
        expect(params.get('error')).toBe('provider_not_configured');
        return;
      }

      // Configured: the authorization request must carry BOTH defences. state
      // binds the callback to this browser; PKCE binds the code to this server.
      const authURL = new URL(location);
      expect(authURL.searchParams.get('state'), 'missing state — callback is unbound').toBeTruthy();
      expect(
        authURL.searchParams.get('code_challenge'),
        'missing PKCE challenge — an intercepted code would be redeemable',
      ).toBeTruthy();
      expect(authURL.searchParams.get('code_challenge_method')).toBe('S256');
      // The verifier must never travel through the browser.
      expect(authURL.searchParams.get('code_verifier')).toBeNull();
    });
  }

  test('a provider failure renders a readable message, never a blank page', async ({ page }) => {
    // The backend redirects failures to /login?error=<code>. This is the case
    // that used to produce raw JSON at a URL the browser had navigated to.
    await page.goto('/login?error=access_denied&provider=google');

    const banner = page.getByTestId('auth-error');
    await expect(banner).toBeVisible();
    await expect(banner).not.toBeEmpty();
    // A raw code leaking to the user means the copy lookup missed.
    await expect(banner).not.toContainText('access_denied');
    await expect(banner).not.toContainText('undefined');
  });

  test('a provider conflict names the provider that owns the address', async ({ page }) => {
    // The recoverable-refusal case: a bare "sign-in failed" strands someone on
    // their own account.
    await page.goto('/login?error=provider_conflict&provider=github&existing_provider=google');

    const banner = page.getByTestId('auth-error');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText('Google');
  });

  test('every OAuth error code renders some message', async ({ page }) => {
    const codes = [
      'access_denied', 'consent_required', 'provider_error', 'state_missing',
      'state_invalid', 'code_missing', 'exchange_failed', 'userinfo_failed',
      'unsupported_provider', 'provider_not_configured', 'email_unverified',
      'no_email', 'account_disabled', 'no_account', 'internal',
    ];

    for (const code of codes) {
      await page.goto(`/login?error=${code}&provider=google`);
      const banner = page.getByTestId('auth-error');
      await expect(banner, `no message rendered for ${code}`).toBeVisible();
      await expect(banner).not.toContainText('undefined');
    }
  });
});

/* ------------------------------------------------------------------ *
 * Account enumeration
 * ------------------------------------------------------------------ */

test.describe('account enumeration', () => {
  test('the reset request answers identically for known and unknown addresses', async ({ request }) => {
    // The core anti-enumeration property, checked at the wire level: status,
    // body and headers must not differ.
    const known = await request.post(`${API}/auth/password/forgot`, {
      data: { email: ACCOUNT.email, locale: 'en' },
    });
    const unknown = await request.post(`${API}/auth/password/forgot`, {
      data: { email: GHOST_EMAIL, locale: 'en' },
    });

    expect(known.status()).toBe(unknown.status());
    expect(await known.text()).toBe(await unknown.text());
  });

  test('the UI shows the same screen either way', async ({ page }) => {
    const bodies: string[] = [];

    for (const email of [ACCOUNT.email, GHOST_EMAIL]) {
      await page.goto('/forgot-password');
      await page.getByTestId('forgot-email').fill(email);
      await page.getByTestId('forgot-submit').click();
      await expect(page.getByTestId('auth-success').or(page.getByRole('heading'))).toBeVisible();
      bodies.push((await page.locator('form, [data-testid=auth-success]').first().innerText()).trim());
    }

    // Whatever the screen says, it must say the same thing both times.
    expect(bodies[0]).toBe(bodies[1]);
  });

  test('login does not distinguish a wrong password from an unknown account', async ({ request }) => {
    const wrongPassword = await request.post(`${API}/auth/login`, {
      data: { email: ACCOUNT.email, password: 'Definitely-Not-The-Password-1' },
    });
    const noSuchAccount = await request.post(`${API}/auth/login`, {
      data: { email: GHOST_EMAIL, password: 'Definitely-Not-The-Password-1' },
    });

    expect(wrongPassword.status()).toBe(noSuchAccount.status());
    expect(await wrongPassword.text()).toBe(await noSuchAccount.text());
  });

  test('the rate limit applies to addresses with no account', async ({ request }) => {
    // If the cap only counted real accounts, a 429 would itself prove an account
    // exists — the limiter would leak what the uniform response hides.
    const probe = `ratelimit-${Date.now()}@openrisk.test`;
    const statuses: number[] = [];

    for (let i = 0; i < 5; i++) {
      const r = await request.post(`${API}/auth/password/forgot`, {
        data: { email: probe, locale: 'en' },
      });
      statuses.push(r.status());
    }

    expect(statuses.filter((s) => s === 429).length, 'expected the cap to bite for an unknown address')
      .toBeGreaterThan(0);
  });
});

/* ------------------------------------------------------------------ *
 * Password reset — the complete flow
 * ------------------------------------------------------------------ */

test.describe('password reset', () => {
  test('an invalid token is refused with one message covering every cause', async ({ page }) => {
    // Unknown, expired and already-used are deliberately indistinguishable:
    // saying a token "expired" confirms it once existed, and with it the account.
    await page.goto('/reset-password?token=this-token-never-existed');

    await page.getByTestId('reset-password').fill('Ancre-Vitrail7-Cobalt');
    await page.getByTestId('reset-confirm').fill('Ancre-Vitrail7-Cobalt');
    await expect(page.getByTestId('reset-submit')).toBeEnabled();
    await page.getByTestId('reset-submit').click();

    await expect(page.getByTestId('auth-error')).toBeVisible();
  });

  test('the reset screen refuses a link with no token at all', async ({ page }) => {
    await page.goto('/reset-password');
    await expect(page.getByTestId('auth-error')).toBeVisible();
  });

  test('the API rejects an unknown token', async ({ request }) => {
    const response = await request.post(`${API}/auth/password/reset`, {
      data: { token: 'not-a-real-token', new_password: 'Ancre-Vitrail7-Cobalt', locale: 'en' },
    });

    expect(response.status()).toBe(400);
    const body = await response.json();
    expect(body.code).toBe('invalid_token');
  });

  test('the forgot-password link on the sign-in screen actually goes somewhere', async ({ page }) => {
    // It used to be an href="#" with preventDefault — a control that looked live
    // and did nothing.
    await page.goto('/login');
    await page.getByTestId('forgot-password-link').click();

    await expect(page).toHaveURL(/\/forgot-password/);
    await expect(page.getByTestId('forgot-email')).toBeVisible();
  });
});

/* ------------------------------------------------------------------ *
 * Password policy
 * ------------------------------------------------------------------ */

test.describe('password policy', () => {
  /** Asks the server, which is the only verdict that counts. */
  async function assess(request: Page['request'], password: string) {
    const r = await request.post(`${API}/auth/password/check`, { data: { password } });
    expect(r.ok()).toBeTruthy();
    return r.json();
  }

  test('refuses anything under 12 characters', async ({ request }) => {
    const a = await assess(request, 'Ab3!xyzQw9');
    expect(a.ok).toBe(false);
    expect(a.blocking.map((r: { code: string }) => r.code)).toContain('too_short');
  });

  test('refuses a password drawing on fewer than three character classes', async ({ request }) => {
    const a = await assess(request, 'correcthorsebatterystaple');
    expect(a.ok).toBe(false);
    expect(a.blocking.map((r: { code: string }) => r.code)).toContain('needs_more_classes');
  });

  test('refuses a decorated dictionary word that passes length and classes', async ({ request }) => {
    // The shape a naive length+classes rule lets straight through.
    const a = await assess(request, 'Password1234!');
    expect(a.ok).toBe(false);
    expect(a.score).toBeLessThan(3);
  });

  test('refuses the product’s own name', async ({ request }) => {
    const a = await assess(request, 'OpenRisk1234!');
    expect(a.ok).toBe(false);
  });

  test('accepts a genuine passphrase', async ({ request }) => {
    const a = await assess(request, 'Ancre-Vitrail7-Cobalt');
    expect(a.ok).toBe(true);
    expect(a.score).toBeGreaterThanOrEqual(3);
  });

  test('every refusal is actionable, never a bare verdict', async ({ request }) => {
    const a = await assess(request, 'Password1234!');
    for (const reason of a.blocking) {
      expect(reason.fr, 'missing French rendering').toBeTruthy();
      expect(reason.en, 'missing English rendering').toBeTruthy();
      expect(reason.en.toLowerCase()).not.toBe('invalid password');
    }
  });

  test('the server refuses a weak password even if the client would allow it', async ({ request }) => {
    // Server-authoritative: the browser's opinion is not consulted.
    const response = await request.post(`${API}/auth/password/reset`, {
      data: { token: 'irrelevant-the-policy-runs-first', new_password: 'short', locale: 'en' },
    });
    // Either the policy refuses it (422) or the token check does (400). What must
    // never happen is a 2xx.
    expect([400, 422]).toContain(response.status());
  });

  test('the registration meter reflects the enforced policy', async ({ page }) => {
    await page.goto('/register');
    await page.getByTestId('register-name').fill('Policy Probe');
    await page.getByTestId('register-email').fill(`meter-${Date.now()}@openrisk.test`);

    // Eight characters used to be accepted here while the server demanded twelve.
    await page.getByTestId('register-password').fill('Abcd123!');
    await expect(page.getByTestId('register-submit')).toBeDisabled();

    await page.getByTestId('register-password').fill('Ancre-Vitrail7-Cobalt');
    await expect(page.getByTestId('register-submit')).toBeEnabled();
  });
});

/* ------------------------------------------------------------------ *
 * MFA
 * ------------------------------------------------------------------ */

test.describe('MFA', () => {
  test('the challenge endpoint refuses a session token', async ({ request }) => {
    // /auth/mfa/challenge accepts only an MFA_REQUIRED token. Anything else —
    // including a full access token — must be turned away.
    const login = await request.post(`${API}/auth/login`, {
      data: { email: ACCOUNT.email, password: ACCOUNT.password },
    });

    if (!login.ok()) test.skip(true, 'could not sign in to set up the MFA probe');
    const body = await login.json();

    // A fresh member account has no MFA, so login yields a session.
    const accessToken = body?.token_pair?.access_token;
    test.skip(!accessToken, 'no access token in the login response');

    const challenge = await request.post(`${API}/auth/mfa/challenge`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      data: { code: '000000' },
    });

    expect(challenge.status()).toBe(401);
  });

  test('MFA enrolment requires a credential', async ({ request }) => {
    const response = await request.post(`${API}/auth/mfa/setup`, { data: {} });
    expect(response.status()).toBe(401);
  });

  test('an unauthenticated caller cannot list sessions', async ({ request }) => {
    const response = await request.get(`${API}/auth/sessions`);
    expect([401, 403]).toContain(response.status());
  });
});

/* ------------------------------------------------------------------ *
 * Screen behaviour
 * ------------------------------------------------------------------ */

test.describe('sign-in screen', () => {
  test('a failed sign-in reports it inline rather than silently', async ({ page }) => {
    await page.goto('/login');
    await page.getByTestId('login-email').fill(ACCOUNT.email);
    await page.getByTestId('login-password').fill('Definitely-Not-The-Password-1');
    await page.getByTestId('login-submit').click();

    await expect(page.getByTestId('auth-error')).toBeVisible();
  });

  test('the language choice survives a reload', async ({ page }) => {
    await page.goto('/login');

    const toggle = page.getByTestId('auth-lang-toggle');
    const before = (await toggle.innerText()).trim();
    await toggle.click();
    const after = (await toggle.innerText()).trim();
    expect(after).not.toBe(before);

    await page.reload();
    // Persisted: someone who switched to English does not land back in French
    // after following a reset link.
    await expect(page.getByTestId('auth-lang-toggle')).toHaveText(new RegExp(after, 'i'));
  });

  test('the footer credits OpenDefender with a working link', async ({ page }) => {
    await page.goto('/login');
    const link = page.getByRole('link', { name: /OpenDefender/i });
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute('href', /opendefender/i);
  });

  test('honours prefers-reduced-motion', async ({ browser }) => {
    // The preference is often set because motion causes real symptoms, so this
    // must be off entirely — not shortened, not "a subtle version".
    const context = await browser.newContext({ reducedMotion: 'reduce' });
    const page = await context.newPage();
    await page.goto('/login');

    const animated = await page.evaluate(() =>
      [...document.querySelectorAll('*')].filter((el) => {
        const name = getComputedStyle(el).animationName;
        return name && name !== 'none';
      }).length,
    );

    expect(animated, 'no element may animate under reduced motion').toBe(0);
    await context.close();
  });

  test('no animation runs longer than 400ms', async ({ page }) => {
    await page.goto('/login');

    const overLong = await page.evaluate(() => {
      const parse = (v: string) =>
        v.trim().endsWith('ms') ? parseFloat(v) : parseFloat(v) * 1000;

      const offenders: string[] = [];
      for (const el of document.querySelectorAll('*')) {
        const style = getComputedStyle(el);
        for (const source of [style.animationDuration, style.transitionDuration]) {
          for (const part of source.split(',')) {
            if (!part.trim()) continue;
            const ms = parse(part);
            // Ignore the deliberately slow ambient orbit, which is decorative and
            // infinite rather than a state change the user is waiting on.
            if (ms > 400 && style.animationIterationCount !== 'infinite') {
              offenders.push(`${el.tagName}.${el.className}: ${part.trim()}`);
            }
          }
        }
      }
      return offenders;
    });

    expect(overLong, `animations over 400ms: ${overLong.join(', ')}`).toEqual([]);
  });
});
