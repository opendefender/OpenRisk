// Smoke: walk EVERY route declared in App.tsx + navModel.ts.
// Per route we assert the screen renders content (no white screen) and no
// uncaught pageerror. Console errors and 5xx responses are RECORDED as
// annotations (→ CI route×status table + docs/UX_AUDIT_2026-07.md) but do not
// fail the gate — this session builds the instrument and documents; it fixes no
// product bug. Routes that genuinely white-screen today are quarantined in
// KNOWN_BROKEN with a bug ID (test.fixme) so the suite stays green while the
// breakage stays named.

import { test, expect, type Page, type TestInfo } from '@playwright/test';
import { authFileFor } from './support/env';
import { STATIC_ROUTES, REDIRECT_ROUTES, paramRoutes, readSeedIds, type RouteDef } from './support/routes';

test.use({ storageState: authFileFor('admin') });

// path -> OR-BUG id. Filled after the first run for routes that white-screen or
// crash. Keeps the blocking suite green while the audit records the defect.
const KNOWN_BROKEN: Record<string, string> = {
  // e.g. '/simulations': 'OR-BUG-0xx',
};

interface Diag {
  pageErrors: string[];
  consoleErrors: string[];
  serverErrors: string[]; // "GET /x -> 500"
}

function attachDiagListeners(page: Page): Diag {
  const diag: Diag = { pageErrors: [], consoleErrors: [], serverErrors: [] };
  page.on('pageerror', (e) => diag.pageErrors.push(e.message));
  page.on('console', (m) => {
    if (m.type() === 'error') diag.consoleErrors.push(m.text());
  });
  page.on('response', (r) => {
    if (r.status() >= 500) diag.serverErrors.push(`${r.request().method()} ${new URL(r.url()).pathname} -> ${r.status()}`);
  });
  return diag;
}

async function bodyText(page: Page): Promise<string> {
  return (await page.locator('body').innerText().catch(() => '')).trim();
}

async function recordAndScreenshot(page: Page, route: RouteDef, diag: Diag, info: TestInfo, status: string) {
  const png = await page.screenshot({ fullPage: false }).catch(() => null);
  if (png) await info.attach(`screenshot`, { body: png, contentType: 'image/png' });
  await info.attach('diagnostics.json', {
    body: JSON.stringify({ path: route.path, name: route.name, status, ...diag }, null, 2),
    contentType: 'application/json',
  });
  info.annotations.push({ type: 'route-status', description: `${route.path} | ${route.name} | ${status}` });
  if (diag.serverErrors.length)
    info.annotations.push({ type: 'route-5xx', description: `${route.path} :: ${diag.serverErrors.join(', ')}` });
  if (diag.consoleErrors.length)
    info.annotations.push({ type: 'route-console', description: `${route.path} :: ${diag.consoleErrors.length} console error(s)` });
}

// ---------------------------------------------------------------------------
// Static + parameterised screens: must render content, must not crash.
// ---------------------------------------------------------------------------
function renderTest(route: RouteDef) {
  test(`renders ${route.path} (${route.name})`, async ({ page }, info) => {
    test.fixme(route.path in KNOWN_BROKEN, KNOWN_BROKEN[route.path]);
    const diag = attachDiagListeners(page);
    await page.goto(route.path, { waitUntil: 'domcontentloaded' });
    // Let lazy chunk + first data paint settle without hanging on live polling.
    await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

    const text = await bodyText(page);
    const degraded = diag.serverErrors.length > 0 || diag.consoleErrors.length > 0;
    const status = diag.pageErrors.length > 0 || text.length < 15 ? 'cassé' : degraded ? 'dégradé' : route.soon ? 'placeholder' : 'OK';
    await recordAndScreenshot(page, route, diag, info, status);

    // Hard gate: rendered something + no uncaught crash.
    expect(diag.pageErrors, `uncaught error(s) on ${route.path}:\n${diag.pageErrors.join('\n')}`).toEqual([]);
    expect(text.length, `blank screen on ${route.path}`).toBeGreaterThan(14);
  });
}

test.describe('smoke: static routes', () => {
  for (const route of STATIC_ROUTES) renderTest(route);
});

test.describe('smoke: parameterised routes', () => {
  for (const route of paramRoutes(readSeedIds())) renderTest(route);
});

// ---------------------------------------------------------------------------
// Redirect routes: must land on their destination (not blank).
// ---------------------------------------------------------------------------
test.describe('smoke: redirects', () => {
  for (const route of REDIRECT_ROUTES) {
    test(`redirect ${route.path} -> ${route.redirectsTo}`, async ({ page }, info) => {
      const diag = attachDiagListeners(page);
      await page.goto(route.path, { waitUntil: 'domcontentloaded' });
      // Wait for the client-side redirect to actually fire (it happens after
      // mount, so networkidle alone can race ahead of it).
      await page
        .waitForURL(
          (u) => {
            const p = new URL(u).pathname;
            return route.redirectsTo === '/' ? p === '/' : p.startsWith(route.redirectsTo!);
          },
          { timeout: 15_000 },
        )
        .catch(() => {});
      const dest = new URL(page.url()).pathname;
      await recordAndScreenshot(page, route, diag, info, `redirect->${dest}`);
      // Settings/roles etc. resolve under the destination; '/' is exact.
      const ok = route.redirectsTo === '/' ? dest === '/' : dest.startsWith(route.redirectsTo!);
      expect(ok, `${route.path} landed on ${dest}, expected ${route.redirectsTo}`).toBeTruthy();
    });
  }
});
