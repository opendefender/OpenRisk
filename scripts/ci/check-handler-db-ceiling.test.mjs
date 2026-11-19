import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, cpSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { countRefs, compare } from './check-handler-db-ceiling.mjs';

test('a comment is not counted', () => {
  const src = [
    '// database.DB in a line comment',
    '/* database.DB in',
    '   a block comment */',
    'x := database.DB',
    'y := 1 // database.DB trailing',
  ].join('\n');
  assert.equal(countRefs(src), 1);
});

test('a URL in a string does not open a comment', () => {
  const src = 'u := "http://x"; z := database.DB\nw := `//` + database.DB\n';
  assert.equal(countRefs(src), 2);
});

test('database.DB inside a string literal is still a textual reference we count', () => {
  // The rule is /\bdatabase\.DB\b/ on non-comment text; literals are kept.
  assert.equal(countRefs('s := "database.DB"'), 1);
});

test('database.DBX is not counted', () => {
  assert.equal(countRefs('a := database.DBX\nb := mydatabase.DB\n'), 0);
});

test('a regression fails', () => {
  const p = compare({ 'internal/handler/a.go': 3 }, { files: { 'internal/handler/a.go': 2 } });
  assert.deepEqual(p, [{ file: 'internal/handler/a.go', count: 3, ceiling: 2, delta: 1 }]);
});

test('a stale (lower) count fails', () => {
  const p = compare({ 'internal/handler/a.go': 1 }, { files: { 'internal/handler/a.go': 2 } });
  assert.equal(p.length, 1);
  assert.equal(p[0].delta, -1);
});

test('a new file with one reference fails', () => {
  const p = compare({ 'internal/handler/new.go': 1 }, { files: {} });
  assert.equal(p.length, 1);
  assert.equal(p[0].ceiling, 0);
});

test('an equal count passes', () => {
  assert.deepEqual(compare({ 'a.go': 2 }, { files: { 'a.go': 2 } }), []);
});

// --update must refuse to raise. Run the real script against a scratch copy of
// the layout so the repository is not touched.
test('--update refuses to raise a ceiling', () => {
  const here = dirname(fileURLToPath(import.meta.url));
  const root = mkdtempSync(join(tmpdir(), 'ceiling-'));
  try {
    mkdirSync(join(root, 'scripts', 'ci'), { recursive: true });
    mkdirSync(join(root, 'backend', 'internal', 'handler'), { recursive: true });
    cpSync(join(here, 'check-handler-db-ceiling.mjs'), join(root, 'scripts', 'ci', 'check-handler-db-ceiling.mjs'));
    writeFileSync(join(root, 'backend', 'internal', 'handler', 'a.go'), 'package handler\nvar _ = database.DB\nvar _ = database.DB\n');
    const ceilingPath = join(root, 'backend', '.handler-db-ceiling.json');
    writeFileSync(ceilingPath, JSON.stringify({ total: 1, files: { 'internal/handler/a.go': 1 } }));
    const script = join(root, 'scripts', 'ci', 'check-handler-db-ceiling.mjs');

    const r = spawnSync('node', [script, '--update'], { encoding: 'utf8' });
    assert.equal(r.status, 1);
    assert.equal(JSON.parse(readFileSync(ceilingPath, 'utf8')).files['internal/handler/a.go'], 1);

    // The very first write (no file yet) is allowed.
    rmSync(ceilingPath);
    const first = spawnSync('node', [script, '--update'], { encoding: 'utf8' });
    assert.equal(first.status, 0);
    assert.equal(JSON.parse(readFileSync(ceilingPath, 'utf8')).total, 2);

    // A reduction is accepted and rewritten.
    writeFileSync(join(root, 'backend', 'internal', 'handler', 'a.go'), 'package handler\nvar _ = database.DB\n');
    const down = spawnSync('node', [script, '--update'], { encoding: 'utf8' });
    assert.equal(down.status, 0);
    assert.equal(JSON.parse(readFileSync(ceilingPath, 'utf8')).total, 1);

    // Check mode passes on equal and fails on a missing ceiling file.
    assert.equal(spawnSync('node', [script], { encoding: 'utf8' }).status, 0);
    rmSync(ceilingPath);
    assert.equal(spawnSync('node', [script], { encoding: 'utf8' }).status, 2);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
