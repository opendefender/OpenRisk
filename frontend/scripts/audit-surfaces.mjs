// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Structural guards against deceptive UI (W0-05).
 *
 * `openrisk/no-mock-data` already forbids fabricated data by NAME — MOCK_RISKS,
 * fakeUsers, placeholderData. That is the reliable signal for a fixture array,
 * but it cannot see three other shapes the W0-05 audit found:
 *
 *   1. a route that exists only as a placeholder,
 *   2. a fixture file that is genuinely imported by production code,
 *   3. a primary action rendered as available with no handler behind it.
 *
 * None of those involve the word "mock". So they are checked structurally here,
 * from the import graph and the route table, rather than by grepping for words.
 *
 * Run: `npm run audit:surfaces`
 * Asserted by: src/__tests__/deceptive-ui.test.ts
 *
 * Exit code 0 = clean, 1 = at least one violation.
 */

import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, dirname, normalize, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = normalize(join(dirname(fileURLToPath(import.meta.url)), '..'));
const SRC = join(ROOT, 'src');
const ENTRY = join(SRC, 'main.tsx');

/* ------------------------------------------------------------------ *
 * Import graph
 * ------------------------------------------------------------------ */

const CODE = /\.(ts|tsx)$/;
const TEST = /(\.test\.tsx?|\.spec\.tsx?)$/;

/** Resolves a relative specifier the way the bundler does. */
function resolveModule(base) {
  for (const ext of ['', '.tsx', '.ts', '/index.tsx', '/index.ts']) {
    const candidate = base + ext;
    if (existsSync(candidate) && statSync(candidate).isFile()) return normalize(candidate);
  }
  return null;
}

function listSourceFiles(dir, acc = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (name === '__tests__' || name === 'node_modules') continue;
      listSourceFiles(full, acc);
    } else if (CODE.test(name) && !TEST.test(name)) {
      acc.push(normalize(full));
    }
  }
  return acc;
}

// Matches `from './x'`, `import('./x')` — relative specifiers only. Bare
// specifiers are package imports and cannot reach a repo fixture.
const IMPORT = /(?:from\s+|import\s*\(\s*)['"](\.[^'"]+)['"]/g;

function buildGraph(files) {
  const graph = new Map();
  for (const file of files) {
    const source = readFileSync(file, 'utf8');
    const deps = new Set();
    for (const match of source.matchAll(IMPORT)) {
      const target = resolveModule(normalize(join(dirname(file), match[1])));
      if (target) deps.add(target);
    }
    graph.set(file, deps);
  }
  return graph;
}

function reachableFrom(graph, entry) {
  const seen = new Set();
  const stack = [entry];
  while (stack.length) {
    const node = stack.pop();
    if (seen.has(node)) continue;
    seen.add(node);
    for (const dep of graph.get(node) ?? []) stack.push(dep);
  }
  return seen;
}

/* ------------------------------------------------------------------ *
 * Check 1 — placeholder routes
 * ------------------------------------------------------------------ */

// A route path that reads as scaffolding. `preview` is included because a
// "preview" route historically meant "we have not built this yet"; the one
// legitimate use (the scan preview, which shows real discovered assets before
// import) is declared below rather than pattern-matched away.
const PLACEHOLDER_PATH = /\b(demo|mock|placeholder|coming-?soon|experimental|dummy|fixture|sandbox)\b/i;

// Paths that match the pattern but are genuinely implemented. Each needs a
// reason, so an exception is a decision recorded in review rather than a
// silent addition to a list.
const ALLOWED_PATHS = new Map([
  [
    'infrastructure/scans/:jobId',
    'Scan preview: renders REAL assets and findings discovered by a scan, held ' +
      'in Redis until the user imports them. "Preview" describes the staging ' +
      'step (nothing is written to the inventory yet), not an unbuilt feature.',
  ],
]);

function checkPlaceholderRoutes() {
  const app = readFileSync(join(SRC, 'App.tsx'), 'utf8');
  const violations = [];
  for (const match of app.matchAll(/<Route\s+[^>]*path=["']([^"']+)["']/g)) {
    const path = match[1];
    if (!PLACEHOLDER_PATH.test(path)) continue;
    if (ALLOWED_PATHS.has(path)) continue;
    const line = app.slice(0, match.index).split('\n').length;
    violations.push(`App.tsx:${line}  route "${path}" reads as a placeholder`);
  }
  return violations;
}

/* ------------------------------------------------------------------ *
 * Check 2 — fixture leakage
 * ------------------------------------------------------------------ */

// Directories whose contents exist to serve tests or demonstrations. Reaching
// one of them from main.tsx means fabricated data is in the production bundle.
const FIXTURE_DIRS = [
  join(ROOT, 'src', 'test'),
  join(ROOT, '..', 'dev', 'fixtures'),
  join(ROOT, '..', 'tests'),
];

function checkFixtureLeakage(reachable) {
  const violations = [];
  for (const file of reachable) {
    if (file.includes('__tests__') || TEST.test(file)) {
      violations.push(`${relative(ROOT, file)} is a test module but is reachable from main.tsx`);
      continue;
    }
    for (const dir of FIXTURE_DIRS) {
      if (file.startsWith(normalize(dir) + '/')) {
        violations.push(`${relative(ROOT, file)} is fixture/test data but is reachable from main.tsx`);
      }
    }
  }
  return violations;
}

/* ------------------------------------------------------------------ *
 * Check 3 — inert primary actions
 * ------------------------------------------------------------------ */

// Any prop that gives a button an effect. A spread (`{...props}`) may carry one,
// so a spread counts: the alternative is flagging every wrapper component.
const INTERACTIVE = /\bon(Click|MouseDown|PointerDown|KeyDown|Submit)\b|\btype\s*=\s*["{]?submit|\.\.\./;

/**
 * Reads the opening tag of a `<button>`, tracking brace depth so a `>` inside a
 * JSX expression (`style={{ a: b > c }}`) does not end the tag early.
 */
function openingTag(source, from) {
  let depth = 0;
  for (let i = from; i < source.length; i += 1) {
    const ch = source[i];
    if (ch === '{') depth += 1;
    else if (ch === '}') depth -= 1;
    else if (ch === '>' && depth === 0) return source.slice(from, i);
  }
  return source.slice(from);
}

/** Strips comments, so a `<button>` discussed in prose is not parsed as one. */
function stripComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' ')).replace(/\/\/[^\n]*/g, '');
}

function checkInertActions(reachable) {
  const violations = [];
  for (const file of [...reachable].sort()) {
    if (!file.endsWith('.tsx')) continue;
    const source = stripComments(readFileSync(file, 'utf8'));
    for (const match of source.matchAll(/<button\b/g)) {
      const tag = openingTag(source, match.index + match[0].length);
      if (INTERACTIVE.test(tag)) continue;
      const line = source.slice(0, match.index).split('\n').length;
      violations.push(`${relative(ROOT, file)}:${line}  <button> with no handler`);
    }
  }
  return violations;
}

/* ------------------------------------------------------------------ *
 * Report
 * ------------------------------------------------------------------ */

export function audit() {
  const files = listSourceFiles(SRC);
  const graph = buildGraph(files);
  const reachable = reachableFrom(graph, ENTRY);
  const orphans = files.filter((f) => !reachable.has(f));

  return {
    reachable: reachable.size,
    orphans: orphans.length,
    orphanFiles: orphans.map((f) => relative(ROOT, f)).sort(),
    placeholderRoutes: checkPlaceholderRoutes(),
    fixtureLeakage: checkFixtureLeakage(reachable),
    inertActions: checkInertActions(reachable),
  };
}

// Run directly (not when imported by the test).
if (process.argv[1] && normalize(process.argv[1]) === normalize(fileURLToPath(import.meta.url))) {
  const result = audit();
  const sections = [
    ['Placeholder routes', result.placeholderRoutes],
    ['Fixture leakage', result.fixtureLeakage],
    ['Inert primary actions', result.inertActions],
  ];

  console.log(`Reachable modules: ${result.reachable}`);
  console.log(`Orphaned modules:  ${result.orphans}  (unreachable from main.tsx; not user-facing)`);
  console.log('');

  let failed = false;
  for (const [name, violations] of sections) {
    if (violations.length === 0) {
      console.log(`PASS  ${name}`);
    } else {
      failed = true;
      console.log(`FAIL  ${name}  (${violations.length})`);
      for (const v of violations) console.log(`        ${v}`);
    }
  }

  process.exit(failed ? 1 : 0);
}
