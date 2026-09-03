#!/usr/bin/env node
// anime.js budget gate (#445, D-028) — BLOCKING in CI.
//
// Two assertions, and the FIRST is the one that matters:
//
//   1. anime.js must never appear in the preloaded graph. It is admitted for one
//      job — drawing SVG on entry — on lazily-routed screens. If it reaches the
//      entry chunk it is being paid for by every user on every page load, which
//      is not what D-028 approved.
//
//   2. The `anime` chunk stays within its gzip ceiling.
//
// WHY THE CEILING IS 15.5 KB AND NOT THE 12 KB #445 ASKED FOR
// -----------------------------------------------------------
// #445 budgeted "Timer 5.60 + Animation 5.20 + SVG 0.35 + Stagger 0.48 = 11.63 KB"
// and set the assertion at 12 KB. That arithmetic omits the **Scope** module —
// while the issue's own first task mandates `createScope({ root })` plus
// `scope.revert()`, without which StrictMode's double mount leaks observers.
// Scope is ~3 KB, so the module set the issue REQUIRES cannot fit the ceiling the
// issue NAMES.
//
// Measured on this branch with `createScope` + `animate` + `stagger` (no SVG
// module — `createDrawable` is deliberately unused, see CartesianChart):
//
//     dist/assets/anime-*.js    37.31 KB raw    14.95 KB gzip
//
// 15.5 KB is that measurement plus ~0.5 KB of headroom. This is a NEW gate where
// none existed, set at the measured floor — not a relaxation of an existing one.
//
// THIS CEILING MAY ONLY EVER BE LOWERED. If it needs to rise, the import surface
// has grown, and that is the thing to review — not this number.
//
// Usage: node scripts/check-anime-budget.mjs   (run after `vite build`)
import { readFileSync, existsSync, readdirSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { join } from 'node:path';

const DIST = 'dist';
const CEILING_KB = Number(process.env.ANIME_BUDGET_KB ?? 15.5);

const indexHtml = join(DIST, 'index.html');
if (!existsSync(indexHtml)) {
  console.error(`✗ ${indexHtml} not found — run \`vite build\` first.`);
  process.exit(2);
}

const assets = join(DIST, 'assets');
const files = readdirSync(assets).filter((f) => f.endsWith('.js'));
const animeChunks = files.filter((f) => /^anime-/.test(f));

/* ---------------------------------------------- 1. never preloaded -------- */
const html = readFileSync(indexHtml, 'utf8');
const preloaded = new Set(
  [...html.matchAll(/assets\/([A-Za-z0-9_-]+\.js)/g)].map((m) => m[1]),
);

const leaked = animeChunks.filter((f) => preloaded.has(f));
if (leaked.length > 0) {
  console.error('✗ anime.js is in the PRELOADED graph:\n');
  for (const f of leaked) console.error(`    assets/${f}`);
  console.error(
    '\nanime.js is admitted for entry animation on lazily-routed screens only.\n' +
      'Something on the preloaded path now imports it — most likely a barrel:\n' +
      '`shared/ds/index.ts` must not re-export anything that reaches useAnimeScope.',
  );
  process.exit(1);
}

/* Also catch it being inlined into a preloaded chunk under another name. */
const FINGERPRINT = /createScope|animejs/;
const contaminated = [...preloaded].filter((f) => {
  const p = join(assets, f);
  return existsSync(p) && FINGERPRINT.test(readFileSync(p, 'utf8'));
});
if (contaminated.length > 0) {
  console.error('✗ anime.js source appears inside a PRELOADED chunk:\n');
  for (const f of contaminated) console.error(`    assets/${f}`);
  console.error('\nThe manualChunks rule in vite.config.ts should isolate it as `anime`.');
  process.exit(1);
}

/* ---------------------------------------------- 2. size ceiling ----------- */
if (animeChunks.length === 0) {
  console.log('✓ No anime chunk in this build — nothing imports anime.js.');
  console.log('  (Assertion 1 passed vacuously; nothing to weigh.)');
  process.exit(0);
}

let total = 0;
for (const f of animeChunks) {
  const gz = gzipSync(readFileSync(join(assets, f))).length;
  total += gz;
  console.log(`  ${(gz / 1000).toFixed(1).padStart(6)} KB  assets/${f}`);
}

const totalKB = total / 1000;
console.log(`\n  ${totalKB.toFixed(1)} KB TOTAL (gzip)   Ceiling: ${CEILING_KB} KB`);

if (totalKB > CEILING_KB) {
  console.error(
    `\n✗ anime.js is ${(totalKB - CEILING_KB).toFixed(1)} KB over its ceiling.\n\n` +
      'Do not raise the number. Narrow the import surface: this budget covers\n' +
      '`createScope`, `animate` and `stagger`, and nothing else. A new named import\n' +
      '— `createDrawable`, `createTimeline`, `Scroll` — is what moved it.',
  );
  process.exit(1);
}

console.log(`\n✓ Within ceiling, and not preloaded.`);
