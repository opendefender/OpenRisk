// Deep link and reload of the asset inventory (#758).
//
// Vite used to emit its bundle into `dist/assets/`, the same path as the
// `/assets` inventory route. Behind nginx (`frontend/nginx.conf`),
// `try_files $uri $uri/` matched that directory: `/assets` was redirected to
// `/assets/` and answered 403, so a reload or a pasted link broke while in-app
// navigation kept working. The fix moves the bundle to `dist/static/`.
//
// What this spec can and cannot catch: on the Vite dev server, which is what the
// PR job drives, there is no `dist/` and the collision cannot happen. It guards
// the regression only when E2E_BASE_URL points at a built bundle served by the
// nginx image. The direct-load and reload assertions hold on both.

import { test, expect, type Page, type Response } from '@playwright/test';
import { authFileFor } from './support/env';

test.use({ storageState: authFileFor('admin') });

const INVENTORY_HEADING = /^(Inventaire|Inventory)$/;

async function expectInventory(page: Page, res: Response | null) {
  expect(res, 'navigation produced a response').not.toBeNull();
  expect(res!.status(), 'document status').toBe(200);
  // A redirect to `/assets/` is the first half of the 403, so it fails too.
  expect(new URL(page.url()).pathname, 'no redirect to the static directory').toBe('/assets');
  await expect(page.getByRole('heading', { level: 1, name: INVENTORY_HEADING })).toBeVisible();
}

test('UX-758: /assets loads directly and survives a reload', async ({ page }) => {
  await expectInventory(page, await page.goto('/assets', { waitUntil: 'domcontentloaded' }));
  await expectInventory(page, await page.reload({ waitUntil: 'domcontentloaded' }));
});

test('UX-758: a shared /assets link with a query string loads', async ({ page }) => {
  const res = await page.goto('/assets?q=758', { waitUntil: 'domcontentloaded' });
  await expectInventory(page, res);
});
