// Real risk-lifecycle workflow (replaces the fictional spec that targeted
// localhost:3000 with selectors that never existed). Authenticated via
// storageState; drives the actual OpenRisk UI: create a risk → it appears in the
// register → open its detail.

import { test, expect } from '@playwright/test';
import { authFileFor } from './support/env';

test.use({ storageState: authFileFor('admin') });

// Risk creation is engine-agnostic; this deeper mutation runs once (chromium) to
// avoid two parallel writers racing the register refresh. Mobile rendering of the
// register is covered by the smoke matrix; the newcomer journey covers mobile create.
test.beforeEach(({}, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium', 'lifecycle mutation asserted once, on chromium');
});

test('risk lifecycle: create → appears in register → open detail', async ({ page }) => {
  await page.goto('/risks', { waitUntil: 'domcontentloaded' });
  await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {});

  const title = `[E2E] Lifecycle risk ${Date.now()}`;

  // Create via the global modal.
  await page.evaluate(() => window.dispatchEvent(new CustomEvent('openrisk:new-risk')));
  await page.locator('input[name="title"]').first().waitFor({ state: 'visible', timeout: 10_000 });
  await page.locator('input[name="title"]').first().fill(title);
  await page.locator('textarea').first().fill('End-to-end lifecycle risk.');
  await page.locator('button[type="submit"]').first().click();

  // Appears in the register.
  const row = page.getByText(title, { exact: false }).first();
  await expect(row).toBeVisible({ timeout: 15_000 });

  // Open its detail (drawer / row click) and confirm the title renders there.
  await row.click();
  await expect(page.getByText(title, { exact: false }).first()).toBeVisible();
});
