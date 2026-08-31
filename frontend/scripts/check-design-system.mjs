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
const PRODUCT = [
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
 * Compares values as VALUES, not as strings. Prettier rewrites `0.10` to `0.1`
 * on save, and a formatter disagreeing with an upstream author about a trailing
 * zero is not drift — reporting it as drift trains people to ignore the check,
 * which is the only way a check like this actually fails.
 */
function normalise(value) {
  return value.replace(/\d+\.\d+/g, (n) => String(parseFloat(n)));
}

/** Strips comments, then walks top-level rules into [selector, declarations]. */
function parseRules(css) {
  const clean = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const rules = [];
  let i = 0;
  while (i < clean.length) {
    const open = clean.indexOf('{', i);
    if (open === -1) break;
    const selector = clean.slice(i, open).trim().replace(/\s+/g, ' ');

    // Skip at-rules (@media, @import) wholesale: they nest, and nothing in the
    // token contract declares a value inside one that is not also declared out.
    if (selector.startsWith('@')) {
      let depth = 1;
      let j = open + 1;
      while (j < clean.length && depth > 0) {
        if (clean[j] === '{') depth++;
        else if (clean[j] === '}') depth--;
        j++;
      }
      i = j;
      continue;
    }

    const close = clean.indexOf('}', open);
    if (close === -1) break;
    const vars = {};
    for (const line of clean.slice(open + 1, close).split('\n')) {
      const m = line.match(/^\s*(--[\w-]+)\s*:\s*([^;]+);/);
      if (m) vars[m[1]] = m[2].trim().replace(/\s+/g, ' ');
    }
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
    const have = got[token];

    if (have === undefined) {
      console.log(`MISSING  ${token}  in  ${selector}`);
      console.log(`  canonical: ${want}\n`);
      drift++;
      continue;
    }
    if (normalise(have) === normalise(want)) continue;

    const reason = ALLOWED[selector]?.[token];
    if (reason) {
      allowed++;
      continue;
    }
    console.log(`DRIFT    ${token}  in  ${selector}`);
    console.log(`  canonical: ${want}`);
    console.log(`  product:   ${have}\n`);
    drift++;
  }
}

console.log(
  `${checked} canonical tokens checked, ${allowed} deliberate divergence(s), ${drift} drifting.`,
);

if (drift > 0) {
  console.error(
    '\nDesign-system check FAILED.\n' +
      'Either bring the product value back in line with design-system/openrisk.tokens.css,\n' +
      'or — if the difference is deliberate and this product needs it — record it in\n' +
      'ALLOWED in this file with the reason. Do not edit the vendored copy.',
  );
  process.exit(1);
}
console.log('Product tokens are in sync with the canonical design system.');
