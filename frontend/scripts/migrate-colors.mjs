#!/usr/bin/env node
// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Rewrites raw Tailwind palette classes to semantic design tokens.
 *
 * Run: node scripts/migrate-colors.mjs <file|dir>...   (add --dry to preview)
 *
 * Why a codemod rather than hand edits: there were ~1600 raw classes across 69
 * files. Hand-editing that many is both slower and less reliable than a mapping
 * applied uniformly — and a mapping can be reviewed in one place, which a
 * thousand scattered edits cannot.
 *
 * The mapping is intentionally conservative. Where a raw class has no
 * unambiguous semantic meaning (a decorative gradient, a brand colour, a chart
 * series) it is LEFT ALONE and reported, so a human decides. Guessing would
 * quietly change designs.
 *
 * Direction of the neutral mapping: the codebase was authored dark-first, so
 * dark neutrals (zinc-900) are backgrounds and light neutrals (zinc-400) are
 * text. Mapping is therefore by role, not by lightness.
 */

import { readFileSync, writeFileSync, statSync, readdirSync } from 'node:fs';
import { join, extname } from 'node:path';

const NEUTRALS = '(?:slate|gray|zinc|neutral|stone)';

/** Ordered: first match wins, so put specific patterns before general ones. */
const RULES = [
  // ---- surfaces -----------------------------------------------------------
  [new RegExp(`\\bbg-${NEUTRALS}-950\\b`, 'g'), 'bg-surface-0'],
  [new RegExp(`\\bbg-${NEUTRALS}-900\\b`, 'g'), 'bg-surface-1'],
  [new RegExp(`\\bbg-${NEUTRALS}-800\\b`, 'g'), 'bg-surface-2'],
  [new RegExp(`\\bbg-${NEUTRALS}-700\\b`, 'g'), 'bg-surface-3'],
  [new RegExp(`\\bbg-${NEUTRALS}-(?:50|100|200)\\b`, 'g'), 'bg-surface-sunken'],
  // Mid neutrals are used as muted chips/dividers rather than page surfaces.
  [new RegExp(`\\bbg-${NEUTRALS}-(?:300|400|500|600)\\b`, 'g'), 'bg-surface-3'],
  [/\bbg-white\b/g, 'bg-surface-1'],

  // ---- text ---------------------------------------------------------------
  [/\btext-white\b/g, 'text-text-primary'],
  [new RegExp(`\\btext-${NEUTRALS}-(?:50|100|200)\\b`, 'g'), 'text-text-primary'],
  [new RegExp(`\\btext-${NEUTRALS}-(?:300|400)\\b`, 'g'), 'text-text-secondary'],
  [new RegExp(`\\btext-${NEUTRALS}-(?:500|600)\\b`, 'g'), 'text-text-muted'],
  [new RegExp(`\\btext-${NEUTRALS}-(?:700|800|900)\\b`, 'g'), 'text-text-primary'],

  // ---- borders / rings ----------------------------------------------------
  [new RegExp(`\\bborder-${NEUTRALS}-(?:600|700)\\b`, 'g'), 'border-border-default'],
  [new RegExp(`\\bborder-${NEUTRALS}-(?:800|900)\\b`, 'g'), 'border-border-subtle'],
  [new RegExp(`\\bborder-${NEUTRALS}-(?:100|200|300)\\b`, 'g'), 'border-border-subtle'],
  [new RegExp(`\\bdivide-${NEUTRALS}-(?:100|200|300|700|800)\\b`, 'g'), 'divide-border-subtle'],
  [new RegExp(`\\bborder-${NEUTRALS}-(?:400|500)\\b`, 'g'), 'border-border-strong'],
  [/\bborder-white\b/g, 'border-border-strong'],
  [new RegExp(`\\bring-${NEUTRALS}-(?:400|500|600|700)\\b`, 'g'), 'ring-border-strong'],

  // ---- semantic: danger ---------------------------------------------------
  [/\bbg-(?:red|rose)-(?:50|100|200)\b/g, 'bg-danger-surface'],
  [/\bbg-(?:red|rose)-(?:900|950)\/\d+\b/g, 'bg-danger-surface'],
  [/\bbg-(?:red|rose)-(?:500|600|700)\b/g, 'bg-danger'],
  [/\btext-(?:red|rose)-(?:300|400|500|600|700|800|900)\b/g, 'text-danger-text'],
  [/\bborder-(?:red|rose)-(?:400|500|600|700|800)\b/g, 'border-danger'],

  // ---- semantic: success --------------------------------------------------
  [/\bbg-(?:green|emerald)-(?:50|100|200)\b/g, 'bg-success-surface'],
  [/\bbg-(?:green|emerald)-(?:900|950)\/\d+\b/g, 'bg-success-surface'],
  [/\bbg-(?:green|emerald)-(?:500|600|700)\b/g, 'bg-success'],
  [/\btext-(?:green|emerald)-(?:300|400|500|600|700|800|900)\b/g, 'text-success-text'],
  [/\bborder-(?:green|emerald)-(?:400|500|600|700|800)\b/g, 'border-success'],

  // ---- semantic: warning --------------------------------------------------
  [/\bbg-(?:amber|yellow|orange)-(?:50|100|200)\b/g, 'bg-warning-surface'],
  [/\bbg-(?:amber|yellow|orange)-(?:900|950)\/\d+\b/g, 'bg-warning-surface'],
  [/\bbg-(?:amber|yellow|orange)-(?:500|600|700)\b/g, 'bg-warning'],
  [/\btext-(?:amber|yellow|orange)-(?:300|400|500|600|700|800|900)\b/g, 'text-warning-text'],
  [/\bborder-(?:amber|yellow|orange)-(?:400|500|600|700|800)\b/g, 'border-warning'],

  // ---- semantic: info / accent -------------------------------------------
  [/\bbg-(?:blue|sky|cyan|indigo)-(?:50|100|200)\b/g, 'bg-info-surface'],
  [/\bbg-(?:blue|sky|cyan|indigo)-(?:900|950)\/\d+\b/g, 'bg-info-surface'],
  [/\bbg-(?:blue|indigo)-(?:500|600|700)\b/g, 'bg-accent-500'],
  [/\btext-(?:blue|sky|cyan|indigo)-(?:300|400|500|600|700|800|900)\b/g, 'text-info-text'],
  [/\bborder-(?:blue|sky|cyan|indigo)-(?:400|500|600|700|800)\b/g, 'border-accent-400'],
  [/\bring-(?:blue|indigo)-(?:400|500|600)\b/g, 'ring-accent-400'],

  // ---- focus/hover variants of the above ---------------------------------
  // The prefixes ride along because the patterns match the utility itself,
  // e.g. "hover:bg-zinc-800" contains "bg-zinc-800".
];

/**
 * Classes left deliberately untouched, with the reason. Reported at the end so
 * the remaining work is visible rather than silently skipped.
 */
const KEEP = [
  [/\bbg-black\b/, 'scrim/backdrop — intentionally dark in both themes; use bg-surface-overlay if it is a modal scrim'],
  [/\bfrom-|\bvia-|\bto-/, 'gradient stop — decorative, needs a design decision'],
  [/-(?:purple|violet|fuchsia|pink|teal|lime)-/, 'no semantic token for this hue — chart series or brand accent'],
];

const args = process.argv.slice(2);
const dry = args.includes('--dry');
const targets = args.filter((a) => !a.startsWith('--'));

if (targets.length === 0) {
  console.error('usage: migrate-colors.mjs [--dry] <file|dir>...');
  process.exit(2);
}

function collect(path, out = []) {
  const st = statSync(path);
  if (st.isDirectory()) {
    for (const entry of readdirSync(path)) collect(join(path, entry), out);
  } else if (['.tsx', '.ts'].includes(extname(path))) {
    out.push(path);
  }
  return out;
}

const files = targets.flatMap((t) => collect(t));

let changedFiles = 0;
let totalReplacements = 0;
const remaining = new Map();

for (const file of files) {
  const original = readFileSync(file, 'utf8');
  let next = original;
  let count = 0;

  for (const [pattern, replacement] of RULES) {
    next = next.replace(pattern, () => {
      count++;
      return replacement;
    });
  }

  if (count > 0) {
    totalReplacements += count;
    changedFiles++;
    if (!dry) writeFileSync(file, next);
    console.log(`${dry ? 'would fix' : 'fixed'} ${count.toString().padStart(4)}  ${file}`);
  }

  // Report what the codemod deliberately declined to touch.
  for (const [pattern, reason] of KEEP) {
    if (pattern.test(next)) {
      const key = reason;
      remaining.set(key, (remaining.get(key) ?? 0) + 1);
    }
  }
}

console.log(
  `\n${totalReplacements} replacements across ${changedFiles}/${files.length} files${dry ? ' (dry run)' : ''}.`,
);

if (remaining.size > 0) {
  console.log('\nLeft for a human, by reason:');
  for (const [reason, n] of remaining) console.log(`  ${n} file(s): ${reason}`);
}
