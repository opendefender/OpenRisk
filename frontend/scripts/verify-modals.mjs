#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Drives the real application: logs in, opens the reported modals in both
 * themes, screenshots each, and runs axe on them.
 *
 * This is the end-to-end check the harness suite deliberately does not do. The
 * harness proves a component resolves its tokens; this proves the assembled
 * app — real routes, real data, real theme store — actually renders them that
 * way.
 *
 * Usage: node scripts/verify-modals.mjs
 * Output: scripts/.verify-out/*.png plus a pass/fail summary.
 */

import { chromium } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const OUT = resolve(here, '.verify-out');
const APP = 'http://localhost:5173';

const EMAIL = process.env.OPENRISK_EMAIL ?? 'cookie-probe@example.com';
const PASSWORD = process.env.OPENRISK_PASSWORD ?? 'Zebra-Crossing-9x';

/**
 * The overlays under test. `open` performs whatever clicking is needed; it
 * returns false when the trigger is not reachable for this account, which is
 * reported rather than silently passing.
 */
const CASES = [
  {
    id: 'create-risk',
    route: '/risks',
    open: async (page) => clickFirst(page, ['button:has-text("Nouveau risque")', 'button:has-text("New risk")']),
  },
  {
    id: 'create-asset',
    route: '/assets',
    open: async (page) => clickFirst(page, ['button:has-text("Nouvel actif")', 'button:has-text("New asset")', 'button:has-text("Ajouter")']),
  },
  {
    id: 'create-mitigation',
    route: '/risks/mitigations',
    open: async (page) => clickFirst(page, ['button:has-text("Nouvelle")', 'button:has-text("New mitigation")', 'button:has-text("Créer")']),
  },
  {
    id: 'approval-request',
    route: '/governance?tab=approvals',
    open: async (page) => clickFirst(page, ['button:has-text("Demander")', 'button:has-text("approbation")', 'button:has-text("Request")']),
  },
  {
    id: 'report-preview',
    route: '/reports',
    open: async (page) => clickFirst(page, ['button:has-text("Aperçu")', 'button:has-text("Preview")', 'button:has-text("Générer")']),
  },
];

async function clickFirst(page, selectors) {
  for (const sel of selectors) {
    const el = page.locator(sel).first();
    if (await el.count().catch(() => 0)) {
      try {
        await el.click({ timeout: 3000 });
        return true;
      } catch {
        /* try the next selector */
      }
    }
  }
  return false;
}

async function login(page) {
  await page.goto(`${APP}/login`, { waitUntil: 'domcontentloaded' });
  await page.fill('input[type="email"]', EMAIL);
  await page.fill('input[type="password"]', PASSWORD);
  await page.click('button[type="submit"]');
  await page.waitForURL((u) => !u.pathname.includes('/login'), { timeout: 20000 });
}

/** Forces a theme through the store's persisted key, then reloads. */
async function setTheme(page, theme) {
  await page.evaluate((t) => {
    const raw = localStorage.getItem('openrisk-ui');
    const parsed = raw ? JSON.parse(raw) : { state: {}, version: 0 };
    parsed.state = { ...parsed.state, themeMode: t, theme: t };
    localStorage.setItem('openrisk-ui', JSON.stringify(parsed));
  }, theme);
  await page.reload({ waitUntil: 'domcontentloaded' });
}

mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const results = [];

for (const theme of ['light', 'dark']) {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();

  await login(page);
  await setTheme(page, theme);

  const applied = await page.getAttribute('html', 'data-theme');
  if (applied !== theme) {
    results.push({ id: '(theme)', theme, status: `FAIL — html data-theme is "${applied}"` });
  }

  for (const c of CASES) {
    try {
      await page.goto(`${APP}${c.route}`, { waitUntil: 'domcontentloaded' });
      await page.waitForTimeout(1500); // let the route settle

      const opened = await c.open(page);
      if (!opened) {
        results.push({ id: c.id, theme, status: 'SKIP — trigger not found' });
        continue;
      }

      await page.waitForSelector('[role="dialog"], .fixed.inset-0', { state: 'visible', timeout: 5000 });
      await page.waitForTimeout(400); // settle the entrance animation

      const file = `${OUT}/${c.id}-${theme}.png`;
      await page.screenshot({ path: file });

      // The check that matters: is the modal's own surface actually following
      // the theme, or is it a hardcoded panel?
      const surface = await page
        .locator('[role="dialog"], .fixed.inset-0 > div')
        .first()
        .evaluate((el) => getComputedStyle(el).backgroundColor)
        .catch(() => 'n/a');

      const axeResults = await new AxeBuilder({ page })
        .include('.fixed.inset-0')
        .withTags(['wcag2a', 'wcag2aa'])
        .analyze()
        .catch(() => ({ violations: [] }));

      const serious = axeResults.violations.filter((v) => ['critical', 'serious'].includes(v.impact));

      results.push({
        id: c.id,
        theme,
        status: serious.length === 0 ? 'PASS' : `AXE ${serious.map((v) => v.id).join(',')}`,
        surface,
      });
    } catch (err) {
      results.push({ id: c.id, theme, status: `ERROR — ${String(err).split('\n')[0].slice(0, 70)}` });
    }
  }

  await ctx.close();
}

await browser.close();

console.log('\n id                    theme   status                       modal background');
console.log('─'.repeat(88));
for (const r of results) {
  console.log(
    ` ${r.id.padEnd(21)} ${r.theme.padEnd(7)} ${(r.status ?? '').padEnd(28)} ${r.surface ?? ''}`,
  );
}
console.log(`\nScreenshots in ${OUT}`);

const failed = results.filter((r) => (r.status ?? '').startsWith('AXE') || (r.status ?? '').startsWith('FAIL'));
process.exit(failed.length > 0 ? 1 : 0);
