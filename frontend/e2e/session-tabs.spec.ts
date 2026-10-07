// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Two tabs renewing the session at once (#700).
 *
 * Tabs of one browser share a cookie jar. Before D-048 (#777) the losing
 * request of a concurrent refresh pair answered REFRESH_REUSE_DETECTED and
 * cleared the session cookies, wiping the ones the winner had just set: every
 * tab ended on /login while the 30-day refresh cookie was still valid.
 *
 * Requires a running stack (backend + `vite dev`). Skipped automatically when
 * the API is unreachable, like e2e/empty-states.spec.ts.
 */

import { test, expect, type Browser, type BrowserContext, type Page } from '@playwright/test';

const API = process.env.E2E_API_URL ?? 'http://localhost:8080/api/v1';

const stamp = Date.now();

/** A dedicated account per run, created over the API. */
const ACCOUNT = {
  email: `tabs-e2e-${stamp}@openrisk.test`,
  password: 'Ancre-Vitrail7-Cobalt',
  full_name: 'Tabs E2E Probe',
  company_name: `Tabs E2E Co ${stamp}`,
};

let apiUp = false;

test.beforeAll(async ({ request }) => {
  try {
    const health = await request.get(`${API}/health`, { timeout: 4000 });
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
  test.skip(!apiUp, 'API unreachable — start the stack to run the session suite');
});

/** Signs in through the screen and opens a second tab on the same page. */
async function twoSignedInTabs(browser: Browser) {
  const context = await browser.newContext();
  const a = await context.newPage();
  await a.goto('/login');
  await a.getByTestId('login-email').fill(ACCOUNT.email);
  await a.getByTestId('login-password').fill(ACCOUNT.password);
  await a.getByTestId('login-submit').click();
  await expect(a).not.toHaveURL(/\/login/);
  const b = await context.newPage();
  await b.goto(new URL(a.url()).pathname);
  await expect(b).not.toHaveURL(/\/login/);
  return { context, a, b };
}

/** Every /auth/refresh status a page receives, in order. */
function refreshStatuses(page: Page): number[] {
  const seen: number[] = [];
  page.on('response', (r) => {
    if (r.url().includes('/auth/refresh')) seen.push(r.status());
  });
  return seen;
}

async function sessionCookies(context: BrowserContext): Promise<string[]> {
  return (await context.cookies())
    .map((c) => c.name)
    .filter((n) => n.startsWith('or_'))
    .sort();
}

/** POST /auth/refresh from inside the page, as the app's own client would. */
function refreshFromPage(page: Page, notBefore = 0) {
  return page.evaluate(async (when) => {
    const csrf = document.cookie.match(/(?:^|;\s*)or_csrf=([^;]*)/)?.[1] ?? '';
    while (Date.now() < when) await new Promise((r) => setTimeout(r, 1));
    const r = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      credentials: 'include',
      headers: { 'X-CSRF-Token': decodeURIComponent(csrf) },
    });
    return { status: r.status, body: await r.text() };
  }, notBefore);
}

test.describe('two tabs renewing the session at once', () => {
  test('both tabs reload with an expired access cookie and both stay signed in', async ({
    browser,
  }) => {
    const { context, a, b } = await twoSignedInTabs(browser);
    const seenA = refreshStatuses(a);
    const seenB = refreshStatuses(b);

    // What the browser does once the 15-minute access cookie lapses.
    await context.clearCookies({ name: 'or_access' });
    await Promise.all([a.reload(), b.reload()]);
    await Promise.all([a.waitForLoadState('networkidle'), b.waitForLoadState('networkidle')]);

    expect(seenA.length, 'tab A never renewed: the test would prove nothing').toBeGreaterThan(0);
    expect(seenB.length, 'tab B never renewed: the test would prove nothing').toBeGreaterThan(0);
    expect([...seenA, ...seenB], 'every refresh must succeed').toEqual(
      [...seenA, ...seenB].map(() => 200),
    );
    await expect(a).not.toHaveURL(/\/login/);
    await expect(b).not.toHaveURL(/\/login/);
    expect(await sessionCookies(context)).toEqual(['or_access', 'or_csrf', 'or_refresh']);
    for (const page of [a, b]) {
      const me = await page.evaluate(async () => (await fetch('/api/v1/auth/me')).status);
      expect(me).toBe(200);
    }
    await context.close();
  });

  test('both tabs call /auth/refresh at the same instant and keep one working session', async ({
    browser,
  }) => {
    // The issue's own reproduction: refreshes fired together from both tabs,
    // rather than left to the timing of two reloads.
    const { context, a, b } = await twoSignedInTabs(browser);
    const at = Date.now() + 500;
    const [ra, rb] = await Promise.all([refreshFromPage(a, at), refreshFromPage(b, at)]);

    expect(ra.status, ra.body).toBe(200);
    expect(rb.status, rb.body).toBe(200);
    expect(await sessionCookies(context)).toEqual(['or_access', 'or_csrf', 'or_refresh']);
    for (const page of [a, b]) {
      const me = await page.evaluate(async () => (await fetch('/api/v1/auth/me')).status);
      expect(me).toBe(200);
    }
    // The session still renews afterwards: the successor both tabs share is live.
    expect((await refreshFromPage(a)).status).toBe(200);
    await context.close();
  });
});
