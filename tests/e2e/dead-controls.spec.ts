// Dead controls — the ban, asserted.
//
// One test per entry of docs/ui/dead-controls.md §1. Each asserts an OBSERVABLE
// EFFECT (rule (c)), or, for the two controls the user chose to delete, asserts
// that they are gone and stay gone.

import { test, expect } from '@playwright/test';
import { authFileFor } from './support/env';

test.use({ storageState: authFileFor('admin') });

test.describe('header', () => {
  test('the status dot reports the REAL connection, not a permanent green', async ({ page, context }) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    const dot = page.getByTestId('connection-status');
    await expect(dot).toBeVisible({ timeout: 20_000 });
    await expect(dot).toHaveAttribute('data-state', 'online');

    // Pull the network: the indicator must follow. This is the whole point —
    // the old dot pulsed green here too.
    await context.setOffline(true);
    await expect(dot).toHaveAttribute('data-state', 'offline', { timeout: 10_000 });

    await context.setOffline(false);
    await expect(dot).toHaveAttribute('data-state', 'online', { timeout: 10_000 });
  });

  test('the microphone button is gone (no speech feature exists)', async ({ page }) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await expect(page.getByRole('button', { name: /voice assistant/i })).toHaveCount(0);
  });

  test('the notification panel no longer offers a "view all" that only closes it', async ({ page }) => {
    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await page.getByRole('button', { name: /notification/i }).first().click();
    await expect(page.getByRole('button', { name: /voir toutes les notifications|view all notifications/i })).toHaveCount(0);
  });
});

test.describe('buttons that had a handler but showed nothing', () => {
  test('executive dashboard: Refresh disables, spins, and confirms', async ({ page }) => {
    // /analytics redirects to the dashboard's executive view.
    await page.goto('/?view=executive', { waitUntil: 'domcontentloaded' });

    const refresh = page.getByRole('button', { name: /actualiser|refresh/i });
    await expect(refresh).toBeVisible({ timeout: 20_000 });

    // Slow the endpoint down so the in-flight state is observable at all.
    await page.route('**/api/v1/analytics/executive*', async (route) => {
      await new Promise((r) => setTimeout(r, 1200));
      await route.continue();
    });

    const request = page.waitForRequest('**/api/v1/analytics/executive*');
    await refresh.click();
    await request;
    await expect(page.getByRole('button', { name: /actualisation|refreshing/i })).toBeDisabled();
    // …and it ends with an explicit outcome.
    await expect(page.getByText(/à jour|up to date|impossible|could not refresh/i).first()).toBeVisible({ timeout: 15_000 });
  });

  test('threat intel: Sync shows it is working and reports a result', async ({ page }) => {
    await page.goto('/threat-map', { waitUntil: 'domcontentloaded' });

    await page.route('**/api/v1/cti/sync', async (route) => {
      await new Promise((r) => setTimeout(r, 1200));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ total_vulnerabilities: 7 }),
      });
    });

    const sync = page.getByRole('button', { name: /synchroniser|sync feed/i }).first();
    await expect(sync).toBeVisible({ timeout: 20_000 });
    await sync.click();

    await expect(page.getByRole('button', { name: /synchronisation|syncing/i })).toBeDisabled();
    await expect(page.getByText(/7 CVE|7 CVEs/i).first()).toBeVisible({ timeout: 15_000 });
  });

  test('threat intel: Match assets reports "nothing found" as a result, not silence', async ({ page }) => {
    await page.goto('/threat-map', { waitUntil: 'domcontentloaded' });

    await page.route('**/api/v1/cti/match', async (route) => {
      await new Promise((r) => setTimeout(r, 800));
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ risks_created: 0 }),
      });
    });

    const match = page.getByRole('button', { name: /matcher les actifs|match assets/i }).first();
    await expect(match).toBeVisible({ timeout: 20_000 });
    await match.click();
    await expect(page.getByRole('button', { name: /analyse des actifs|matching assets/i })).toBeDisabled();
    await expect(page.getByText(/aucune nouvelle exposition|no new exposure/i).first()).toBeVisible({ timeout: 15_000 });
  });
});

test.describe('buttons that had no handler at all', () => {
  test('Asset Universe: the funnel is a real criticality filter', async ({ page }) => {
    await page.goto('/assets/universe', { waitUntil: 'domcontentloaded' });

    const count = page.getByTestId('universe-node-count');
    await expect(count).toBeVisible({ timeout: 20_000 });
    const before = (await count.textContent()) ?? '';

    await page.getByTestId('universe-filter').click();
    await expect(page.getByTestId('universe-filter-panel')).toBeVisible();
    await page.getByTestId('universe-filter-critical').click();

    // Observable effect: the node count changes shape ("n / total" once filtered).
    await expect(count).not.toHaveText(before, { timeout: 10_000 });
    await expect(count).toContainText('/');
  });

  test('Settings danger zone: "Delete organization" opens a real impact radiography', async ({ page }) => {
    await page.goto('/settings?tab=danger', { waitUntil: 'domcontentloaded' });

    const del = page.getByTestId('delete-org');
    await expect(del).toBeVisible({ timeout: 20_000 });
    await del.click();

    // The dialog states what is destroyed and offers a safer exit. We stop here
    // on purpose: confirming would delete the E2E tenant.
    await expect(page.getByText(/supprimer l’organisation|delete this organization/i).first()).toBeVisible();
    await expect(page.getByText(/piste d’audit|audit trail/i).first()).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByText(/définitivement l’organisation|permanently delete the organization/i)).toHaveCount(0);
  });

  test('API tokens: the prefix copy control actually copies', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']).catch(() => {});
    await page.goto('/settings?tab=tokens', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('datatable-api-tokens')).toBeVisible({ timeout: 20_000 });

    const copy = page.getByRole('button', { name: /copier le préfixe|copy prefix/i }).first();
    const n = await copy.count();
    test.skip(n === 0, 'no API token seeded in this tenant');

    await copy.click();
    await expect(page.getByText(/préfixe copié|prefix copied/i).first()).toBeVisible({ timeout: 10_000 });
  });
});
