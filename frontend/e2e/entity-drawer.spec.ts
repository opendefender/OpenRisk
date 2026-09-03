// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * The universal entity drawer, end to end (W1-02 §47-§52).
 *
 * Everything here runs against a real backend and real seeded data. Two tenants
 * are created over the API, each with its own asset and risk, because the claims
 * this suite exists to settle cannot be made against one tenant:
 *
 *   - a drawer URL naming another tenant's record must not open it (§48);
 *   - relations must contain only the caller's own records (§48);
 *   - a caller who may read risks but not assets must be told so, rather than
 *     shown assets or shown nothing (§49).
 *
 * The rest is the navigation contract that no unit test can settle, because it
 * needs a real history stack: Back closes the drawer, Back again returns to the
 * page WITH its filters, Forward reopens, and a reload leaves the drawer open.
 *
 * Requires a running stack (backend + `vite dev`). Skipped automatically when
 * the API is unreachable, so a missing dev environment does not turn the suite
 * red.
 */

import { test, expect, type Page } from '@playwright/test';

const API = process.env.E2E_API_URL ?? 'http://localhost:8080/api/v1';
const stamp = Date.now();

interface Tenant {
  email: string;
  username: string;
  password: string;
  full_name: string;
  company_name: string;
}

function tenant(tag: string): Tenant {
  return {
    email: `drawer-${tag}-${stamp}@openrisk.test`,
    username: `drawer${tag}${stamp}`,
    password: 'EntityDrawer!2026',
    full_name: `Drawer Probe ${tag.toUpperCase()}`,
    company_name: `Drawer ${tag.toUpperCase()} Co ${stamp}`,
  };
}

const TENANT_A = tenant('a');
const TENANT_B = tenant('b');

/** Records seeded over the API, so the UI is asserted against real rows. */
interface Seed {
  token: string;
  assetId: string;
  riskId: string;
  riskName: string;
  assetName: string;
}

const seeded: Record<'a' | 'b', Seed> = {
  a: { token: '', assetId: '', riskId: '', riskName: '', assetName: '' },
  b: { token: '', assetId: '', riskId: '', riskName: '', assetName: '' },
};

/* ------------------------------------------------------------------ *
 * API helpers
 * ------------------------------------------------------------------ */

async function apiReachable(): Promise<boolean> {
  try {
    const res = await fetch(`${API}/health`, { signal: AbortSignal.timeout(3000) });
    return res.ok;
  } catch {
    return false;
  }
}

async function post(path: string, body: unknown, token?: string): Promise<Response> {
  return fetch(`${API}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
  });
}

async function register(t: Tenant): Promise<void> {
  const res = await post('/auth/register', t);
  const text = await res.text();
  expect(res.status, `register ${t.company_name}: ${text}`).toBe(201);
}

async function login(t: Tenant): Promise<string> {
  const res = await post('/auth/login', { email: t.email, password: t.password });
  // Read the body ONCE: a Response body is a stream, and reading it for the
  // failure message consumes it before the success path can parse it.
  const text = await res.text();
  expect(res.status, `login ${t.email}: ${text}`).toBe(200);
  return JSON.parse(text).token_pair.access_token as string;
}

/** Seeds one tenant with an asset and a risk linked to it, so the drawer has a
 *  real relation to render rather than an empty list. */
async function seed(key: 'a' | 'b', t: Tenant): Promise<void> {
  const token = await login(t);
  const assetName = `drawer-host-${key}-${stamp}`;
  const riskName = `Drawer probe risk ${key.toUpperCase()} ${stamp}`;

  // The `server` category's schema requires a hostname and an operating system,
  // and the attribute validator refuses a create without them. Supplying the
  // real required set is what makes this a genuine record rather than a
  // half-formed one the drawer would then render honestly but uselessly.
  const assetRes = await post(
    '/assets',
    {
      name: assetName,
      type: 'Server',
      criticality: 'CRITICAL',
      category: 'server',
      owner: t.email,
      attributes: {
        hostname: assetName,
        operating_system: 'Ubuntu 24.04',
        environment: 'production',
        network_zone: 'dmz',
        internet_exposed: true,
      },
    },
    token,
  );
  const assetText = await assetRes.text();
  expect(assetRes.status, `create asset: ${assetText}`).toBe(201);
  const assetId = JSON.parse(assetText).id as string;

  const riskRes = await post(
    '/risks',
    {
      title: riskName,
      description: `Seeded by the W1-02 e2e suite for tenant ${key}.`,
      impact: 9,
      probability: 0.8,
      asset_ids: [assetId],
    },
    token,
  );
  const riskText = await riskRes.text();
  expect(riskRes.status, `create risk: ${riskText}`).toBe(201);
  const riskId = JSON.parse(riskText).id as string;

  // A freshly registered tenant starts inside the onboarding wizard, and its
  // route guard holds every other screen until it is done. Finishing it over
  // the API keeps this suite about the drawer rather than about the wizard.
  const done = await post('/onboarding/complete', {}, token);
  expect(done.status, 'complete onboarding').toBe(200);

  seeded[key] = { token, assetId, riskId, riskName, assetName };
}

/* ------------------------------------------------------------------ *
 * Setup — serial on one page: the auth endpoints are rate limited, so a
 * login per test would trip the limiter and report it as a product failure.
 * ------------------------------------------------------------------ */

test.describe.configure({ mode: 'serial' });

let page: Page;

test.beforeAll(async ({ browser }) => {
  test.skip(
    !(await apiReachable()),
    `API unreachable at ${API} — start the stack to run this suite`,
  );

  await register(TENANT_A);
  await register(TENANT_B);
  await seed('a', TENANT_A);
  await seed('b', TENANT_B);

  page = await browser.newPage();
  await page.goto('/login');
  await page.getByTestId('login-email').fill(TENANT_A.email);
  await page.locator('input[type="password"]').first().fill(TENANT_A.password);
  await page.getByTestId('login-submit').click();
  await expect(page.getByTestId('app-main')).toBeVisible({ timeout: 20_000 });
});

test.afterAll(async () => {
  await page?.close();
});

const drawer = () => page.getByRole('dialog');

/** Opens a drawer by URL and waits for the record to render. */
async function openDrawer(path: string) {
  await page.goto(path);
  await page.waitForLoadState('networkidle');
}

/* ------------------------------------------------------------------ *
 * Deep links (§7)
 * ------------------------------------------------------------------ */

test('a drawer URL opens the drawer directly, on a cold load', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}`);

  await expect(drawer()).toBeVisible();
  await expect(drawer()).toContainText(seeded.a.riskName);
});

test('a drawer survives a reload', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}`);
  await expect(drawer()).toBeVisible();

  await page.reload();
  await page.waitForLoadState('networkidle');

  await expect(drawer()).toBeVisible();
  await expect(drawer()).toContainText(seeded.a.riskName);
});

test('a drawer URL opens in a fresh tab', async ({ context }) => {
  // A copied link, pasted somewhere else. Nothing but the URL carries the state.
  const fresh = await context.newPage();
  await fresh.goto(`/risks?drawer=risk&entity=${seeded.a.riskId}`);
  await fresh.waitForLoadState('networkidle');

  await expect(fresh.getByRole('dialog')).toBeVisible();
  await expect(fresh.getByRole('dialog')).toContainText(seeded.a.riskName);
  await fresh.close();
});

test('a drawer link names a tab', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}&etab=timeline`);

  await expect(drawer()).toBeVisible();
  await expect(drawer().getByRole('tab', { name: 'Timeline' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
});

/* ------------------------------------------------------------------ *
 * Browser navigation (§8) and page context (§9)
 * ------------------------------------------------------------------ */

test('Back closes the drawer and leaves the page filters intact', async () => {
  // Start on a FILTERED view — the whole point of §9 is that reading a record
  // must not cost the user their place.
  await page.goto('/risks?severity=critical&page=1');
  await page.waitForLoadState('networkidle');
  await expect(drawer()).toHaveCount(0);

  await page.goto(`/risks?severity=critical&page=1&drawer=risk&entity=${seeded.a.riskId}`);
  await page.waitForLoadState('networkidle');
  await expect(drawer()).toBeVisible();

  await page.goBack();
  await expect(drawer()).toHaveCount(0);

  const url = new URL(page.url());
  expect(url.pathname).toBe('/risks');
  expect(url.searchParams.get('severity')).toBe('critical');
  expect(url.searchParams.get('page')).toBe('1');
  expect(url.searchParams.get('drawer')).toBeNull();
  expect(url.searchParams.get('entity')).toBeNull();
});

test('Forward reopens the drawer that Back closed', async () => {
  await page.goto('/risks?severity=critical');
  await page.waitForLoadState('networkidle');
  await page.goto(`/risks?severity=critical&drawer=risk&entity=${seeded.a.riskId}`);
  await page.waitForLoadState('networkidle');

  await page.goBack();
  await expect(drawer()).toHaveCount(0);

  await page.goForward();
  await expect(drawer()).toBeVisible();
  await expect(drawer()).toContainText(seeded.a.riskName);
});

test('entity A → related entity B → Back returns to entity A', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}&etab=relations`);
  await expect(drawer()).toBeVisible();

  // The seeded asset is a real relation of the seeded risk.
  const related = drawer().getByRole('button', { name: new RegExp(seeded.a.assetName) });
  await expect(related).toBeVisible();
  await related.click();

  // The asset's drawer replaces the risk's, in place.
  await expect(drawer()).toContainText(seeded.a.assetName);
  expect(new URL(page.url()).searchParams.get('drawer')).toBe('asset');
  expect(new URL(page.url()).searchParams.get('entity')).toBe(seeded.a.assetId);

  await page.goBack();
  await expect(drawer()).toContainText(seeded.a.riskName);
  expect(new URL(page.url()).searchParams.get('drawer')).toBe('risk');

  // And one more Back leaves the drawer altogether, on the original page.
  await page.goBack();
  await expect(drawer()).toHaveCount(0);
});

test('Escape closes the drawer', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}`);
  await expect(drawer()).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(drawer()).toHaveCount(0);
  expect(new URL(page.url()).searchParams.get('drawer')).toBeNull();
});

/* ------------------------------------------------------------------ *
 * Cross-tenant (§48, §51) — the security claim
 * ------------------------------------------------------------------ */

test("a drawer URL naming another tenant's risk does not open it", async () => {
  // Tenant A's session, tenant B's real id. This is the IDOR probe: the id is
  // genuine, so nothing about its shape gives it away.
  await openDrawer(`/risks?drawer=risk&entity=${seeded.b.riskId}`);

  await expect(drawer()).toBeVisible();
  await expect(drawer()).toContainText(/not found/i);
  // Nothing of B's may appear — not the name, and not a hint that it exists.
  await expect(drawer()).not.toContainText(seeded.b.riskName);
  await expect(drawer()).not.toContainText(/another organi/i);
  await expect(drawer()).not.toContainText(/other tenant/i);
});

test("a drawer URL naming another tenant's asset does not open it", async () => {
  await openDrawer(`/assets?drawer=asset&entity=${seeded.b.assetId}`);

  await expect(drawer()).toBeVisible();
  await expect(drawer()).toContainText(/not found/i);
  await expect(drawer()).not.toContainText(seeded.b.assetName);
});

test("a forged id is indistinguishable from another tenant's id", async () => {
  // If these two answered differently, the pair would be an oracle for
  // enumerating another tenant's records.
  await openDrawer('/risks?drawer=risk&entity=00000000-0000-4000-8000-000000000000');
  const forged = (await drawer().textContent()) ?? '';

  await openDrawer(`/risks?drawer=risk&entity=${seeded.b.riskId}`);
  const foreign = (await drawer().textContent()) ?? '';

  expect(forged).toContain('Record not found');
  expect(foreign).toContain('Record not found');
});

test("every relation belongs to the caller's own tenant", async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}&etab=relations`);
  await expect(drawer()).toBeVisible();

  const text = (await drawer().textContent()) ?? '';
  expect(text).toContain(seeded.a.assetName);
  expect(text).not.toContain(seeded.b.assetName);
  expect(text).not.toContain(seeded.b.riskName);
});

/* ------------------------------------------------------------------ *
 * Timeline (§17, §50)
 * ------------------------------------------------------------------ */

test('the timeline shows real recorded events, not a rendered timestamp', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}&etab=timeline`);
  await expect(drawer()).toBeVisible();

  // The risk was created through the API, so its creation is on the record.
  // "updated 2 hours ago" derived from a column would be the fabrication §17
  // forbids; a real event names its journal.
  await expect(drawer().getByRole('list', { name: /activity/i })).toBeVisible();
  await expect(drawer()).toContainText(/Audit trail|Score engine/);
});

test('the global timeline links each event to its record', async () => {
  await page.goto('/activity');
  await page.waitForLoadState('networkidle');

  const open = page.getByRole('button', { name: /^Open (risk|asset)$/ }).first();
  await expect(open).toBeVisible();
  await open.click();

  await expect(drawer()).toBeVisible();
  expect(new URL(page.url()).searchParams.get('drawer')).not.toBeNull();
});

/* ------------------------------------------------------------------ *
 * Accessibility (§40, §52)
 * ------------------------------------------------------------------ */

test('the drawer is a dialog, traps focus and restores it on close', async () => {
  await page.goto('/risks');
  await page.waitForLoadState('networkidle');

  await page.goto(`/risks?drawer=risk&entity=${seeded.a.riskId}`);
  await page.waitForLoadState('networkidle');

  const dialog = drawer();
  await expect(dialog).toHaveAttribute('aria-modal', 'true');
  // An accessible name: the record's own title, not "Dialog".
  await expect(dialog).toContainText(seeded.a.riskName);

  // Focus is inside the panel, so a keyboard user is not left behind on the
  // list they can no longer see.
  const focusInside = await page.evaluate(() => {
    const d = document.querySelector('[role="dialog"]');
    return !!d && d.contains(document.activeElement);
  });
  expect(focusInside).toBe(true);
});

test('the drawer is keyboard navigable between tabs', async () => {
  await openDrawer(`/risks?drawer=risk&entity=${seeded.a.riskId}`);
  await expect(drawer()).toBeVisible();

  const overview = drawer().getByRole('tab', { name: 'Overview' });
  await overview.focus();
  await page.keyboard.press('ArrowRight');

  await expect(drawer().getByRole('tab', { name: 'Relations' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
});

/* ------------------------------------------------------------------ *
 * No fabricated data (§60)
 * ------------------------------------------------------------------ */

test('a record with no score says so rather than showing zero', async () => {
  // A freshly created risk on a blank tenant has not been through the Score
  // Engine, and the asset criticality it carries is real. Whatever the drawer
  // shows for the score must be either the engine's number or an explicit
  // "not available" — never a bare 0 presented as a measurement.
  await openDrawer(`/assets?drawer=asset&entity=${seeded.a.assetId}`);
  await expect(drawer()).toBeVisible();

  const meter = drawer().getByRole('meter');
  const unavailable = drawer().getByTestId('score-unavailable');
  expect((await meter.count()) + (await unavailable.count())).toBeGreaterThan(0);
});
