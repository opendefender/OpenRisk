#!/usr/bin/env node
// Handler database.DB ratchet (#808, D-055) - BLOCKING in CI.
//
// Handlers reach the database through use cases and repositories that take a
// tenantID. Every direct `database.DB` reference in backend/internal/handler/
// is a query that no repository gate sees, so the count may only go down.
//
// Per FILE, not one global total: a total alone lets a PR remove three
// references from one file and add three to a new handler. A file missing from
// the ceiling map has a ceiling of 0, so no new handler file may use
// database.DB.
//
// Exact match, not "<=": a reduction lowers backend/.handler-db-ceiling.json
// in the same PR (criterion 1 of #808), so an unrecorded reduction fails too.
//
// What counts: /\bdatabase\.DB\b/ in *.go files under internal/handler/
// (recursive), *_test.go excluded, comments stripped first. String, raw-string
// and rune literals are respected so a URL in a string is not a comment.
//
// Limit: only the literal `database.DB` is counted. A handler that reaches the
// database through another name (a package-level alias, a `db` field filled from
// database.DB elsewhere, a passed-in *gorm.DB) is not seen by this check.
//
// Usage (from the repository root):
//   node scripts/ci/check-handler-db-ceiling.mjs            check (CI)
//   node scripts/ci/check-handler-db-ceiling.mjs --update   re-freeze after a reduction
//                                                           (refuses to raise a ceiling)
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const BACKEND = join(ROOT, 'backend');
const HANDLER_DIR = join(BACKEND, 'internal', 'handler');
const CEILING_FILE = join(BACKEND, '.handler-db-ceiling.json');

// stripComments removes // and /* */ comments from Go source, leaving string,
// raw-string and rune literals intact.
export function stripComments(src) {
  let out = '';
  let i = 0;
  const n = src.length;
  while (i < n) {
    const c = src[i];
    const d = src[i + 1];
    if (c === '/' && d === '/') {
      while (i < n && src[i] !== '\n') i++;
    } else if (c === '/' && d === '*') {
      i += 2;
      while (i < n && !(src[i] === '*' && src[i + 1] === '/')) {
        if (src[i] === '\n') out += '\n';
        i++;
      }
      i += 2;
    } else if (c === '"' || c === "'") {
      out += c;
      i++;
      while (i < n && src[i] !== c) {
        if (src[i] === '\\') {
          out += src[i++];
        }
        if (i < n) out += src[i++];
      }
      if (i < n) out += src[i++];
    } else if (c === '`') {
      out += c;
      i++;
      while (i < n && src[i] !== '`') out += src[i++];
      if (i < n) out += src[i++];
    } else {
      out += c;
      i++;
    }
  }
  return out;
}

// countRefs returns the number of database.DB code references in src.
export function countRefs(src) {
  const m = stripComments(src).match(/\bdatabase\.DB\b/g);
  return m ? m.length : 0;
}

function walk(dir) {
  const out = [];
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) out.push(...walk(p));
    else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go')) out.push(p);
  }
  return out;
}

export function countAll() {
  const files = {};
  for (const f of walk(HANDLER_DIR).sort()) {
    const c = countRefs(readFileSync(f, 'utf8'));
    if (c > 0) files[relative(BACKEND, f).split('\\').join('/')] = c;
  }
  return files;
}

// compare returns the list of mismatches between counts and a ceiling
// ({files: {path: n}}). Above is a regression, below is an unrecorded reduction.
export function compare(counts, ceiling) {
  const max = ceiling.files ?? {};
  const problems = [];
  for (const file of new Set([...Object.keys(counts), ...Object.keys(max)])) {
    const count = counts[file] ?? 0;
    const allowed = max[file] ?? 0;
    if (count !== allowed) problems.push({ file, count, ceiling: allowed, delta: count - allowed });
  }
  return problems.sort((a, b) => a.file.localeCompare(b.file));
}

export function frozen(counts) {
  const files = Object.fromEntries(
    Object.entries(counts).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])),
  );
  const total = Object.values(files).reduce((a, b) => a + b, 0);
  return { total, files };
}

function main() {
  const update = process.argv.includes('--update');
  const counts = countAll();
  const total = Object.values(counts).reduce((a, b) => a + b, 0);

  if (update) {
    if (existsSync(CEILING_FILE)) {
      let old;
      try {
        old = JSON.parse(readFileSync(CEILING_FILE, 'utf8'));
      } catch {
        console.error(`ceiling file ${CEILING_FILE} is unreadable; fix it by hand.`);
        process.exit(2);
      }
      const raised = compare(counts, old).filter((p) => p.delta > 0);
      if (raised.length > 0) {
        for (const p of raised) console.error(`${p.file}: ${p.count} > ceiling ${p.ceiling} (+${p.delta})`);
        console.error('--update refuses to raise a ceiling. Move the query behind a use case.');
        process.exit(1);
      }
    }
    writeFileSync(CEILING_FILE, JSON.stringify(frozen(counts), null, 2) + '\n');
    console.log(`Ceiling frozen at ${total} database.DB references in ${Object.keys(counts).length} files.`);
    return;
  }

  let ceiling;
  try {
    ceiling = JSON.parse(readFileSync(CEILING_FILE, 'utf8'));
  } catch {
    console.error(`${CEILING_FILE} missing or unreadable. Run with --update to create it.`);
    process.exit(2);
  }

  const problems = compare(counts, ceiling);
  if (problems.length > 0) {
    console.error('Handler database.DB ratchet failed:');
    for (const p of problems) {
      const sign = p.delta > 0 ? `+${p.delta}` : `${p.delta}`;
      const what = p.delta > 0 ? 'regression' : 'reduction not recorded in the ceiling';
      console.error(`  ${p.file}: ${p.count} references, ceiling ${p.ceiling} (${sign}) - ${what}`);
    }
    console.error('Move the query behind a use case; do not raise the ceiling.');
    console.error('After a reduction: node scripts/ci/check-handler-db-ceiling.mjs --update');
    process.exit(1);
  }
  console.log(`OK: ${total} database.DB references in ${Object.keys(counts).length} handler files, equal to the ceiling.`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) main();
