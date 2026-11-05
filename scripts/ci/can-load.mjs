#!/usr/bin/env node
// Says whether a script's relative imports all exist, WITHOUT running it (#1299).
// Delivery workflows run the default branch's scripts. When PRs that depend on each other
// merge in the wrong order, a script can be present while a module it imports is not yet,
// and the run would fail with ERR_MODULE_NOT_FOUND on develop. The workflows call this
// first and skip with a notice instead.
//
//   node scripts/ci/can-load.mjs <entry.mjs> [<entry2.mjs> …]
// Exit 0: every relative import resolves. Exit 1: the first missing file is printed.

import { readFileSync, existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const IMPORT = /(?:^|[\s;])(?:import|export)\s[^'"]*?\sfrom\s*['"](\.{1,2}\/[^'"]+)['"]|import\(\s*['"](\.{1,2}\/[^'"]+)['"]\s*\)|^\s*import\s*['"](\.{1,2}\/[^'"]+)['"]/gm;

export function missingImport(entry, seen = new Set()) {
  const file = resolve(entry);
  if (seen.has(file)) return null;
  seen.add(file);
  if (!existsSync(file)) return file;
  const src = readFileSync(file, 'utf8');
  for (const m of src.matchAll(IMPORT)) {
    const spec = m[1] || m[2] || m[3];
    const dep = resolve(dirname(file), spec);
    const miss = missingImport(dep, seen);
    if (miss) return miss;
  }
  return null;
}

if (process.argv[1]?.endsWith('can-load.mjs')) {
  const entries = process.argv.slice(2);
  if (!entries.length) { console.error('usage: can-load.mjs <entry.mjs> …'); process.exit(2); }
  for (const e of entries) {
    const miss = missingImport(e);
    if (miss) { console.log(`${e} cannot load yet: missing ${miss}`); process.exit(1); }
  }
}
