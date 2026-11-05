import { test } from 'node:test';
import assert from 'node:assert/strict';
import { groupPrs, groupVerdict } from './queue-gate.mjs';

const E = (position, oid, number) => ({ position, headCommit: { oid }, pullRequest: { number } });

test('a merge group holds its PR and every PR ahead of it', () => {
  const entries = [E(1, 'a', 10), E(2, 'b', 11), E(3, 'c', 12)];
  assert.deepEqual(groupPrs(entries, 'b'), [10, 11]);
  assert.deepEqual(groupPrs(entries, 'c'), [10, 11, 12]);
  assert.equal(groupPrs(entries, 'zzz'), null);
});

test('the group passes only if every PR has the bot approval and its Red gate', () => {
  assert.equal(groupVerdict([{ pr: 1, review: 'success', red: 'success' }, { pr: 2, review: 'success', red: 'success' }]).state, 'success');
  const refused = groupVerdict([{ pr: 1, review: 'success', red: 'success' }, { pr: 1292, review: 'failure', red: 'pending' }]);
  assert.equal(refused.state, 'failure');
  assert.match(refused.description, /#1292/);
  const unreviewed = groupVerdict([{ pr: 1300, review: undefined, red: undefined }]);
  assert.equal(unreviewed.state, 'failure');
  assert.match(unreviewed.description, /#1300.*missing/);
  assert.equal(groupVerdict([]).state, 'failure');
});
