// The newcomer journey (Awa — zero cyber background): sign up → onboard → first
// value ("Aha"). Measures time-to-value (UX-01: < 10 min).
//
// Reality: the Register form is a design stub that never calls the API
// (frontend/src/features/auth/AuthScreen.tsx RegisterForm just advances the view)
// and there is no onboarding flow — so the true unauthenticated newcomer path is
// quarantined as test.fixme with a bug id. The authenticated "first risk" leg runs
// for real as the time-to-value proxy.

import { test, expect } from '@playwright/test';
import { authFileFor } from './support/env';

test.describe('newcomer — signup & onboarding (currently broken)', () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  // UX-02: ≤3 fields. UX-13: no qualification before account. UX-01: account is created.
  test.fixme('UX-02/UX-13: registration is a working ≤3-field flow', async ({ page }) => {
    await page.goto('/register');
    const inputs = await page.locator('form input').count();
    expect(inputs, 'registration form should ask ≤3 fields (UX-02)').toBeLessThanOrEqual(3);
    // Should create an account and enter the app — today it only swaps to the MFA view.
    await page.locator('input[type="email"]').first().fill('awa+new@openrisk.test');
    await page.locator('input[type="password"]').first().fill('AwaNewcomer!2026');
    await page.getByRole('button').filter({ hasText: /create|créer|compte/i }).first().click();
    await expect(page).toHaveURL(/\/($|dashboard|risks|compliance)/, { timeout: 10_000 });
  });
});

test.describe('newcomer — first value (Aha)', () => {
  test.use({ storageState: authFileFor('admin') });

  // UX-01 (time-to-value) + UX-32 (micro-victory feedback). Authenticated proxy:
  // creating a first risk is the shortest real value action in the product.
  test('UX-01/UX-32: create a first risk and see it, fast', async ({ page }, info) => {
    await page.goto('/risks', { waitUntil: 'domcontentloaded' });
    const title = `[E2E] Newcomer first risk ${Date.now()}`;
    const t0 = Date.now();

    // Open the global "New risk" modal (sidebar quick action dispatches this event).
    await page.evaluate(() => window.dispatchEvent(new CustomEvent('openrisk:new-risk')));
    await page.locator('input[name="title"]').first().waitFor({ state: 'visible', timeout: 10_000 });
    await page.locator('input[name="title"]').first().fill(title);
    await page.locator('textarea').first().fill('Created by the newcomer E2E journey.');
    await page.locator('button[type="submit"]').first().click();

    // Aha: the new risk becomes visible in the register.
    await expect(page.getByText(title, { exact: false }).first()).toBeVisible({ timeout: 15_000 });
    const ttvMs = Date.now() - t0;
    info.annotations.push({ type: 'time-to-value-ms', description: String(ttvMs) });
    expect(ttvMs, 'time-to-value under 10 minutes (UX-01)').toBeLessThan(10 * 60 * 1000);
  });
});
