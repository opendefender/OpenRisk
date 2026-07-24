// The ONLY test that authenticates through the UI (all others attach a
// storageState). Uses stable data-testids added in the "add stable test ids"
// commit, with type-based fallbacks so it is resilient either way.

import { test, expect } from '@playwright/test';
import { ADMIN } from './support/env';

// Fresh context — no storageState.
test.use({ storageState: { cookies: [], origins: [] } });

// The UI login mechanism is browser-engine agnostic; asserting it once (chromium)
// keeps auth load — and the backend rate limiter — deterministic. Mobile viewport
// coverage is provided by the rendering suites.
test.beforeEach(({}, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium', 'UI login asserted once, on chromium');
});

const email = () => '[data-testid="login-email"], input[type="email"]';
const password = () => '[data-testid="login-password"], input[type="password"]';
const submit = () => '[data-testid="login-submit"], button[type="submit"]';

test('login: rejects invalid credentials and stays on /login', async ({ page }) => {
  await page.goto('/login');
  await page.locator(email()).first().fill('nobody@openrisk.test');
  await page.locator(password()).first().fill('wrong-password');
  await page.locator(submit()).first().click();

  // Error surfaced (sonner toast) + still unauthenticated.
  await expect(page.getByText(/incorrect email or password/i)).toBeVisible();
  await expect(page).toHaveURL(/\/login/);
});

test('login: valid credentials land in the app with a session', async ({ page }) => {
  await page.goto('/login');
  await page.locator(email()).first().fill(ADMIN.email);
  await page.locator(password()).first().fill(ADMIN.password);
  await page.locator(submit()).first().click();

  // Leaves /login (admin lands on the dashboard).
  await expect(page).not.toHaveURL(/\/login/, { timeout: 15_000 });
  const token = await page.evaluate(() => localStorage.getItem('auth_token'));
  expect(token, 'auth_token persisted after login').toBeTruthy();
});
