#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Inventory of every Tailwind opacity-modifier class the product uses on a
 * DESIGN TOKEN colour — `bg-danger/10`, `border-border-strong/20`, and so on.
 *
 * This exists so e2e/visual/alpha-modifiers.spec.ts can be driven by the real
 * call sites instead of a list someone typed once. #427 is a bug where these
 * classes emitted no CSS at all and the page still looked plausible, so the
 * only guard worth having is one that grows with the codebase: a new
 * `bg-<token>/<alpha>` anywhere under src/ becomes a new assertion by itself.
 *
 * Tailwind's own palette (`bg-purple-500/10`, `bg-black/50`) is deliberately
 * NOT inventoried. Those never broke — they are literal colours the framework
 * ships and can already alpha-blend — and including them would pad the suite
 * with assertions about upstream rather than about our token layer.
 *
 * Usage: node scripts/alpha-modifier-sites.mjs   (prints the inventory)
 */

import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve, join, relative } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const SRC = resolve(here, '../src');
const THEME = resolve(here, '../src/styles/theme.css');

/**
 * Utilities whose alpha modifier we can actually read back from the browser,
 * mapped to the thing to read. A `property` is a real CSS property on the
 * element; a `variable` is a custom property Tailwind sets instead, which is
 * still a computed value and still proves the modifier resolved.
 *
 * `divide-*` and the gradient stops are here because they broke the same way;
 * they are just read from a different place.
 */
export const UTILITIES = {
  bg: { property: 'background-color' },
  text: { property: 'color' },
  border: { property: 'border-top-color' },
  'border-t': { property: 'border-top-color' },
  'border-r': { property: 'border-right-color' },
  'border-b': { property: 'border-bottom-color' },
  'border-l': { property: 'border-left-color' },
  'border-x': { property: 'border-left-color' },
  'border-y': { property: 'border-top-color' },
  fill: { property: 'fill' },
  stroke: { property: 'stroke' },
  outline: { property: 'outline-color' },
  ring: { variable: '--tw-ring-color' },
  shadow: { variable: '--tw-shadow-color' },
  from: { variable: '--tw-gradient-from' },
  via: { variable: '--tw-gradient-via' },
  to: { variable: '--tw-gradient-to' },
  // divide-* colours the gap between children, so it is read off a child.
  divide: { childProperty: 'border-top-color' },
};

/** Longest prefix first, so `border-t` wins over `border` on `border-t-primary`. */
const PREFIXES = Object.keys(UTILITIES).sort((a, b) => b.length - a.length);

/** Colour names registered in theme.css's `--color-*` namespace. */
export function tokenNames(themeFile = THEME) {
  const css = readFileSync(themeFile, 'utf8');
  const names = new Set();
  for (const m of css.matchAll(/^\s*--color-([a-z0-9-]+)\s*:/gm)) names.add(m[1]);
  return names;
}

function walk(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (/\.(tsx?|jsx?)$/.test(entry.name)) out.push(full);
  }
  return out;
}

/**
 * Every `<utility>-<token>/<alpha>` in the source tree, deduplicated, with the
 * files that use it. A class is returned only when `<token>` is a registered
 * `--color-*` name — that is what makes it ours rather than Tailwind's.
 */
export function alphaModifierSites({ srcDir = SRC, themeFile = THEME } = {}) {
  const tokens = tokenNames(themeFile);
  const found = new Map();

  for (const file of walk(srcDir)) {
    const text = readFileSync(file, 'utf8');
    for (const m of text.matchAll(/\b([a-z]+(?:-[a-z]+)*)-([a-z0-9-]+)\/(\d{1,3})\b/g)) {
      const [cls] = m;
      const alpha = Number(m[3]);
      if (alpha > 100) continue;

      const prefix = PREFIXES.find(
        (p) => cls.startsWith(`${p}-`) && tokens.has(cls.slice(p.length + 1, cls.lastIndexOf('/'))),
      );
      if (!prefix) continue;

      const token = cls.slice(prefix.length + 1, cls.lastIndexOf('/'));
      const entry = found.get(cls) ?? { cls, prefix, token, alpha, files: new Set() };
      entry.files.add(relative(resolve(srcDir, '..'), file));
      found.set(cls, entry);
    }
  }

  return [...found.values()]
    .map((e) => ({ ...e, files: [...e.files].sort(), uses: e.files.size }))
    .sort((a, b) => b.uses - a.uses || a.cls.localeCompare(b.cls));
}

const MANIFEST = resolve(here, '../e2e/visual/alpha-classes.generated.ts');

/**
 * The manifest exists for one mechanical reason.
 *
 * Most of these classes appear in src only behind a variant — `focus:ring-primary/50`,
 * `hover:bg-surface-1/10`. Tailwind then emits `.focus\:ring-primary\/50:focus` and
 * NOT the bare `.ring-primary/50`, so a test that injects the bare class would find
 * no rule and fail for a reason that has nothing to do with #427.
 *
 * theme.css carries `@source '../../e2e'`, so writing the base class names as
 * literal strings in a file under e2e/ makes Tailwind emit each one bare. The
 * manifest is that file. It is generated, committed, and checked for staleness by
 * the spec — the same shape as the overlay registry guard in overlays.spec.ts.
 */
export function renderManifest(sites) {
  return (
    `// Copyright (c) 2026 OpenDefender Contributors\n` +
    `// SPDX-License-Identifier: AGPL-3.0-only\n\n` +
    `/**\n` +
    ` * GENERATED — do not edit by hand.\n` +
    ` *   npm run generate:alpha-classes\n` +
    ` *\n` +
    ` * Every Tailwind opacity-modifier class the product uses on a design token.\n` +
    ` * Two jobs at once: it is the list e2e/visual/alpha-modifiers.spec.ts asserts\n` +
    ` * over, and — because theme.css scans this directory via \`@source\` — naming\n` +
    ` * each class here is what makes Tailwind emit it BARE, rather than only behind\n` +
    ` * the \`focus:\` / \`hover:\` variant the product happens to use it with.\n` +
    ` *\n` +
    ` * Regenerate when a call site is added or removed; the spec fails if this list\n` +
    ` * and the source tree disagree.\n` +
    ` */\n\n` +
    `export const ALPHA_CLASSES = [\n` +
    sites.map((s) => `  '${s.cls}',`).join('\n') +
    `\n] as const;\n`
  );
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const sites = alphaModifierSites();

  if (process.argv.includes('--write')) {
    writeFileSync(MANIFEST, renderManifest(sites), 'utf8');
    console.log(`wrote ${relative(process.cwd(), MANIFEST)} — ${sites.length} classes`);
  } else {
    for (const s of sites) console.log(String(s.uses).padStart(4), s.cls);
    console.log(
      `\n${sites.length} distinct token opacity-modifier classes, ` +
        `${sites.reduce((n, s) => n + s.uses, 0)} files using them.`,
    );
  }
}
