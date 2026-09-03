#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Fails when the product's design tokens drift from the canonical contract.
 *
 * `design-system/openrisk.tokens.css` is a verbatim copy of the file authored
 * in the opendefender-website repository, which declares itself the single
 * source of truth for both surfaces. The product does not import it — it cannot,
 * because the console carries the azure/iris accent variants, the --select-caret
 * glyph and ~2000 call sites of legacy aliases that have no place in a shared
 * contract. So the copy is compared against instead.
 *
 * The check is one-directional and deliberately narrow: for every token the
 * CANONICAL file declares, the product must declare the same value in the same
 * scope. Tokens the product adds on top are its own business and are ignored.
 * A token the product had to RENAME is followed through the rename and its
 * value still compared — see RENAMED below.
 *
 * The last time these two drifted, the site declared a brand palette on a bare
 * `:root` while the console declared the product palette on
 * `:root[data-theme='dark']`, the brand layer lost the cascade in the default
 * theme, and the two products came apart in dark mode without either side
 * noticing. Nothing caught it because nothing was looking.
 *
 * Usage: npm run check:design-system
 * Exit 0 = in sync, 1 = drift.
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const CANONICAL = resolve(here, '../design-system/openrisk.tokens.css');
// Order is cascade order, because scopesOf() merges with "later wins".
// theme.css comes FIRST even though index.css imports it LAST: its @theme
// blocks land in Tailwind's `@layer theme`, while the other three are imported
// with `layer(base)`. Base outranks theme, so a token declared in both resolves
// to the base value in the browser — and has to resolve to it here too.
const PRODUCT = [
  resolve(here, '../src/styles/theme.css'),
  resolve(here, '../src/styles/primitives.css'),
  resolve(here, '../src/styles/tokens.css'),
  resolve(here, '../src/styles/components.css'),
];

/**
 * Differences that are intentional. Each one needs a reason, and the reason has
 * to be about this product rather than about taste — "we like it darker" is
 * drift with a sentence in front of it.
 */
const ALLOWED = {
  ":root, :root[data-theme='dark']": {
    '--border-control':
      'Canonical #6b6a66 measures 2.87:1 against --surface-3, and --surface-3 is ' +
      'where a control sits while it is hovered. Raised to clear WCAG 1.4.11 on ' +
      'every surface a control can rest on. Asserted by check-contrast.mjs.',
    '--graph-edge':
      'Canonical treats an edge as a decorative dim. In the console a graph edge ' +
      'carries the topology — it is the only thing saying which asset feeds which ' +
      '— so it is held to 3:1 like any other meaningful non-text mark.',
    '--graph-node-stroke':
      'Resolves to --border-control: same role (a structural line that has to be ' +
      'seen), same bar, one value to keep in step.',
  },
  ":root[data-theme='light']": {
    '--border-control': 'See the dark block: 2.92:1 against --surface-3.',
    '--graph-edge': 'See the dark block.',
    '--graph-node-stroke': 'See the dark block.',
  },
};

/**
 * Tokens the product declares under a DIFFERENT NAME than the canonical file.
 * The value contract still holds — the check follows the rename and compares
 * the values — but the name is allowed to diverge, with a reason.
 *
 * This exists because the two surfaces do not run the same Tailwind. A rename
 * forced by the console's build is not drift in the design system; refusing it
 * would mean either shipping a token that silently breaks, or editing the
 * vendored copy, and the second is how the last split started.
 */
const RENAMED = {
  '--text-primary': {
    to: '--fg-primary',
    why:
      'Tailwind v4 (#441) makes --text-* the font-size namespace: theme.css ' +
      'declares --text-sm/--text-lg/--text-display-1 in @theme, where the name ' +
      'both defines the scale and generates the utility. A colour left at ' +
      '--text-primary emits `text-primary { font-size: #f6f4ef }`, which the ' +
      'browser drops in silence. #442.',
  },
  '--text-secondary': { to: '--fg-secondary', why: 'See --text-primary.' },
  '--text-muted': { to: '--fg-muted', why: 'See --text-primary.' },
  '--text-inverse': { to: '--fg-inverse', why: 'See --text-primary.' },
  '--text-on-solid': { to: '--fg-on-solid', why: 'See --text-primary.' },

  // The other half of the same v4 namespace rule, in the opposite direction.
  // A font size only generates its utility if it IS called --text-*, so the
  // canonical --display-1 has to be declared as --text-display-1 for
  // `text-display-1` to exist. Values are compared unchanged.
  '--display-1': {
    to: '--text-display-1',
    why:
      'Tailwind v4 (#441) derives font-size utilities from the --text-* ' +
      'namespace. Declared as --display-1 the token is inert: no ' +
      '`text-display-1` class is generated and the marketing headline falls ' +
      'back to the base size.',
  },
  '--display-2': { to: '--text-display-2', why: 'See --display-1.' },
  '--display-3': { to: '--text-display-3', why: 'See --display-1.' },
  '--display-4': { to: '--text-display-4', why: 'See --display-1.' },
  '--lead': { to: '--text-lead', why: 'See --display-1.' },
};

/**
 * Compares values as VALUES, not as strings. Prettier rewrites `0.10` to `0.1`
 * on save, and a formatter disagreeing with an upstream author about a trailing
 * zero is not drift — reporting it as drift trains people to ignore the check,
 * which is the only way a check like this actually fails.
 */
function normalise(value) {
  return value.replace(/\d+\.\d+/g, (n) => String(parseFloat(n)));
}

/** Pulls `--x: y;` declarations out of a rule body, ignoring nested blocks. */
function declarationsIn(body) {
  const vars = {};
  let depth = 0;
  for (const line of body.split('\n')) {
    if (depth === 0) {
      const m = line.match(/^\s*(--[\w-]+)\s*:\s*([^;]+);/);
      if (m) vars[m[1]] = m[2].trim().replace(/\s+/g, ' ');
    }
    for (const ch of line) {
      if (ch === '{') depth++;
      else if (ch === '}') depth--;
    }
  }
  return vars;
}

/** Index of the `}` closing the block opened at `open`. -1 if unbalanced. */
function closingBrace(css, open) {
  let depth = 1;
  let j = open + 1;
  while (j < css.length) {
    if (css[j] === '{') depth++;
    else if (css[j] === '}' && --depth === 0) return j;
    j++;
  }
  return -1;
}

/** Strips comments, then walks top-level rules into [selector, declarations]. */
function parseRules(css) {
  const clean = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const rules = [];
  let i = 0;
  while (i < clean.length) {
    const open = clean.indexOf('{', i);
    if (open === -1) break;
    // Everything after the last `;` is this block's own prelude. Without the
    // split, a statement at-rule ahead of it (`@source ...;`,
    // `@custom-variant ...;`) is swallowed into the selector and `@theme` is
    // read as `@source` — which is how theme.css stayed unparsed.
    const selector = clean.slice(i, open).split(';').pop().trim().replace(/\s+/g, ' ');

    if (selector.startsWith('@')) {
      const end = closingBrace(clean, open);
      if (end === -1) break;

      // `@theme` and `@theme inline` are read as `:root`, because that is what
      // Tailwind v4 compiles them to. Since #441 they are the ONLY declaration
      // site for 36 contract tokens — the type scale, radii, easings, leading,
      // tracking and font families — so skipping them means shipping those 36
      // unchecked. Every other at-rule (@media, @import, @source, @layer,
      // @utility, @custom-variant) is still skipped: none of them declares a
      // contract token that is not also declared at the top level.
      if (/^@theme\b/.test(selector)) {
        const vars = declarationsIn(clean.slice(open + 1, end));
        if (Object.keys(vars).length) rules.push([':root', vars]);
      }
      i = end + 1;
      continue;
    }

    const close = clean.indexOf('}', open);
    if (close === -1) break;
    const vars = declarationsIn(clean.slice(open + 1, close));
    if (Object.keys(vars).length) rules.push([selector, vars]);
    i = close + 1;
  }
  return rules;
}

/** Merges every block sharing a selector into one scope, later wins. */
function scopesOf(files) {
  const scopes = new Map();
  for (const file of files) {
    for (const [selector, vars] of parseRules(readFileSync(file, 'utf8'))) {
      const target = scopes.get(selector) ?? {};
      Object.assign(target, vars);
      scopes.set(selector, target);
    }
  }
  return scopes;
}

const canonical = scopesOf([CANONICAL]);
const product = scopesOf(PRODUCT);

let drift = 0;
let allowed = 0;
let renamed = 0;
let checked = 0;

for (const [selector, wanted] of canonical) {
  const got = product.get(selector);
  if (!got) {
    console.log(`MISSING SCOPE  ${selector}`);
    console.log('  the product declares no block with this selector at all\n');
    drift++;
    continue;
  }

  for (const [token, want] of Object.entries(wanted)) {
    checked++;
    const rename = RENAMED[token];
    const productToken = rename?.to ?? token;
    const have = got[productToken];

    if (have === undefined) {
      console.log(`MISSING  ${productToken}  in  ${selector}`);
      console.log(`  canonical: ${token}: ${want}\n`);
      drift++;
      continue;
    }
    if (rename) renamed++;
    if (normalise(have) === normalise(want)) continue;

    const reason = ALLOWED[selector]?.[productToken];
    if (reason) {
      allowed++;
      continue;
    }
    console.log(`DRIFT    ${productToken}  in  ${selector}`);
    console.log(`  canonical: ${token}: ${want}`);
    console.log(`  product:   ${have}\n`);
    drift++;
  }
}

console.log(
  `${checked} canonical tokens checked, ${renamed} renamed, ` +
    `${allowed} deliberate divergence(s), ${drift} drifting.`,
);

if (drift > 0) {
  console.error(
    '\nDesign-system check FAILED.\n' +
      'Either bring the product value back in line with design-system/openrisk.tokens.css,\n' +
      'or — if the difference is deliberate and this product needs it — record it in\n' +
      'ALLOWED (a different value) or RENAMED (a different name) in this file with the\n' +
      'reason. Do not edit the vendored copy.',
  );
  process.exit(1);
}
console.log('Product tokens are in sync with the canonical design system.');
