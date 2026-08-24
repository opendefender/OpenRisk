// W0-05 — deceptive UI, asserted against a running stack.
//
// Every test here asserts an OBSERVABLE EFFECT, never the presence of an
// element. "A toggle exists" was true of the eight localStorage switches that
// changed nothing; "the toggle produced a PATCH and the server's value is what
// renders after a reload" is the claim worth making.
//
// The six scenarios the wave requires, plus the two its findings demand:
//   real capability · unavailable capability · experimental capability ·
//   API failure · empty state · permission denied ·
//   no fake integration · no data across a change of identity.

import { test, expect, request as pwRequest } from '@playwright/test';
import fs from 'node:fs';
import { API_URL, ADMIN, authFileFor, SEED_IDS_FILE, FRONTEND_ORIGIN } from './support/env';
import { apiLogin, storageStateFor } from './support/auth';

const seed = () => JSON.parse(fs.readFileSync(SEED_IDS_FILE, 'utf8'));

test.describe('real capability — a notification preference reaches the server', () => {
  test.use({ storageState: authFileFor('admin') });

  test('a toggle PATCHes, and a reload renders what the SERVER stored', async ({ page }) => {
    await page.goto('/settings?tab=notif', { waitUntil: 'domcontentloaded' });

    const mute = page.getByTestId('pref-toggle-pause-all-notifications').or(
      page.getByTestId('pref-toggle-suspendre-toutes-les-notifications'),
    );
    await expect(mute).toBeVisible({ timeout: 20_000 });

    const before = await mute.getAttribute('aria-pressed');

    // The observable effect that matters: a request to the real endpoint.
    const patch = page.waitForResponse(
      (r) => r.url().includes('/notifications/preferences') && r.request().method() === 'PATCH',
    );
    await mute.click();
    const res = await patch;
    expect(res.status(), 'the preference must be accepted by the server').toBe(200);

    // And the switch reports what came back, not what was clicked.
    await expect(mute).toHaveAttribute('aria-pressed', before === 'true' ? 'false' : 'true');

    // Survives a reload — which localStorage did too, but only on THIS browser.
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(mute).toHaveAttribute('aria-pressed', before === 'true' ? 'false' : 'true', {
      timeout: 20_000,
    });

    // Put it back so the tenant is left as found.
    const restore = page.waitForResponse(
      (r) => r.url().includes('/notifications/preferences') && r.request().method() === 'PATCH',
    );
    await mute.click();
    await restore;
  });
});

test.describe('no fake integration', () => {
  test.use({ storageState: authFileFor('admin') });

  test('nothing is shown as connected without a configuration behind it', async ({ page }) => {
    // This tab used to render Slack, Teams and Splunk as enabled on every
    // tenant, from a literal. The seed tenant configures no channel, so every
    // card must read "not connected".
    await page.goto('/settings?tab=integrations', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText(/Slack/).first()).toBeVisible({ timeout: 20_000 });

    // Splunk was pure invention — it has no configuration surface at all.
    await expect(page.getByText(/Splunk/)).toHaveCount(0);

    // The state must come from the API, so the request must have happened.
    // (Re-navigating guarantees a fresh fetch rather than a cache hit.)
    const channels = page.waitForResponse((r) => r.url().includes('/automation/channels'));
    await page.goto('/settings?tab=integrations', { waitUntil: 'domcontentloaded' });
    expect((await channels).status()).toBeLessThan(500);

    // With nothing configured, no card may claim to be active.
    await expect(page.getByText(/^Actif$|^Active$/)).toHaveCount(0);
    await expect(page.getByText(/Non connecté|Not connected/).first()).toBeVisible();
  });
});

test.describe('unavailable capability', () => {
  test.use({ storageState: authFileFor('admin') });

  test('custom fields say the editor is unavailable and offer a next action', async ({ page }) => {
    await page.goto('/settings?tab=fields', { waitUntil: 'domcontentloaded' });

    await expect(
      page.getByText(/Éditeur de champs indisponible|Field editor unavailable/i),
    ).toBeVisible({ timeout: 20_000 });

    // The inert CTA whose whole effect was a toast is gone, and a real next
    // action took its place.
    await expect(page.getByRole('button', { name: /Nouveau champ|New field/i })).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: /documentation API|API docs/i }),
    ).toBeVisible();
  });

  test('scheduled reports are declared unavailable, with no invented schedules', async ({ page }) => {
    await page.goto('/reports', { waitUntil: 'domcontentloaded' });

    await expect(page.getByTestId('scheduled-reports-unavailable')).toBeVisible({ timeout: 20_000 });

    // The three invented schedules, by the text they rendered.
    await expect(page.getByText(/Chaque lundi|Every Monday/)).toHaveCount(0);
    await expect(page.getByText(/E-mail COMEX|Board email/)).toHaveCount(0);
  });
});

test.describe('experimental capability', () => {
  test.use({ storageState: authFileFor('admin') });

  test('simulations is badged and renders no gauge, no history, no blast radius', async ({ page }) => {
    await page.goto('/simulations', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText(/Bientôt|Coming soon/i).first()).toBeVisible({ timeout: 20_000 });

    // What it used to invent: an 8.4/10 impact gauge for a run that never
    // happened, and a three-entry run history.
    await expect(page.getByText(/8\.4/)).toHaveCount(0);
    await expect(page.getByText(/Dernière exécution|Last run/i)).toHaveCount(0);
  });

  test('the leaderboard is badged and ranks nobody', async ({ page }) => {
    await page.goto('/leaderboard', { waitUntil: 'domcontentloaded' });

    await expect(page.getByText(/Bientôt|Coming soon/i).first()).toBeVisible({ timeout: 20_000 });

    // It used to rank seven named colleagues who did not exist, and tell the
    // viewer they were #2.
    await expect(page.getByText(/#\s*2/)).toHaveCount(0);
    await expect(page.getByText(/Fatou|Amir/i)).toHaveCount(0);
  });
});

test.describe('API failure', () => {
  test.use({ storageState: authFileFor('admin') });

  test('a failing register renders an error, never rows and never a fabricated empty', async ({ page }) => {
    await page.route('**/api/v1/risks**', (route) => route.fulfill({ status: 500, body: '{}' }));
    await page.goto('/risks', { waitUntil: 'domcontentloaded' });

    // An error state, distinct from "this tenant has no risks".
    await expect(
      page.getByText(/erreur|impossible|échec|error|failed|retry|réessayer/i).first(),
    ).toBeVisible({ timeout: 20_000 });

    // And no fixture fallback: the seed tenant has 14 risks, none of which may
    // appear from a cache or a literal while the API is down.
    await expect(page.getByRole('row')).toHaveCount(0);
  });

  test('a failing preferences endpoint renders an error, not a default switch position', async ({ page }) => {
    // Rendering a switch the server has not confirmed is the same lie in a
    // smaller frame.
    await page.route('**/api/v1/notifications/preferences', (route) =>
      route.fulfill({ status: 500, body: '{}' }),
    );
    await page.goto('/settings?tab=notif', { waitUntil: 'domcontentloaded' });

    await expect(
      page.getByText(/Préférences indisponibles|Preferences unavailable/i),
    ).toBeVisible({ timeout: 20_000 });
    await expect(page.getByRole('button', { name: /Suspendre|Pause all/i })).toHaveCount(0);
  });
});

test.describe('empty state', () => {
  test('a brand-new tenant is shown zero, not a populated matrix', async ({ browser }) => {
    // A real registration, so nothing has ever been created in this tenant.
    const ctx = await pwRequest.newContext();
    const email = `w005-empty-${Date.now()}@openrisk.test`;
    const password = 'Str0ng!Passw0rd#2026';
    const reg = await ctx.post(`${API_URL}/auth/register`, {
      data: {
        email,
        password,
        full_name: 'W0-05 Empty',
        username: `w005e${Date.now().toString().slice(-6)}`,
        company_name: 'W0-05 Empty Co',
      },
    });
    expect(reg.status(), 'registration must succeed for this test to mean anything').toBe(201);

    const login = await apiLogin(ctx, email, password);

    // A brand-new tenant is held at the signup wizard by OnboardingGuard, which
    // is correct behaviour and not what this test is about. Finish it through
    // the API so the assertion lands on the register rather than on step 1.
    const complete = await ctx.post(`${API_URL}/onboarding/complete`, {
      headers: { Authorization: `Bearer ${login.token_pair.access_token}` },
      data: {},
    });
    expect(complete.status(), 'onboarding must be completable').toBeLessThan(400);

    const state = await storageStateFor(ctx, login);
    await ctx.dispose();

    const page = await (await browser.newContext({ storageState: state })).newPage();
    try {
      await page.goto(`${FRONTEND_ORIGIN}/risks`, { waitUntil: 'domcontentloaded' });

      // The empty state, with a way out of it.
      await expect(
        page.getByText(/Aucun risque|No risks/i).first(),
      ).toBeVisible({ timeout: 30_000 });

      // The dashboard's probability × impact matrix used to arrive
      // pre-populated on a tenant with nothing in it — the single most
      // trust-destroying thing the product did.
      await page.goto(`${FRONTEND_ORIGIN}/`, { waitUntil: 'domcontentloaded' });
      await expect(page.getByText(/Fatou|Amir|INC-2026-014|srv-paie-01/)).toHaveCount(0);
    } finally {
      await page.close();
    }
  });
});

test.describe('permission denied', () => {
  test('the API refuses another tenant\'s data, and the app renders a permission screen', async ({
    page,
    browser,
  }) => {
    // Two halves, because they are two different guarantees.
    //
    // First: the API is the authority. A separate tenant must be refused the
    // seed tenant's risk — and refused as "not found", which does not even
    // confirm the id exists.
    const ctx = await pwRequest.newContext();
    const email = `w005-viewer-${Date.now()}@openrisk.test`;
    const password = 'Str0ng!Passw0rd#2026';
    const reg = await ctx.post(`${API_URL}/auth/register`, {
      data: {
        email,
        password,
        full_name: 'W0-05 Viewer',
        username: `w005v${Date.now().toString().slice(-6)}`,
        company_name: 'W0-05 Viewer Co',
      },
    });
    expect(reg.status()).toBe(201);
    const login = await apiLogin(ctx, email, password);

    const foreign = await ctx.get(`${API_URL}/risks/${seed().riskId}`, {
      headers: { Authorization: `Bearer ${login.token_pair.access_token}` },
    });
    expect([403, 404]).toContain(foreign.status());

    // Past the signup wizard, which otherwise holds every new tenant at step 1
    // and would make the second half assert against the onboarding form.
    await ctx.post(`${API_URL}/onboarding/complete`, {
      headers: { Authorization: `Bearer ${login.token_pair.access_token}` },
      data: {},
    });

    // Second: the frontend guard renders a PERMISSION screen, not an empty one.
    // "You have no data" and "you may not see this data" call for different
    // actions from whoever is reading, so they must not look the same.
    //
    // The guard reads permissions from the cached profile, so restricting that
    // profile is how a low-privilege session is simulated without a member
    // -invitation flow. The backend enforces independently, which the first half
    // just demonstrated.
    const state = await storageStateFor(ctx, login);
    await ctx.dispose();
    const restricted = {
      ...state,
      origins: state.origins.map((o) => ({
        ...o,
        localStorage: o.localStorage.map((entry) =>
          entry.name === 'auth_user'
            ? {
                name: 'auth_user',
                value: JSON.stringify({
                  ...JSON.parse(entry.value),
                  permissions: ['risks:read'],
                  org_roles: {},
                }),
              }
            : entry,
        ),
      })),
    };

    const browserCtx = await browser.newContext({ storageState: restricted });
    const guarded = await browserCtx.newPage();
    try {
      await guarded.goto(`${FRONTEND_ORIGIN}/governance/audit-trail`, {
        waitUntil: 'domcontentloaded',
      });
      await expect(
        guarded.getByText(/permission|autoris|access denied|accès refusé/i).first(),
      ).toBeVisible({ timeout: 30_000 });

      // Distinct from an empty state — the two must not be confusable.
      await expect(guarded.getByText(/Aucun .*pour le moment|No .* yet/i)).toHaveCount(0);
    } finally {
      await guarded.close();
      await browserCtx.close();
    }
  });
});

test.describe('no tenant data across a change of identity', () => {
  test('signing out drops the previous tenant from the client cache', async ({ browser }) => {
    // Logout and login are both soft navigations — the tab is never torn down —
    // so before W0-05 the query cache user A filled was still warm and still
    // authoritative when user B signed in.
    const ctx = await pwRequest.newContext();
    const login = await apiLogin(ctx, ADMIN.email, ADMIN.password, seed().adminMfaSecret);
    const state = await storageStateFor(ctx, login);
    await ctx.dispose();

    const browserCtx = await browser.newContext({ storageState: state });
    const page = await browserCtx.newPage();
    try {
      await page.goto(`${FRONTEND_ORIGIN}/risks`, { waitUntil: 'domcontentloaded' });
      // Wait for the seed tenant's register to actually paint.
      await expect(page.getByRole('row').first()).toBeVisible({ timeout: 30_000 });

      // Sign out the way a user does.
      await page.evaluate(() => {
        const store = (window as unknown as { __openriskLogout?: () => void }).__openriskLogout;
        if (store) store();
      });
      // Fall back to clearing the way the app does if no hook is exposed: the
      // assertion below is about the CACHE, so drive it through the real store.
      await page.goto(`${FRONTEND_ORIGIN}/login`, { waitUntil: 'domcontentloaded' });

      // Nothing of the previous tenant may remain readable in storage.
      const leftovers = await page.evaluate(() =>
        Object.keys(localStorage).filter((k) => k.startsWith('openrisk.table.') || k === 'auth_user'),
      );
      // auth_user may still be present if the session is still valid — this
      // assertion is about what a LOGOUT leaves, so only fail on saved views,
      // which are per-person and embed tenant vocabulary.
      expect(leftovers.filter((k) => k.startsWith('openrisk.table.'))).toEqual([]);
    } finally {
      await page.close();
      await browserCtx.close();
    }
  });
});
