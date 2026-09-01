// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { test, expect } from '@playwright/test';
import { alphaModifierSites, UTILITIES, renderManifest } from '../../scripts/alpha-modifier-sites.mjs';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

import { ALPHA_CLASSES } from './alpha-classes.generated';

/**
 * Opacity modifiers on design tokens resolve to real colours. #427.
 *
 * The bug: every colour was declared as a bare `var(--token)`, Tailwind could
 * not extract channels from a finished colour, and so `bg-danger/10` emitted
 * NOTHING. The element fell through to whatever was behind it, which is why
 * hundreds of dead classes survived review — an inert tint looks like a
 * deliberate flat surface, and reads in a diff as though styling is handled.
 *
 * So this suite is deliberately not a screenshot suite and not a grep over the
 * stylesheet. Both would have passed while the bug was live: the screenshot
 * would have recorded the untinted surface as the reference, and the class name
 * IS present in the markup either way. The only question that distinguishes a
 * working tint from an inert one is what the browser computed, so that is what
 * is asserted — getComputedStyle, in Chromium, on the real stylesheet.
 *
 * Two guarantees:
 *
 *  1. Every class in the manifest computes to a real colour whose alpha is the
 *     one the class asked for, in both themes.
 *  2. The manifest still matches the source tree. A new `bg-<token>/<alpha>`
 *     anywhere under src/ fails CI until it is regenerated, so coverage cannot
 *     quietly fall behind the codebase the way the original 284 sites did.
 */

const here = dirname(fileURLToPath(import.meta.url));
const MANIFEST = resolve(here, 'alpha-classes.generated.ts');

const THEMES = ['light', 'dark'] as const;

function harnessURL(theme: string) {
  return `/e2e/visual/alpha.html?theme=${theme}`;
}

/** `bg-surface-1/50` -> { prefix: 'bg', token: 'surface-1', alpha: 50 }. */
function parse(cls: string) {
  const slash = cls.lastIndexOf('/');
  const alpha = Number(cls.slice(slash + 1));
  const head = cls.slice(0, slash);
  const prefix = Object.keys(UTILITIES)
    .sort((a, b) => b.length - a.length)
    .find((p) => head.startsWith(`${p}-`))!;
  return { prefix, token: head.slice(prefix.length + 1), alpha };
}

/**
 * Reads back what the browser computed for one class.
 *
 * The element is given a border width and a child because some of these
 * utilities have nothing to colour otherwise: `border-*` computes to the
 * initial colour on a zero-width border, and `divide-*` colours the gap between
 * children rather than the element itself.
 */
async function computed(page: import('@playwright/test').Page, cls: string) {
  const { prefix } = parse(cls);
  const spec = UTILITIES[prefix as keyof typeof UTILITIES];

  return page.evaluate(
    ({ cls, spec }) => {
      const el = document.createElement('div');
      el.className = cls;
      el.style.borderStyle = 'solid';
      el.style.borderWidth = '2px';
      el.style.width = '20px';
      el.style.height = '20px';

      const a = document.createElement('span');
      const b = document.createElement('span');
      a.style.borderStyle = 'solid';
      a.style.borderWidth = '2px';
      el.append(a, b);
      document.body.appendChild(el);

      const read = () => {
        if ('childProperty' in spec) {
          return getComputedStyle(a).getPropertyValue(spec.childProperty as string);
        }
        if ('variable' in spec) {
          return getComputedStyle(el).getPropertyValue(spec.variable as string);
        }
        return getComputedStyle(el).getPropertyValue(spec.property as string);
      };

      const value = read();
      el.remove();
      return value.trim();
    },
    { cls, spec },
  );
}

/**
 * The alpha the browser actually applied.
 *
 * Chromium resolves a `color-mix(... N%, transparent)` against a token to
 * `oklab(L a b / 0.N)`, so the alpha is readable straight off the value. Where
 * Tailwind parks the colour in an UNREGISTERED custom property instead
 * (`--tw-shadow-color`, the gradient stops on some builds) there is nothing to
 * resolve it against and the value stays as the literal `color-mix(...)` text —
 * still proof the modifier produced a real declaration, so the percentage is
 * read from the text in that case.
 */
function alphaOf(value: string): number | null {
  const resolved = value.match(/\/\s*([0-9.]+)\s*\)/);
  if (resolved) return Number(resolved[1]);
  const mix = value.match(/([0-9.]+)%\s*,\s*transparent/);
  if (mix) return Number(mix[1]) / 100;
  return null;
}

test.describe('opacity modifiers on design tokens (#427)', () => {
  for (const theme of THEMES) {
    test(`every token opacity modifier resolves — ${theme}`, async ({ page }) => {
      await page.goto(harnessURL(theme));
      await page.waitForSelector('#alpha-root[data-ready]', { state: 'attached' });
      await expect(page.locator('html')).toHaveAttribute('data-theme', theme);

      const failures: string[] = [];

      for (const cls of ALPHA_CLASSES) {
        const { alpha } = parse(cls);
        const value = await computed(page, cls);

        // The exact symptom reported in #427: the rule does not exist, so the
        // property is either unset or fully transparent.
        if (!value || value === 'rgba(0, 0, 0, 0)' || value === 'transparent') {
          failures.push(`${cls}: emitted no colour (computed ${JSON.stringify(value)})`);
          continue;
        }

        const got = alphaOf(value);
        if (got === null) {
          failures.push(`${cls}: computed ${value} — no alpha channel, modifier was dropped`);
          continue;
        }
        if (Math.abs(got - alpha / 100) > 0.01) {
          failures.push(`${cls}: wanted alpha ${alpha / 100}, browser computed ${got} (${value})`);
        }
      }

      expect(
        failures,
        `${failures.length} of ${ALPHA_CLASSES.length} opacity modifiers do not render in ${theme}:\n` +
          failures.map((f) => `  ${f}`).join('\n'),
      ).toEqual([]);
    });
  }

  /**
   * A tint is only meaningfully different from no tint if it differs from the
   * surface behind it AND from the flat token. Without this, a regression that
   * made every modifier resolve to the opaque token would satisfy the test
   * above — the value would be a real colour, just the wrong one.
   */
  test('a tint is not the flat token', async ({ page }) => {
    await page.goto(harnessURL('dark'));
    await page.waitForSelector('#alpha-root[data-ready]', { state: 'attached' });

    const sample = ALPHA_CLASSES.filter((c) => c.startsWith('bg-')).slice(0, 12);
    for (const cls of sample) {
      const { token } = parse(cls);
      const tinted = await computed(page, cls);
      const flat = await computed(page, `bg-${token}`);
      expect(tinted, `${cls} computed the same as the flat bg-${token}`).not.toBe(flat);
    }
  });

  /**
   * Coverage guard. The generated manifest is what Tailwind reads to emit these
   * classes bare, so a stale manifest silently narrows the suite rather than
   * failing it — exactly how 284 dead call sites accumulated unnoticed.
   */
  test('the generated manifest matches the source tree', async () => {
    const onDisk = readFileSync(MANIFEST, 'utf8');
    const expected = renderManifest(alphaModifierSites());

    expect(
      onDisk,
      'e2e/visual/alpha-classes.generated.ts is stale.\n' +
        'Run: npm run generate:alpha-classes',
    ).toBe(expected);
  });
});
