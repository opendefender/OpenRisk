#!/usr/bin/env node
// Performance budget gate (task §2) — BLOCKING in CI.
//
// Sums the gzipped size of the JS chunks the entry HTML preloads (the critical
// path a browser must fetch+parse before first paint) and fails if it exceeds the
// budget. Route-split chunks and on-demand chunks (charts, zxcvbn dictionaries,
// the command palette, forms…) are NOT counted — they load with the route/action
// that needs them, which is the whole point of the split.
//
// Usage: node scripts/check-bundle-budget.mjs   (run after `vite build`)
import { readFileSync, existsSync, readdirSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { join } from 'node:path';

const DIST = 'dist';
// The design guide's target, met by #462: 250 -> 248 (#446) -> 180. Measured
// 179.7 KB on 2026-09-01, so there is ~0.3 KB of headroom and the next
// regression fails the build rather than eroding the number.
//
// It got here by moving what was never needed on first paint OUT of the preload
// rather than by deleting features: recharts' and @hello-pangea/dnd's Redux
// engines, framer-motion's engine, and a second toast library were all falling
// through to the `vendor` chunk, which is preloaded because it also holds axios
// and zustand. See #462 and D-018.
//
// THIS BUDGET MAY ONLY EVER BE LOWERED. Raising it to accommodate a new
// dependency is the decision that makes it meaningless; the dependency goes in
// a lazy chunk instead, or it does not go in.
const BUDGET_KB = Number(process.env.BUNDLE_BUDGET_KB ?? 180);

const indexHtml = join(DIST, 'index.html');
if (!existsSync(indexHtml)) {
  console.error(`✗ ${indexHtml} not found — run \`vite build\` first.`);
  process.exit(2);
}

const html = readFileSync(indexHtml, 'utf8');
// Collect every JS asset the entry HTML references (script src + modulepreload).
const refs = [...html.matchAll(/assets\/[A-Za-z0-9_-]+\.js/g)].map((m) => m[0]);
const initial = [...new Set(refs)];

if (initial.length === 0) {
  console.error('✗ No JS assets referenced by index.html — unexpected build output.');
  process.exit(2);
}

let total = 0;
const rows = [];
for (const rel of initial) {
  const file = join(DIST, rel);
  if (!existsSync(file)) continue;
  const gz = gzipSync(readFileSync(file)).length;
  total += gz;
  rows.push({ rel, gz });
}
rows.sort((a, b) => b.gz - a.gz);

const kb = (n) => (n / 1024).toFixed(1);
console.log('Initial (preloaded) JS — gzipped:');
for (const r of rows) console.log(`  ${kb(r.gz).padStart(7)} KB  ${r.rel}`);
console.log(`  ${'-'.repeat(7)}`);
console.log(`  ${kb(total).padStart(7)} KB  TOTAL`);
console.log(`Budget: ${BUDGET_KB} KB`);

// Report the largest lazy chunks too, for visibility (not counted).
const lazy = readdirSync(join(DIST, 'assets'))
  .filter((f) => f.endsWith('.js') && !initial.includes(`assets/${f}`))
  .map((f) => ({ f, gz: gzipSync(readFileSync(join(DIST, 'assets', f))).length }))
  .sort((a, b) => b.gz - a.gz)
  .slice(0, 5);
if (lazy.length) {
  console.log('\nLargest lazy chunks (loaded on demand, not in the budget):');
  for (const l of lazy) console.log(`  ${kb(l.gz).padStart(7)} KB  assets/${l.f}`);
}

if (total > BUDGET_KB * 1024) {
  console.error(`\n✗ Initial bundle ${kb(total)} KB exceeds the ${BUDGET_KB} KB budget.`);
  process.exit(1);
}
console.log(`\n✓ Within budget (${kb(total)} KB ≤ ${BUDGET_KB} KB).`);
