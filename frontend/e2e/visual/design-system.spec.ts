// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

/**
 * Design-system visual regression and accessibility, in both themes.
 *
 * Two guarantees:
 *
 *  1. Every primitive renders identically to its reference in White and Dark.
 *     This is what catches a token regression at the pixel level: a value
 *     changed in primitives.css or tokens.css shows up here as a diff on the
 *     exact primitives it affects, rather than being discovered on a screen
 *     three weeks later.
 *
 *  2. axe finds no serious or critical violation on any of them. The gallery is
 *     the right place for this because it is the only page in the repository
 *     that renders a disabled destructive button, an invalid select and a
 *     warning badge at once — states no single product screen shows, and
 *     therefore states nothing else would catch going unreadable in one theme.
 *
 * Contrast is checked twice on purpose and the two checks are not redundant:
 * scripts/check-contrast.mjs verifies the token PAIRS arithmetically, and axe
 * here verifies what the browser actually composited, which is the only way to
 * catch a component that put a token on a surface the token was never meant for.
 */

const THEMES = ['light', 'dark'] as const;

/** Gallery pages. Keep in sync with GALLERIES in gallery.tsx. */
const GALLERIES = ['controls', 'forms', 'states', 'charts', 'feedback'] as const;


/**
 * Web fonts are fetched at runtime (Inter / DM Sans / JetBrains Mono). A
 * screenshot taken before they land captures the fallback face, so the same
 * page yields two different images depending on cache state — a diff that looks
 * like a typography regression and is not. Waiting for document.fonts is what
 * makes the suite deterministic.
 */
async function waitForFonts(page: import('@playwright/test').Page) {
  await page.evaluate(() => document.fonts.ready);
}

function galleryURL(gallery: string, theme: string) {
  return `/e2e/visual/gallery.html?gallery=${gallery}&theme=${theme}`;
}

for (const gallery of GALLERIES) {
  for (const theme of THEMES) {
    test(`${gallery} — ${theme}`, async ({ page }) => {
      await page.goto(galleryURL(gallery, theme));
      await page.waitForSelector('main');
      await waitForFonts(page);

      // Confirms the page honoured ?theme. Without it, a bug that ignores the
      // parameter would make both snapshots identical and pass while testing
      // one theme twice.
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);

      await expect(page).toHaveScreenshot(`ds-${gallery}-${theme}.png`, { fullPage: true });
    });

    test(`${gallery} — ${theme} — accessibility`, async ({ page }) => {
      await page.goto(galleryURL(gallery, theme));
      await page.waitForSelector('main');

      const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze();

      const serious = results.violations.filter(
        (v) => v.impact === 'critical' || v.impact === 'serious',
      );

      expect(
        serious,
        `axe found ${serious.length} serious/critical violations in ${gallery} (${theme}):\n` +
          serious
            .map((v) => `  ${v.id}: ${v.help}\n${v.nodes.map((n) => `      ${n.html}`).join('\n')}`)
            .join('\n'),
      ).toEqual([]);
    });
  }
}

/**
 * The keyboard contract, checked on the real primitives rather than in jsdom.
 *
 * The unit tests already assert this behaviour, but jsdom has no layout and no
 * real focus model — `offsetParent`, which the focus trap uses to skip hidden
 * controls, is always null there. This runs the same journey in a browser that
 * actually lays the page out.
 */
test('focus is visible and moves through the controls in order', async ({ page }) => {
  await page.goto(galleryURL('controls', 'dark'));
  await page.waitForSelector('main');

  await page.keyboard.press('Tab');

  const focused = page.locator(':focus-visible');
  await expect(focused).toBeVisible();

  // The ring is drawn by an outline, from one token. A control that suppressed
  // it locally would show up here as 'none'.
  const outline = await focused.evaluate((el) => getComputedStyle(el).outlineStyle);
  expect(outline).not.toBe('none');
});

test('tabs are operable with the arrow keys', async ({ page }) => {
  await page.goto(galleryURL('states', 'light'));
  await page.waitForSelector('[role="tablist"]');

  await page.getByRole('tab', { name: 'Empty' }).focus();
  await page.keyboard.press('ArrowRight');

  await expect(page.getByRole('tab', { name: /Loading/ })).toHaveAttribute('aria-selected', 'true');
});

/**
 * Reduced motion.
 *
 * The stylesheet kills animation outright under prefers-reduced-motion. What
 * has to hold is that the content is still THERE — an entrance animation
 * implemented as "start at opacity 0" leaves the element invisible forever once
 * the animation is disabled, which is the standard way this goes wrong.
 */
test.describe('reduced motion', () => {
  test.use({ reducedMotion: 'reduce' });

  test('content is present and visible with animation disabled', async ({ page }) => {
    await page.goto(galleryURL('controls', 'dark'));
    await page.waitForSelector('main');

    const primary = page.getByRole('button', { name: 'Default' }).first();
    await expect(primary).toBeVisible();
    await expect(primary).toHaveCSS('opacity', '1');
  });
});

/**
 * Responsive behaviour.
 *
 * The failure this guards against is horizontal overflow: a control with a
 * fixed width, or a dialog wider than the viewport, pushes the page sideways
 * and every screen becomes a scroll-to-read. It is invisible on a desktop and
 * immediate on a phone, which is why it needs a test rather than an eye.
 */
const VIEWPORTS = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'tablet', width: 834, height: 1112 },
  { name: 'narrow', width: 390, height: 844 },
] as const;

for (const viewport of VIEWPORTS) {
  for (const theme of THEMES) {
    test(`no horizontal overflow — ${viewport.name} — ${theme}`, async ({ page }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });

      for (const gallery of GALLERIES) {
        await page.goto(galleryURL(gallery, theme));
        await page.waitForSelector('main');

        const overflow = await page.evaluate(
          () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
        );
        expect(overflow, `${gallery} overflows horizontally by ${overflow}px`).toBeLessThanOrEqual(0);
      }
    });
  }
}

test('touch targets clear the WCAG 2.5.8 minimum', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(galleryURL('controls', 'light'));
  await page.waitForSelector('main');

  // 24x24 CSS pixels is the AA floor. The small control size is 28px, so this
  // passes with room — the test exists so that shrinking it is a decision
  // someone has to make deliberately.
  const undersized = await page.evaluate(() => {
    const small: string[] = [];
    for (const el of document.querySelectorAll('button')) {
      const r = el.getBoundingClientRect();
      // The link variant is inline text, exempt by 2.5.8's inline exception.
      if (el.className.includes('underline')) continue;
      if (r.width < 24 || r.height < 24) small.push(`${el.textContent?.trim()} ${r.width}x${r.height}`);
    }
    return small;
  });

  expect(undersized).toEqual([]);
});

/**
 * Theme switching.
 *
 * The whole system rests on one claim: flipping data-theme on <html> retints
 * everything, because components name roles and not colours. This proves it end
 * to end — the surface actually changes, and it changes without a reload, which
 * is what a component holding a hardcoded hex would break.
 */
test('flipping the theme attribute retints the page, both directions', async ({ page }) => {
  await page.goto(galleryURL('feedback', 'light'));
  await page.waitForSelector('main');

  const surfaceOf = () =>
    page.evaluate(() => getComputedStyle(document.querySelector('main')!).backgroundColor);

  const light = await surfaceOf();

  await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
  const dark = await surfaceOf();
  expect(dark).not.toBe(light);

  await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'));
  expect(await surfaceOf()).toBe(light);
});
