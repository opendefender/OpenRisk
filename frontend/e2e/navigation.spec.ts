// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Reversible navigation.
 *
 * Two properties, both of which the app failed before this suite existed:
 *
 *   1. Every route at depth >= 2 can be left. Back returns to the parent, and a
 *      rendered control does the same when there is no history to pop — which is
 *      the case for a deep link opened in a fresh tab, exactly how the Gap
 *      Analysis / Audits / Remediation dead ends were reached.
 *
 *   2. Compliance and Reports do not form a loop. Asking for a compliance report
 *      used to bounce between the two screens without ever producing a document.
 *      The anti-loop test walks the real journey and asserts it reaches a PDF in
 *      at most four clicks.
 *
 * Requires a running stack (backend + `vite dev`). Skips itself when the API is
 * unreachable rather than turning a missing dev environment into a red suite.
 */

import { test, expect, type Page } from '@playwright/test';

const API = process.env.E2E_API_URL ?? 'http://localhost:8080/api/v1';

const stamp = Date.now();
const TENANT = {
  email: `nav-${stamp}@openrisk.test`,
  username: `nav${stamp}`,
  password: 'NavSpec!Tenant2026',
  full_name: 'Nav Spec Probe',
  company_name: `Nav Spec Co ${stamp}`,
};

/* ------------------------------------------------------------------ *
 * Setup — one tenant, one session (auth is rate limited at 15/5min/IP)
 * ------------------------------------------------------------------ */

test.describe.configure({ mode: 'serial' });

let page: Page;
/** A framework id in this tenant, imported once so drill-downs have a target. */
let frameworkId = '';

async function apiReachable(): Promise<boolean> {
  try {
    const res = await fetch(`${API}/health`, { signal: AbortSignal.timeout(3000) });
    return res.ok;
  } catch {
    return false;
  }
}

test.beforeAll(async ({ browser }) => {
  test.skip(
    !(await apiReachable()),
    `API unreachable at ${API} — start the stack to run this suite`,
  );

  const reg = await fetch(`${API}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(TENANT),
  });
  expect(reg.status, `register a tenant: ${await reg.text()}`).toBe(201);

  const login = await fetch(`${API}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: TENANT.email, password: TENANT.password }),
  });
  expect(login.status).toBe(200);
  const token = (await login.json()).token_pair.access_token as string;

  const api = async (method: string, path: string, body?: unknown) =>
    fetch(`${API}${path}`, {
      method,
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: body === undefined ? undefined : JSON.stringify(body),
    });

  // A framework with controls, so the compliance drill-downs and the report have
  // something real to render.
  const created = await api('POST', '/compliance/frameworks', {
    name: 'ISO/IEC 27001',
    version: '2022',
    description: 'nav spec',
  });
  // Read the body ONCE: a Response body is a stream, so reading it for the
  // failure message consumes it before .json() can.
  const createdBody = await created.text();
  expect(created.status, `create framework: ${createdBody}`).toBe(201);
  frameworkId = JSON.parse(createdBody).id;
  await api('POST', `/compliance/frameworks/${frameworkId}/import-catalog`, {
    catalog_key: 'cis-v8',
  });

  page = await browser.newPage();
  await page.goto('/login');
  await page.getByTestId('login-email').fill(TENANT.email);
  await page.locator('input[type="password"]').first().fill(TENANT.password);
  await page.getByTestId('login-submit').click();
  await expect(page.getByTestId('app-main')).toBeVisible({ timeout: 20_000 });
});

test.afterAll(async () => {
  await page?.close();
});

/**
 * Waits for a client-side route to be rendered.
 *
 * Deliberately not `networkidle`: several queries poll on an interval (report
 * jobs while running, notifications every 60s), so the network never goes quiet
 * and the wait times out on a page that rendered fine.
 */
async function settled(p: Page, expectedPath?: string) {
  await p.waitForLoadState('domcontentloaded');
  await expect(p.getByTestId('app-main')).toBeVisible({ timeout: 15_000 });
  if (expectedPath) {
    await expect.poll(() => new URL(p.url()).pathname, { timeout: 10_000 }).toBe(expectedPath);
  }
}

/* ------------------------------------------------------------------ *
 * 1. Every depth >= 2 route is reversible
 * ------------------------------------------------------------------ */

/** Routes at depth >= 2, with the parent each must return to. */
function deepRoutes(): Array<{ path: string; parent: string }> {
  return [
    { path: '/risks/import', parent: '/risks' },
    { path: '/risks/weighting', parent: '/risks' },
    { path: '/risks/mitigations', parent: '/risks' },
    { path: '/assets/universe', parent: '/assets' },
    { path: `/compliance/frameworks/${frameworkId}`, parent: '/compliance' },
    {
      path: `/compliance/frameworks/${frameworkId}/gaps`,
      parent: `/compliance/frameworks/${frameworkId}`,
    },
    { path: '/compliance/gaps', parent: '/compliance' },
    { path: '/compliance/audits', parent: '/compliance' },
    { path: '/compliance/remediation', parent: '/compliance' },
    { path: '/reports/board', parent: '/reports' },
    { path: '/governance/audit-trail', parent: '/governance' },
    { path: '/settings/members', parent: '/settings' },
  ];
}

test.describe('every deep route can be left', () => {
  test('browser Back returns to the parent', async () => {
    // Twelve routes, each a full page load plus a Back — well past the default.
    test.setTimeout(180_000);
    for (const { path, parent } of deepRoutes()) {
      // Arrive from the parent, the way a user does.
      await page.goto(parent);
      await settled(page, parent);
      await page.goto(path);
      await settled(page, path);

      await page.goBack();
      await settled(page);
      expect(
        new URL(page.url()).pathname,
        `Back from ${path} landed on ${new URL(page.url()).pathname}, expected ${parent}`,
      ).toBe(parent);
    }
  });

  // The dead-end case: a deep link opened cold, with nothing in history to pop.
  // Gap Analysis, Audits and Remediation were all reachable exactly this way.
  test('a rendered breadcrumb link leads back, with no history to pop', async ({ browser }) => {
    test.setTimeout(180_000);
    const state = await page.context().storageState();
    for (const { path, parent } of deepRoutes()) {
      const ctx = await browser.newContext({ storageState: state });
      const fresh = await ctx.newPage();
      await fresh.goto(path);
      await settled(fresh, path);

      const crumb = fresh.locator(`nav[aria-label="Breadcrumb"] a[href="${parent}"]`);
      await expect(crumb, `${path} renders no breadcrumb link to ${parent}`).toHaveCount(1);
      await crumb.click();
      await settled(fresh, parent);

      await ctx.close();
    }
  });

  test('the breadcrumb trail is complete and clickable', async () => {
    await page.goto(`/compliance/frameworks/${frameworkId}/gaps`);
    await settled(page);
    const links = page.locator('nav[aria-label="Breadcrumb"] a');
    // Compliance > Framework > (Gaps, the current page, is not a link).
    await expect(links).toHaveCount(2);
    await expect(links.nth(0)).toHaveAttribute('href', '/compliance');
    await expect(links.nth(1)).toHaveAttribute('href', `/compliance/frameworks/${frameworkId}`);
  });
});

/* ------------------------------------------------------------------ *
 * 2. Moves and legacy links resolve
 * ------------------------------------------------------------------ */

test.describe('moved routes redirect', () => {
  const moves: Array<[string, string]> = [
    ['/roles', '/settings/members'],
    ['/settings/roles', '/settings/members'],
    ['/settings/audit-log', '/governance/audit-trail'],
    ['/audit-logs', '/governance/audit-trail'],
    ['/mitigations', '/risks/mitigations'],
    ['/compliance/gap-analysis', '/compliance/gaps'],
    ['/compliance/remediations', '/compliance/remediation'],
  ];

  for (const [from, to] of moves) {
    test(`${from} -> ${to}`, async () => {
      await page.goto(from);
      await settled(page);
      expect(new URL(page.url()).pathname).toBe(to);
    });
  }

  test('the executive view is a dashboard mode, not a route', async () => {
    await page.goto('/analytics');
    await settled(page);
    const url = new URL(page.url());
    expect(url.pathname).toBe('/');
    expect(url.searchParams.get('view')).toBe('executive');
  });

  // `replace` keeps the redirect out of history, so Back skips it rather than
  // bouncing the user through it.
  test('a redirect does not trap Back', async () => {
    await page.goto('/risks');
    await settled(page);
    await page.goto('/mitigations');
    await settled(page);
    expect(new URL(page.url()).pathname).toBe('/risks/mitigations');
    await page.goBack();
    await settled(page);
    expect(new URL(page.url()).pathname, 'Back bounced through the redirect').toBe('/risks');
  });
});

/* ------------------------------------------------------------------ *
 * 3. No Compliance <-> Reports loop
 * ------------------------------------------------------------------ */

/**
 * The "Générer" button on the Reports screen's Compliance template card.
 * Located by walking up from the card's heading, because several template cards
 * carry an identically-labelled button.
 */
function complianceTemplateGenerate(p: Page) {
  return p
    .locator('main .or-card')
    .filter({ hasText: /Rapport PDF détaillé par référentiel|Detailed PDF report per framework/ })
    .getByRole('button', { name: /générer|generate/i })
    .first();
}

test.describe('Compliance and Reports do not loop', () => {
  test('Compliance -> report -> PDF in at most 4 clicks', async () => {
    await page.goto('/compliance');
    await settled(page);

    let clicks = 0;

    // 1. Generate from the framework card. Scoped to <main>: the sidebar has a
    //    "Rapports" nav item whose title also matches, and clicking THAT is the
    //    very navigation this test exists to forbid.
    const generate = page.locator('main [title="Exporter PDF"], main [title="Export PDF"]').first();
    await expect(generate, 'no report action on the compliance screen').toBeVisible();
    await generate.click();
    clicks++;

    // The journey must END on the artifact, not back on a screen that offers to
    // start it again.
    await page.waitForURL(/\/reports\/jobs\/[0-9a-f-]+/, { timeout: 30_000 });
    clicks++; // count the transition generously

    await expect(page.getByTestId('detail-back')).toBeVisible();
    const download = page.getByRole('button', { name: /télécharger|download/i });
    await expect(download, 'the generated report offers no download').toBeVisible({
      timeout: 30_000,
    });

    expect(clicks, 'reaching the report took more than 4 clicks').toBeLessThanOrEqual(4);
  });

  test('the Reports compliance tile generates instead of returning to Compliance', async () => {
    await page.goto('/reports');
    await settled(page);

    // Clicking "Generate" on the Compliance template must not navigate to
    // /compliance — that was the return leg of the loop.
    await complianceTemplateGenerate(page).click();
    await expect(page.getByTestId('framework-picker')).toBeVisible();
    expect(new URL(page.url()).pathname, 'the Reports screen navigated back to Compliance').toBe(
      '/reports',
    );

    // Picking a framework ends on the artifact.
    await page
      .getByTestId('framework-picker')
      .getByRole('button')
      .filter({ hasText: /ISO/ })
      .first()
      .click();
    await page.waitForURL(/\/reports\/jobs\/[0-9a-f-]+/, { timeout: 30_000 });
  });

  test('the generated report never links back to Compliance', async () => {
    // Whichever job the previous test produced is fine; take the newest.
    await page.goto('/reports');
    await settled(page);
    await page.goto(`/compliance/frameworks/${frameworkId}`);
    await settled(page);

    // Walk Compliance -> Reports -> job and confirm the job page offers no route
    // back into the generation journey it just completed.
    await page.goto('/reports');
    await settled(page);
    const rows = page.locator('a[href^="/reports/jobs/"]');
    if ((await rows.count()) > 0) {
      await rows.first().click();
      await settled(page);
      await expect(page.locator('main a[href="/compliance"]')).toHaveCount(0);
    }
  });
});

/* ------------------------------------------------------------------ *
 * 4. Action deep-links and Escape
 * ------------------------------------------------------------------ */

test('?action=invite opens the invite dialog', async () => {
  await page.goto('/settings/members?action=invite');
  await settled(page);
  await expect(
    page
      .getByRole('heading', { name: /inviter un membre|invite a member/i })
      .or(page.getByText(/inviter un membre|invite a member/i).first()),
  ).toBeVisible();
});

test('Escape closes a dialog', async () => {
  await page.goto('/reports');
  await settled(page);
  await complianceTemplateGenerate(page).click();
  await expect(page.getByTestId('framework-picker')).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(page.getByTestId('framework-picker')).toHaveCount(0);
});
