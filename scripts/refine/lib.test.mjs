import { test } from 'node:test';
import assert from 'node:assert/strict';
import { closeProblem } from './lib.mjs';

test('a close needs a known reason, a proof of 10+ characters, and the kept issue for a duplicate', () => {
  assert.equal(closeProblem('not_planned', undefined, 'see docs/DECISIONS.md'), null);
  assert.equal(closeProblem('duplicate', '12', 'same request as #12'), null);
  assert.match(closeProblem('wontfix', undefined, 'see docs/DECISIONS.md'), /reason/);
  assert.match(closeProblem('not_planned', undefined, 'short'), /proof/);
  assert.match(closeProblem('duplicate', undefined, 'same request as another'), /kept issue/);
  assert.match(closeProblem('duplicate', 'x', 'same request as another'), /kept issue/);
});
