import { test } from 'node:test';
import assert from 'node:assert/strict';
import { pickRefinement, pickImplementation, isRefined, byUrgency, turnBudget, hitUsageLimit } from './select.mjs';

const I = (number, labels, extra = {}) => ({ number, labels, authorAssociation: 'OWNER', milestone: 'M1', comments: [], hasOpenPr: false, ...extra });
const REFINED = ['<!-- refined: 2026-11-05 po-openrisk --> nothing to change: criteria testable'];

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

test('implementation takes only Ready AND refined issues (D-094)', () => {
  const issues = [
    I(20, ['status:ready', 'priority:P0-critical']), // not refined: never implemented
    I(21, ['status:ready', 'priority:P2-medium'], { comments: REFINED }),
    I(22, ['status:needs-refinement', 'priority:P0-critical'], { comments: REFINED }),
    I(23, ['status:ready', 'priority:P1-high'], { comments: REFINED }),
  ];
  assert.deepEqual(pickImplementation(issues, 5).map((i) => i.number), [23, 21]);
});

test('never: outsiders, blocked, deferred tier, issues with an open PR, other milestones', () => {
  const ok = { comments: REFINED };
  const issues = [
    I(30, ['status:ready'], { ...ok, authorAssociation: 'NONE' }),
    I(31, ['status:ready', 'status:blocked'], ok),
    I(32, ['status:ready', 'tier:5-deferred'], ok),
    I(33, ['status:ready'], { ...ok, hasOpenPr: true }),
    I(34, ['status:ready'], { ...ok, milestone: 'Other' }),
    I(35, ['status:ready'], ok),
  ];
  assert.deepEqual(pickImplementation(issues, 9, { milestone: 'M1' }).map((i) => i.number), [35]);
  assert.deepEqual(pickRefinement(issues.map((i) => ({ ...i, comments: [] })), 9).map((i) => i.number).includes(30), false);
});

test('urgency: priority, then tier, then age', () => {
  const s = [I(3, ['priority:P1', 'tier:2-killer-workflow']), I(2, ['priority:P1-high', 'tier:0-trust']), I(1, ['priority:P2']), I(4, [])].sort(byUrgency);
  assert.deepEqual(s.map((i) => i.number), [2, 3, 1, 4]);
});

test('marker, budgets and the usage-limit signal', () => {
  assert.ok(isRefined({ comments: REFINED }));
  assert.ok(!isRefined({ comments: ['refined by hand'] }));
  assert.equal(turnBudget('L', { S: 40, M: 90, L: 160 }), 160);
  assert.equal(turnBudget(undefined, { S: 40, M: 90, L: 160 }), 90);
  assert.ok(hitUsageLimit('Claude AI usage limit reached|1793900000'));
  assert.ok(!hitUsageLimit('all tests passed'));
});
