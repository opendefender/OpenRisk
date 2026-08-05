#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Verifies WCAG contrast for every text/surface pair in the design tokens, in
 * both themes.
 *
 * The tokens claim AA compliance; without this the claim is an assertion in a
 * comment. Run in CI so a value cannot be changed to something unreadable
 * without the build failing.
 *
 * Usage: node scripts/check-contrast.mjs
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const TOKENS = resolve(here, '../src/styles/tokens.css');

const AA_NORMAL = 4.5;
const AA_LARGE = 3.0; // >=18.66px bold or >=24px

/** Parses `--name: value;` declarations out of one `:root...{ }` block. */
function parseBlock(css, selector) {
  const start = css.indexOf(selector);
  if (start === -1) throw new Error(`selector not found: ${selector}`);
  const open = css.indexOf('{', start);
  const close = css.indexOf('}', open);
  const body = css.slice(open + 1, close);

  const vars = {};
  for (const line of body.split('\n')) {
    const m = line.match(/^\s*(--[\w-]+)\s*:\s*([^;]+);/);
    if (m) vars[m[1]] = m[2].trim();
  }
  return vars;
}

/** Resolves var() indirection so legacy aliases can be checked too. */
function resolveVar(value, scope, depth = 0) {
  if (depth > 10) return value;
  const m = value.match(/^var\((--[\w-]+)\)$/);
  if (!m) return value;
  const target = scope[m[1]];
  return target ? resolveVar(target, scope, depth + 1) : value;
}

function parseColor(value) {
  const hex = value.match(/^#([0-9a-fA-F]{6})$/);
  if (hex) {
    const n = parseInt(hex[1], 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
  }
  const short = value.match(/^#([0-9a-fA-F]{3})$/);
  if (short) {
    const [r, g, b] = short[1].split('');
    return [parseInt(r + r, 16), parseInt(g + g, 16), parseInt(b + b, 16)];
  }
  const rgba = value.match(/^rgba?\(([^)]+)\)$/);
  if (rgba) {
    const parts = rgba[1].split(',').map((p) => parseFloat(p.trim()));
    return [parts[0], parts[1], parts[2], parts[3] ?? 1];
  }
  return null;
}

/** Composites a translucent colour over an opaque backdrop. */
function flatten(color, backdrop) {
  if (color.length === 3 || color[3] === 1) return color.slice(0, 3);
  const a = color[3];
  return [0, 1, 2].map((i) => color[i] * a + backdrop[i] * (1 - a));
}

function relativeLuminance([r, g, b]) {
  const chan = (v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  };
  return 0.2126 * chan(r) + 0.7152 * chan(g) + 0.0722 * chan(b);
}

function contrast(fg, bg) {
  const l1 = relativeLuminance(fg);
  const l2 = relativeLuminance(bg);
  const [hi, lo] = l1 > l2 ? [l1, l2] : [l2, l1];
  return (hi + 0.05) / (lo + 0.05);
}

/**
 * Pairs that must hold. Text roles are checked against every surface they can
 * legitimately sit on — a token that only passes on one surface is a trap,
 * because nothing stops a component using it on another.
 */
const SURFACES = ['--surface-0', '--surface-1', '--surface-2', '--surface-3', '--surface-sunken'];

const PAIRS = [
  ...['--text-primary', '--text-secondary', '--text-muted'].flatMap((text) =>
    SURFACES.map((surface) => ({ text, surface, min: AA_NORMAL })),
  ),
  // Semantic text on its own tinted surface.
  { text: '--success-text', surface: '--success-surface', min: AA_NORMAL },
  { text: '--warning-text', surface: '--warning-surface', min: AA_NORMAL },
  { text: '--danger-text', surface: '--danger-surface', min: AA_NORMAL },
  { text: '--info-text', surface: '--info-surface', min: AA_NORMAL },
  // Semantic text on the plain raised surface, which is where badges and
  // inline messages actually render most of the time.
  { text: '--success-text', surface: '--surface-2', min: AA_NORMAL },
  { text: '--warning-text', surface: '--surface-2', min: AA_NORMAL },
  { text: '--danger-text', surface: '--surface-2', min: AA_NORMAL },
  { text: '--info-text', surface: '--surface-2', min: AA_NORMAL },
  // Inverse text sits on the accent fill of a primary button.
  { text: '--text-inverse', surface: '--accent-500', min: AA_NORMAL },
  // Text on a solid semantic fill (destructive button, status badge). axe
  // caught this pair missing: the button used --danger as a fill with the
  // ordinary text token on top, which measured 3.1:1.
  { text: '--text-on-solid', surface: '--danger-solid', min: AA_NORMAL },
  { text: '--text-on-solid', surface: '--success-solid', min: AA_NORMAL },
  { text: '--text-on-solid', surface: '--warning-solid', min: AA_NORMAL },
  { text: '--text-on-solid', surface: '--info-solid', min: AA_NORMAL },
  { text: '--text-on-solid', surface: '--accent-solid', min: AA_NORMAL },
  // Risk chips are small but bold; AA large is the honest bar for a swatch
  // used as a fill behind short labels.
  ...['--risk-low', '--risk-moderate', '--risk-high', '--risk-critical', '--risk-extreme'].map(
    (risk) => ({ text: risk, surface: '--surface-2', min: AA_LARGE }),
  ),
];

const css = readFileSync(TOKENS, 'utf8');
const themes = {
  dark: parseBlock(css, ":root[data-theme='dark']"),
  light: parseBlock(css, ":root[data-theme='light']"),
};

let failures = 0;
let checked = 0;

for (const [themeName, scope] of Object.entries(themes)) {
  console.log(`\n${themeName.toUpperCase()}`);

  for (const { text, surface, min } of PAIRS) {
    const rawText = scope[text];
    const rawSurface = scope[surface];
    if (!rawText || !rawSurface) {
      console.log(`  ?  ${text} on ${surface} — token missing in ${themeName}`);
      failures++;
      continue;
    }

    const fg = parseColor(resolveVar(rawText, scope));
    const bgRaw = parseColor(resolveVar(rawSurface, scope));
    if (!fg || !bgRaw) {
      console.log(`  ?  ${text} on ${surface} — unparseable colour`);
      failures++;
      continue;
    }

    // Translucent surfaces composite over the page background.
    const pageBg = parseColor(resolveVar(scope['--surface-0'], scope));
    const bg = flatten(bgRaw, pageBg);
    const ratio = contrast(flatten(fg, bg), bg);
    checked++;

    const ok = ratio >= min;
    if (!ok) failures++;
    const mark = ok ? '.' : 'FAIL';
    if (!ok) {
      console.log(`  ${mark} ${text} on ${surface}: ${ratio.toFixed(2)}:1 (needs ${min}:1)`);
    }
  }
}

console.log(`\n${checked} pairs checked, ${failures} failing.`);

if (failures > 0) {
  console.error('\nContrast check FAILED. Adjust the token values in src/styles/tokens.css.');
  process.exit(1);
}
console.log('All token pairs meet their WCAG target in both themes.');
