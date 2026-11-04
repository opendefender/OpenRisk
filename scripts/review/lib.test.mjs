import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  requiredRoles, aggregate, requiredComment, parseRequired, verdictComment, rounds,
  afterAggregate, closingIssue, MAX_ROUNDS,
} from './lib.mjs';

const SHA = 'a'.repeat(40);

test('roles: tech-lead always, devsecops on red or unknown, qa when tests change, routed kept', () => {
  assert.deepEqual(requiredRoles({ routed: [], cls: 'green', files: ['docs/x.md'] }), ['tech-lead']);
  assert.deepEqual(requiredRoles({ routed: [], cls: 'red', files: ['backend/pkg/auth/x.go'] }), ['devsecops', 'tech-lead']);
  assert.deepEqual(requiredRoles({ routed: [], cls: undefined, files: [] }), ['devsecops', 'tech-lead']);
  assert.deepEqual(requiredRoles({ routed: [], cls: 'green', files: ['frontend/src/a.test.tsx'] }), ['qa-automation', 'tech-lead']);
  assert.deepEqual(requiredRoles({ routed: ['product-verifier', 'art-director'], cls: 'green', files: [] }), ['art-director', 'product-verifier', 'tech-lead']);
});

test('aggregate: all pass → success; any failure → failure; otherwise pending; nothing recorded → failure', () => {
  const req = ['devsecops', 'tech-lead'];
  assert.equal(aggregate(req, { devsecops: 'success', 'tech-lead': 'success' }).state, 'success');
  assert.equal(aggregate(req, { devsecops: 'failure', 'tech-lead': 'success' }).state, 'failure');
  assert.match(aggregate(req, { devsecops: 'failure' }).description, /devsecops/);
  assert.equal(aggregate(req, { 'tech-lead': 'success' }).state, 'pending');
  assert.match(aggregate(req, { 'tech-lead': 'success' }).description, /waiting for devsecops/);
  assert.equal(aggregate([], {}).state, 'failure');
});

test('required comment: written and read back; the latest one wins', () => {
  const c1 = requiredComment('b'.repeat(40), ['tech-lead']);
  const c2 = requiredComment(SHA, ['devsecops', 'tech-lead'], ['red class']);
  assert.deepEqual(parseRequired(['hello', c1, c2]), { sha: SHA, roles: ['devsecops', 'tech-lead'] });
  assert.equal(parseRequired(['nothing here']), null);
});

test('rounds: distinct head SHAs with a failing verdict; passes do not count', () => {
  const v = (sha, verdict) => verdictComment({ role: 'tech-lead', sha, verdict, blocking: verdict === 'fail' ? 1 : 0, findings: '1. x' });
  assert.equal(rounds([v('1'.repeat(40), 'fail'), v('1'.repeat(40), 'fail'), v('2'.repeat(40), 'fail'), v('3'.repeat(40), 'pass')]), 2);
  assert.equal(rounds([]), 0);
});

test('after a failure: send back, then block at the last round', () => {
  assert.deepEqual(afterAggregate('success', 2).action, 'none');
  assert.deepEqual(afterAggregate('pending', 0).action, 'none');
  assert.deepEqual(afterAggregate('failure', 1), { action: 'send-back', labels: { add: ['status:in-progress'], remove: ['status:in-review'] } });
  assert.equal(afterAggregate('failure', MAX_ROUNDS).action, 'block');
  assert.ok(afterAggregate('failure', MAX_ROUNDS).labels.add.includes('status:blocked'));
});

test('closing issue: Closes / Fixes / Resolves #n, else null', () => {
  assert.equal(closingIssue('Closes #1271\n\nmore'), 1271);
  assert.equal(closingIssue('fixes #12'), 12);
  assert.equal(closingIssue('Refs #1244'), null);
});
