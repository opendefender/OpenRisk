import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as select from './select.mjs';
import { pickRefinement, isRefined, byUrgency, hitUsageLimit } from './select.mjs';

const I = (number, labels, extra = {}) => ({ number, labels, authorAssociation: 'OWNER', milestone: 'M1', comments: [], hasOpenPr: false, ...extra });
const REFINED = [{ body: '<!-- refined: 2026-11-05 po-openrisk --> nothing to change: criteria testable', authorAssociation: 'OWNER' }];

test('refinement takes Refinement issues and Ready ones not refined yet, most urgent first', () => {
  const issues = [
    I(10, ['status:needs-refinement', 'priority:P2-medium']),
    I(11, ['status:ready', 'priority:P1-high']),
    I(12, ['status:ready', 'priority:P0-critical'], { comments: REFINED }), // already refined
    I(13, ['status:in-progress', 'priority:P0-critical']),
    I(14, ['status:needs-refinement', 'priority:P1-high', 'tier:0-trust']),
  ];
  assert.deepEqual(pickRefinement(issues, 10).map((i) => i.number), [14, 11, 10]);
  assert.deepEqual(pickRefinement(issues, 1).map((i) => i.number), [14]);
});

test('the runner has no way to pick an issue to implement (D-101)', () => {
  assert.equal(select.pickImplementation, undefined);
  assert.equal(select.turnBudget, undefined);
});

test('refinement never takes outsiders, blocked, deferred tier or issues with an open PR', () => {
  const issues = [
    I(30, ['status:ready'], { authorAssociation: 'NONE' }),
    I(31, ['status:ready', 'status:blocked']),
    I(32, ['status:ready', 'tier:5-deferred']),
    I(33, ['status:ready'], { hasOpenPr: true }),
    I(35, ['status:ready']),
  ];
  assert.deepEqual(pickRefinement(issues, 9).map((i) => i.number), [35]);
});

test('urgency: priority, then tier, then age', () => {
  const s = [I(3, ['priority:P1', 'tier:2-killer-workflow']), I(2, ['priority:P1-high', 'tier:0-trust']), I(1, ['priority:P2']), I(4, [])].sort(byUrgency);
  assert.deepEqual(s.map((i) => i.number), [2, 3, 1, 4]);
});

test('marker and the usage-limit signal', () => {
  assert.ok(isRefined({ comments: REFINED }));
  assert.ok(!isRefined({ comments: [{ body: 'refined by hand', authorAssociation: 'OWNER' }] }));
  // A marker from an outsider does not count, and does not stop the real refinement (#1320).
  const forged = { comments: [{ body: REFINED[0].body, authorAssociation: 'NONE' }] };
  assert.ok(!isRefined(forged));
  assert.deepEqual(pickRefinement([I(60, ['status:ready'], forged)], 9).map((i) => i.number), [60]);
  assert.ok(hitUsageLimit('Claude AI usage limit reached|1793900000'));
  assert.ok(!hitUsageLimit('all tests passed'));
});

import { resumeTarget } from './select.mjs';

test('resume: an interrupted fix comes back first; closed or blocked issues do not', () => {
  const issues = [I(50, ['status:in-review']), I(51, ['status:blocked'])];
  assert.deepEqual(resumeTarget({ resume: { step: 'fix', issue: 50, pr: 70 } }, issues), { step: 'fix', issue: 50, pr: 70 });
  // A run that never returned (power cut) is found through `current`.
  assert.deepEqual(resumeTarget({ current: { step: 'fix', issue: 50, pr: 70 } }, issues), { step: 'fix', issue: 50, pr: 70 });
  assert.equal(resumeTarget({ resume: { step: 'fix', issue: 51, pr: 71 } }, issues), null);
  assert.equal(resumeTarget({ resume: { step: 'fix', issue: 99, pr: 79 } }, issues), null);
  assert.equal(resumeTarget({ resume: { step: 'fix', issue: 50 } }, issues), null); // no PR to resume in
  assert.equal(resumeTarget({ current: { step: 'steward', issue: null } }, issues), null);
  assert.equal(resumeTarget({}, issues), null);
});

test('resume never turns an interrupted implementation or refinement into new work (D-101)', () => {
  const issues = [I(50, ['status:in-progress'])];
  // State left by a runner from before D-101, or by a refinement cut short.
  assert.equal(resumeTarget({ resume: { step: 'implement', issue: 50 } }, issues), null);
  assert.equal(resumeTarget({ current: { step: 'implement', issue: 50, pr: 70 } }, issues), null);
  assert.equal(resumeTarget({ resume: { step: 'refine', issue: 50 } }, issues), null);
});
