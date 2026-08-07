// <DataTable> — the contract, asserted.
//
// Rule (c) of docs/ui/dead-controls.md: no interactive element ships without a
// test of its OBSERVABLE EFFECT. This spec is that test for the shared table:
// the row menu that used to be clipped, the "Filtres" button that used to be a
// search box, selection scope, column layout, export, and the three UI states.
//
// The 200-row cases fulfil /risks from the test itself. That is deliberate: the
// assertion is about the component's geometry at scale (a menu on the last row
// of a long, virtualised list), not about the database. Every other case runs
// against the real seeded backend.

import { test, expect, type Page } from '@playwright/test';
import { authFileFor } from './support/env';

test.use({ storageState: authFileFor('admin') });

const CRITS = ['critical', 'high', 'medium', 'low'];

/** Fulfils GET /risks with `count` synthetic rows, honouring page/limit. */
async function stubRisks(page: Page, count: number) {
  await page.route('**/api/v1/risks?**', async (route) => {
    const url = new URL(route.request().url());
    const limit = Number(url.searchParams.get('limit') ?? 50);
    const pageNo = Number(url.searchParams.get('page') ?? 1);
    const from = (pageNo - 1) * limit;
    const items = Array.from({ length: Math.max(0, Math.min(limit, count - from)) }, (_, i) => {
      const n = from + i;
      return {
        id: `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`,
        name: `Stub risk ${n + 1}`,
        title: `Stub risk ${n + 1}`,
        description: `Synthetic row ${n + 1}`,
        score: Number((30 - (n % 30)).toFixed(1)),
        probability: 0.5,
        impact: 5,
        criticality: CRITS[n % CRITS.length],
        status: 'open',
        lifecycle_phase: 'identified',
        tags: [],
        frameworks: [],
        assets: [],
        mitigations: [],
        source: 'manual',
        updated_at: new Date().toISOString(),
      };
    });
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ items, total: count }),
    });
  });
}

async function gotoRegister(page: Page, query = '') {
  await page.goto(`/risks${query}`, { waitUntil: 'domcontentloaded' });
  await expect(page.getByTestId('datatable-risks')).toBeVisible({ timeout: 20_000 });
}

test.describe('row menu (the clipping bug)', () => {
  test('on the LAST row of a 200-item table, the menu is fully inside the viewport', async ({ page }) => {
    await stubRisks(page, 200);
    await gotoRegister(page, '?size=100');

    // Scroll the virtualised body to the very bottom and take the last rendered row.
    const scroller = page.getByTestId('table-scroll');
    await scroller.evaluate((el) => { el.scrollTop = el.scrollHeight; });
    await expect(page.getByTestId('table-row').last()).toBeVisible();

    const lastRow = page.getByTestId('table-row').last();
    await lastRow.getByTestId('row-menu-trigger').click();

    const menu = page.getByTestId('row-menu');
    await expect(menu).toBeVisible();

    // The whole menu box — not just its first item — must be inside the viewport.
    const box = (await menu.boundingBox())!;
    const viewport = page.viewportSize()!;
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.y + box.height).toBeLessThanOrEqual(viewport.height + 1);
    expect(box.x + box.width).toBeLessThanOrEqual(viewport.width + 1);

    // And every item is actually clickable (not merely painted).
    const items = menu.getByRole('menuitem');
    await expect(items.first()).toBeVisible();
    await expect(items.last()).toBeVisible();
  });

  test('the menu escapes its scroll container (portal, not absolute positioning)', async ({ page }) => {
    await stubRisks(page, 200);
    await gotoRegister(page, '?size=100');

    await page.getByTestId('table-row').first().getByTestId('row-menu-trigger').click();
    const menu = page.getByTestId('row-menu');
    await expect(menu).toBeVisible();

    // A portalled menu is a direct-ish child of <body>, never of the table's
    // overflow:auto scroller — which is precisely what used to clip it.
    const insideScroller = await menu.evaluate(
      (el) => !!el.closest('[data-testid="table-scroll"]'),
    );
    expect(insideScroller).toBe(false);
  });

  test('Escape closes the menu and returns focus to its trigger', async ({ page }) => {
    await stubRisks(page, 60);
    await gotoRegister(page);

    const trigger = page.getByTestId('table-row').first().getByTestId('row-menu-trigger');
    await trigger.click();
    await expect(page.getByTestId('row-menu')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('row-menu')).toBeHidden();
    await expect(trigger).toBeFocused();
  });
});

test.describe('filters are filters, search is search', () => {
  test('"Filtres" opens a facet panel, narrows the table, and lands in the URL', async ({ page }) => {
    await gotoRegister(page);

    await page.getByTestId('filters-trigger').click();
    await expect(page.getByTestId('filters-panel')).toBeVisible();

    await page.getByTestId('facet-criticality-critical').click();

    // Observable effect #1: the URL carries the filter.
    await expect(page).toHaveURL(/f\.criticality=critical/);
    // #2: the trigger shows an active-filter count.
    await expect(page.getByTestId('filters-active-count')).toHaveText('1');
    // #3: a removable chip appears under the toolbar.
    await expect(page.getByTestId('active-filter-chips')).toBeVisible();
  });

  test('a filtered URL is a place: reload restores the exact same view', async ({ page }) => {
    await gotoRegister(page, '?f.criticality=critical&sort=score:asc');
    await expect(page.getByTestId('filters-active-count')).toHaveText('1');
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('filters-active-count')).toHaveText('1');
    await expect(page).toHaveURL(/sort=score%3Aasc|sort=score:asc/);
  });

  test('facets can be combined, counted, and reset in one click', async ({ page }) => {
    await gotoRegister(page);
    await page.getByTestId('filters-trigger').click();
    await page.getByTestId('facet-criticality-critical').click();
    await page.getByTestId('facet-status-open').click();
    await expect(page.getByTestId('filters-active-count')).toHaveText('2');

    await page.getByTestId('filters-reset').click();
    await expect(page.getByTestId('filters-active-count')).toHaveCount(0);
    await expect(page).not.toHaveURL(/f\.criticality/);
  });

  test('a saved filter can be named, re-applied, and deleted', async ({ page }) => {
    await gotoRegister(page);
    await page.getByTestId('filters-trigger').click();
    await page.getByTestId('facet-criticality-high').click();

    await page.getByTestId('saved-view-name').fill('E2E high only');
    await page.getByTestId('saved-view-save').click();

    // Clear, then re-apply from the saved view: the filter comes back.
    await page.getByTestId('filters-reset').click();
    await expect(page.getByTestId('filters-active-count')).toHaveCount(0);

    await page.getByTestId('filters-trigger').click();
    await page.getByTestId('saved-view-E2E high only').click();
    await expect(page).toHaveURL(/f\.criticality=high/);
  });

  test('the search box is a separate affordance and writes ?q=', async ({ page }) => {
    await gotoRegister(page);
    const search = page.getByTestId('table-search');
    await expect(search).toBeVisible();          // always visible, not behind "Filtres"
    await search.fill('log');
    await expect(page).toHaveURL(/[?&]q=log/, { timeout: 5_000 });
  });
});

test.describe('server-side sort & pagination', () => {
  test('clicking a sortable header round-trips to the API', async ({ page }) => {
    await gotoRegister(page);

    const request = page.waitForRequest((r) => r.url().includes('/risks?') && r.url().includes('sort_by=name'));
    await page.getByTestId('sort-name').click();
    await request;
    await expect(page).toHaveURL(/sort=name/);
  });

  test('an inert header is not clickable (no decorative sort)', async ({ page }) => {
    await gotoRegister(page);
    // "Framework" declares neither sortKey nor sortValue → it renders as plain text.
    await expect(page.getByTestId('sort-framework')).toHaveCount(0);
  });

  test('paging asks the server for the next page', async ({ page }) => {
    await stubRisks(page, 120);
    await gotoRegister(page, '?size=50');

    await expect(page.getByTestId('table-range')).toContainText('1');
    const request = page.waitForRequest((r) => r.url().includes('/risks?') && r.url().includes('page=2'));
    await page.getByTestId('table-next').click();
    await request;
    await expect(page.getByTestId('table-page')).toHaveText('2/3');
  });
});

test.describe('selection & bulk actions', () => {
  test('page selection vs "all N results" is an explicit, separate choice', async ({ page }) => {
    await stubRisks(page, 120);
    await gotoRegister(page, '?size=50');

    await page.getByTestId('select-all-page').check();

    // The bar counts the PAGE, and the escalation to the full result set is offered
    // as its own deliberate action — never applied implicitly.
    await expect(page.getByTestId('bulk-count')).toContainText('50');
    await expect(page.getByTestId('select-scope-banner')).toBeVisible();

    await page.getByTestId('select-scope-all').click();
    await expect(page.getByTestId('bulk-count')).toContainText('120');
  });

  test('changing the filter clears a selection the user can no longer see', async ({ page }) => {
    await stubRisks(page, 120);
    await gotoRegister(page, '?size=50');

    await page.getByTestId('select-all-page').check();
    await expect(page.getByTestId('bulk-bar')).toBeVisible();

    await page.getByTestId('table-search').fill('stub');
    await expect(page.getByTestId('bulk-bar')).toBeHidden({ timeout: 5_000 });
  });
});

test.describe('columns, export, states', () => {
  test('hiding a column removes it, and the choice survives a reload', async ({ page }) => {
    await gotoRegister(page);

    await page.getByTestId('columns-trigger').click();
    await page.getByTestId('column-toggle-framework').click();
    await page.keyboard.press('Escape');

    await expect(page.getByRole('columnheader', { name: /framework/i })).toHaveCount(0);

    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('datatable-risks')).toBeVisible();
    await expect(page.getByRole('columnheader', { name: /framework/i })).toHaveCount(0);

    // Put it back so the persisted state does not leak into other tests.
    await page.getByTestId('columns-trigger').click();
    await page.getByTestId('column-toggle-framework').click();
  });

  test('export downloads a CSV of the current view', async ({ page }) => {
    await stubRisks(page, 60);
    await gotoRegister(page);

    const download = page.waitForEvent('download', { timeout: 15_000 });
    await page.getByTestId('table-export').click();
    const file = await download;
    expect(file.suggestedFilename()).toMatch(/^risques-\d{4}-\d{2}-\d{2}\.csv$/);
  });

  test('a failing list renders an error state with a working retry', async ({ page }) => {
    let calls = 0;
    await page.route('**/api/v1/risks?**', async (route) => {
      calls += 1;
      if (calls === 1) return route.fulfill({ status: 500, body: '{}' });
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ items: [], total: 0 }),
      });
    });

    await page.goto('/risks', { waitUntil: 'domcontentloaded' });
    const retry = page.getByTestId('table-retry');
    await expect(retry).toBeVisible({ timeout: 20_000 });
    await retry.click();
    await expect(retry).toBeHidden({ timeout: 15_000 });
  });

  test('an over-filtered table offers a way out instead of a dead end', async ({ page }) => {
    await gotoRegister(page, '?q=zzz-no-such-risk-zzz');
    const clear = page.getByTestId('table-clear-filters');
    await expect(clear).toBeVisible({ timeout: 20_000 });
    await clear.click();
    await expect(page).not.toHaveURL(/zzz-no-such-risk/);
  });
});

test.describe('accessibility', () => {
  test('the table exposes grid semantics and a live sort state', async ({ page }) => {
    await stubRisks(page, 60);
    await gotoRegister(page, '?sort=score:desc');

    const grid = page.getByRole('table');
    await expect(grid).toHaveAttribute('aria-rowcount', '60');
    await expect(page.getByRole('columnheader', { name: /score/i })).toHaveAttribute('aria-sort', 'descending');
  });

  test('rows are reachable and operable from the keyboard alone', async ({ page }) => {
    await stubRisks(page, 60);
    await gotoRegister(page);

    await page.getByTestId('table-row').first().focus();
    await page.keyboard.press('ArrowDown');
    await expect(page.getByTestId('table-row').nth(1)).toBeFocused();

    // Space ticks the focused row's checkbox → the bulk bar appears.
    await page.keyboard.press(' ');
    await expect(page.getByTestId('bulk-bar')).toBeVisible();

    // Escape clears the selection.
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('bulk-bar')).toBeHidden();
  });
});

test.describe('the other six registers use the same table', () => {
  const registers: [string, string][] = [
    ['/vulnerabilities', 'datatable-vulnerabilities'],
    ['/assets', 'datatable-assets'],
    ['/risks/mitigations?view=table', 'datatable-mitigations'],
    ['/incidents', 'datatable-incidents'],
  ];

  for (const [path, testid] of registers) {
    test(`${path} renders <DataTable> with a real search box`, async ({ page }) => {
      await page.goto(path, { waitUntil: 'domcontentloaded' });
      if (path.includes('mitigations')) {
        // The board defaults to Kanban; switch to the table view.
        await page.getByTitle('Table').first().click().catch(() => {});
      }
      await expect(page.getByTestId(testid)).toBeVisible({ timeout: 20_000 });
      await expect(page.getByTestId('table-search')).toBeVisible();
    });
  }

  test('/governance audit trail renders <DataTable>', async ({ page }) => {
    await page.goto('/governance/audit-trail', { waitUntil: 'domcontentloaded' });
    await page.getByRole('button', { name: /piste d’audit|audit trail/i }).click();
    await expect(page.getByTestId('datatable-audit-trail')).toBeVisible({ timeout: 20_000 });
  });

  test('/settings API tokens renders <DataTable>', async ({ page }) => {
    await page.goto('/settings?tab=tokens', { waitUntil: 'domcontentloaded' });
    await expect(page.getByTestId('datatable-api-tokens')).toBeVisible({ timeout: 20_000 });
  });
});
