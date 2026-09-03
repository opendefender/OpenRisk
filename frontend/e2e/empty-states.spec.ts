// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Zero-state truthfulness sweep.
 *
 * Creates a genuinely blank tenant over the API, logs into it, and walks every
 * screen asserting two things:
 *
 *   1. the screen renders an <EmptyState>, and
 *   2. nothing on it reports a non-zero quantity.
 *
 * (2) is the part that matters. Assertion (1) alone would have passed against
 * the fabricated build this suite was written for: the dashboard did render
 * empty states in places, while simultaneously showing a fully populated
 * probability x impact matrix, four invented notifications and a leaderboard of
 * seven colleagues who did not exist. A test that only looks for an empty state
 * cannot see the fiction next to it, so this one reads the numbers.
 *
 * Requires a running stack (backend + `vite dev`). Skipped automatically when
 * the API is unreachable, so it does not turn a missing dev environment into a
 * red suite.
 */

import { test, expect, type Page, type Locator } from '@playwright/test';

const API = process.env.E2E_API_URL ?? 'http://localhost:8080/api/v1';

/** A fresh tenant per run: POST /auth/register creates the org, the root
 *  membership and the user in one call, so nothing is inherited from a previous
 *  run or from the demo seeder. */
const stamp = Date.now();
const TENANT = {
  email: `zero-state-${stamp}@openrisk.test`,
  username: `zerostate${stamp}`,
  password: 'ZeroState!Tenant2026',
  full_name: 'Zero State Probe',
  company_name: `Zero State Co ${stamp}`,
};

/* ------------------------------------------------------------------ *
 * The 14 locations named in the spec.
 * ------------------------------------------------------------------ */

interface Location {
  name: string;
  path: string;
  /** Optional in-page tab to click before asserting. */
  tab?: string;
  /** Some screens legitimately show a figure that is zero — "0 risques" is a
   *  true statement. Digits are only forbidden where a non-zero count would be
   *  a fabrication; see assertNoFabricatedCounts. */
  skipDigitScan?: boolean;
}

const LOCATIONS: Location[] = [
  { name: 'dashboard', path: '/' },
  { name: 'risk register', path: '/risks' },
  { name: 'mitigations', path: '/mitigations' },
  { name: 'assets', path: '/assets' },
  { name: 'vulnerabilities', path: '/vulnerabilities' },
  { name: 'threat intel', path: '/threat-map' },
  { name: 'incidents', path: '/incidents' },
  { name: 'compliance', path: '/compliance' },
  { name: 'reports', path: '/reports' },
  { name: 'automation rules', path: '/automation' },
  { name: 'automation SLA', path: '/automation?tab=sla' },
  { name: 'automation executions', path: '/automation?tab=executions' },
  { name: 'audit trail', path: '/governance' },
];

/** Strings that only ever appeared in the deleted fixtures. If any of these is
 *  ever on screen again, a fixture has come back. */
const FIXTURE_TELLS = [
  'srv-paie-01',
  'gw-bank-02',
  'INC-2026-014',
  'Fatou Sy',
  'Amir Diallo',
  'Kofi Mensah',
  'Léa Traoré',
  'Yasmine Ba',
  'Omar Sylla',
  'Nadia Kone',
  'db-core-01',
  'iot-badge',
  'ci-runner',
  'backup-nas',
  'aws-prod',
  'Synthèse exécutive — Juin 2026',
];

/* ------------------------------------------------------------------ *
 * Setup
 * ------------------------------------------------------------------ */

async function apiReachable(): Promise<boolean> {
  try {
    const res = await fetch(`${API}/health`, { signal: AbortSignal.timeout(3000) });
    return res.ok;
  } catch {
    return false;
  }
}

// Serial, on one shared page. The auth endpoints are rate limited (15 requests
// per 5 minutes per IP), so registering and logging in per test would trip the
// limiter partway through the sweep and report it as a product failure.
test.describe.configure({ mode: 'serial' });

let page: Page;

test.beforeAll(async ({ browser }) => {
  test.skip(
    !(await apiReachable()),
    `API unreachable at ${API} — start the stack to run this suite`,
  );

  const res = await fetch(`${API}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(TENANT),
  });
  expect(res.status, `register a blank tenant: ${await res.text()}`).toBe(201);

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

/** Navigates the shared session to a route and waits for its data to settle. */
async function visit(path: string) {
  await page.goto(path);
  await page.waitForLoadState('networkidle');
}

/* ------------------------------------------------------------------ *
 * Assertions
 * ------------------------------------------------------------------ */

/**
 * Fails if any rendered metric shows a non-zero number.
 *
 * Reads the elements that carry counts — the `mono` figures used by every KPI
 * card, gauge and score in the design system — rather than scraping the whole
 * page, because page text legitimately contains digits (dates, version strings,
 * "ISO 27001", "5x5", pagination). A metric that is genuinely zero renders "0",
 * "0 %", "—" or "0 FCFA"; anything else on a blank tenant is invented.
 */
async function assertNoNonZeroMetrics(page: Page, where: string) {
  const figures = page.locator('main .mono');
  const count = await figures.count();
  for (let i = 0; i < count; i++) {
    const el = figures.nth(i);
    if (!(await el.isVisible())) continue;
    const text = ((await el.textContent()) ?? '').trim();
    if (!text) continue;
    // Strip thousands separators (space, NBSP, narrow NBSP, comma) so "1 200"
    // is read as 1200 rather than as 1 and 200.
    const digits = text.replace(/[\s  ,]/g, '');

    // "N/M" ratios — "0/4 étapes", "0/12 risques chiffrés". Only the numerator
    // is tenant data; the denominator is a total (onboarding steps, registered
    // risks) and may legitimately be non-zero. Reading the numerator is exactly
    // the check that catches the reported "0/1 quantified" defect, where a
    // client-side count over one page disagreed with the register.
    const ratio = digits.match(/^(\d+(?:[.,]\d+)?)\s*\/\s*\d/);
    const numbers = ratio ? [ratio[1]] : (digits.match(/\d+(?:[.,]\d+)?/g) ?? []);
    for (const n of numbers) {
      expect(
        Number(n.replace(',', '.')),
        `${where}: metric "${text}" reports a non-zero value on a blank tenant`,
      ).toBe(0);
    }
  }
}

/** Fails if any string unique to the deleted fixtures is back on screen. */
async function assertNoFixtureTells(page: Page, where: string) {
  const body = (await page.locator('body').textContent()) ?? '';
  for (const tell of FIXTURE_TELLS) {
    expect(body, `${where}: fixture string "${tell}" is rendering again`).not.toContain(tell);
  }
}

/** The empty state must be present, and must carry a title. */
async function assertEmptyState(page: Page, where: string): Promise<Locator> {
  const empty = page.locator('[data-testid="empty-state"]').first();
  await expect(empty, `${where}: no EmptyState rendered`).toBeVisible({ timeout: 15_000 });
  const variant = await empty.getAttribute('data-variant');
  expect(
    ['first-use', 'no-results', 'error', 'no-permission'],
    `${where}: unknown EmptyState variant "${variant}"`,
  ).toContain(variant);
  // A blank tenant is a first-use situation, never an error one. Catching
  // `error` here is how a silently failing endpoint gets noticed.
  expect(variant, `${where}: rendered the error variant on a healthy blank tenant`).not.toBe(
    'error',
  );
  return empty;
}

/* ------------------------------------------------------------------ *
 * The sweep
 * ------------------------------------------------------------------ */

test.describe('a brand-new tenant shows zero everywhere', () => {
  for (const loc of LOCATIONS) {
    test(`${loc.name} renders an empty state and no invented numbers`, async () => {
      await visit(loc.path);
      await assertEmptyState(page, loc.name);
      await assertNoFixtureTells(page, loc.name);
      if (!loc.skipDigitScan) {
        await assertNoNonZeroMetrics(page, loc.name);
      }
    });
  }

  test('notifications bell is unlit and its panel is empty', async () => {
    await visit('/');

    const bell = page.getByRole('button', { name: /notification/i }).first();
    await expect(bell).toBeVisible();

    // The unread dot used to be static markup, permanently lit.
    await expect(
      bell.locator('span'),
      'notifications: the unread dot is lit on a tenant with no notifications',
    ).toHaveCount(0);

    await bell.click();
    await assertEmptyState(page, 'notifications');
    await assertNoFixtureTells(page, 'notifications');
  });

  test('dashboard heatmap is empty rather than pre-filled', async () => {
    await visit('/');

    // The 5x5 matrix was a hardcoded literal summing to 52 risks. On a blank
    // tenant every cell must be empty, so no cell may carry a count.
    const cells = page.locator('main [title*="P"][title*="I"]');
    const n = await cells.count();
    for (let i = 0; i < n; i++) {
      const text = ((await cells.nth(i).textContent()) ?? '').trim();
      expect(text, 'dashboard heatmap: a cell reports risks on a blank tenant').toBe('');
    }
  });

  test('security score reads "not measured", not a perfect 100', async () => {
    await visit('/');
    // The API returns 100 for an empty register; rendering that as a score
    // tells a fresh tenant its posture is flawless.
    const main = page.locator('main');
    await expect(main).not.toContainText('100');
    await expect(main.getByText(/non mesuré|not measured/i).first()).toBeVisible();
  });

  test('no demo banner unless the server is in DEMO_MODE', async () => {
    const health = await (await fetch(`${API}/health`)).json();
    await visit('/');
    const banner = page.getByTestId('demo-banner');
    if (health.demo_mode === true) {
      await expect(banner, 'DEMO_MODE is on but the banner is absent').toBeVisible();
      // Non-dismissible by design: no close control anywhere inside it.
      await expect(banner.getByRole('button')).toHaveCount(0);
    } else {
      await expect(banner, 'demo banner rendered while DEMO_MODE is off').toHaveCount(0);
    }
  });
});
