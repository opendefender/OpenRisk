import { test } from 'node:test';
import assert from 'node:assert/strict';
import { judgeAll } from './run.mjs';

test('judgeAll: one PR that throws does not stop the others', async () => {
  const seen = [];
  const lines = [];
  const judge = async (n) => {
    seen.push(n);
    if (n === 2) throw new Error('GET /check-runs: HTTP 500');
    return { pr: n, result: 'refused', reasons: ['x'] };
  };
  const errored = await judgeAll([1, 2, 3], judge, (l) => lines.push(l));
  assert.deepEqual(seen, [1, 2, 3]);
  assert.equal(errored, 1);
  assert.ok(lines.includes('#2: error — GET /check-runs: HTTP 500'));
  assert.ok(lines.includes('#3: refused'));
});

test('judgeAll: no error → 0', async () => {
  const errored = await judgeAll([5], async (n) => ({ pr: n, result: 'enqueued' }), () => {});
  assert.equal(errored, 0);
});
