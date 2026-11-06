import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { missingImport } from './can-load.mjs';

test('a complete import graph loads; a missing module is named; nothing is executed', () => {
  const d = mkdtempSync(join(tmpdir(), 'canload-'));
  mkdirSync(join(d, 'review'));
  mkdirSync(join(d, 'merge'));
  writeFileSync(join(d, 'review', 'gh.mjs'), "import { x } from './lib.mjs';\nexport const y = 1;\n");
  writeFileSync(join(d, 'review', 'lib.mjs'), "throw new Error('must never run');\n");
  writeFileSync(join(d, 'merge', 'enqueue.mjs'), "import { y } from '../review/gh.mjs';\nimport { mayEnqueue } from './conditions.mjs';\n");
  writeFileSync(join(d, 'merge', 'conditions.mjs'), 'export const mayEnqueue = 1;\n');
  assert.equal(missingImport(join(d, 'merge', 'enqueue.mjs')), null);
  // The #1287-before-#1286 case: the imported module is not there yet.
  writeFileSync(join(d, 'merge', 'enqueue2.mjs'), "import { y } from '../review/missing.mjs';\n");
  assert.match(missingImport(join(d, 'merge', 'enqueue2.mjs')), /missing\.mjs$/);
  // Bare and node: imports are not files in the repo and are ignored.
  writeFileSync(join(d, 'merge', 'bare.mjs'), "import fs from 'node:fs';\nimport x from 'lodash';\n");
  assert.equal(missingImport(join(d, 'merge', 'bare.mjs')), null);
  // Dynamic imports count too.
  writeFileSync(join(d, 'merge', 'dyn.mjs'), "const m = await import('./nope.mjs');\n");
  assert.match(missingImport(join(d, 'merge', 'dyn.mjs')), /nope\.mjs$/);
});
