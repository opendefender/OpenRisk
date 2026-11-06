import { test } from 'node:test';
import assert from 'node:assert/strict';
import { deriveStatus, deliveryStage, branchIssueNumbers, derivePriority, deriveTier, ARCHIVE } from './derive.mjs';

const cfg = { ownerLabels: ['status:needs-owner', 'needs-owner'] };
const open = (labels = [], extra = {}) => ({ state: 'OPEN', labels, delivered: null, openPr: false, hasBranch: false, ...extra });
const closed = (stateReason, extra = {}) => ({ state: 'CLOSED', stateReason, labels: [], delivered: null, ...extra });

// One test per row of #1248 criterion 1.
test('opened → Inbox', () => {
  assert.equal(deriveStatus(open(), cfg), 'Inbox');
  assert.equal(deriveStatus(open(['type:bug']), cfg), 'Inbox');
});
test('status:needs-refinement → Refinement', () => assert.equal(deriveStatus(open(['status:needs-refinement']), cfg), 'Refinement'));
test('status:ready → Ready', () => assert.equal(deriveStatus(open(['status:ready']), cfg), 'Ready'));
test('a <type>/<n>-* branch → In progress, even before the label', () => {
  assert.equal(deriveStatus(open(['status:ready'], { hasBranch: true }), cfg), 'In progress');
  assert.equal(deriveStatus(open(['status:in-progress']), cfg), 'In progress');
});
test('an open closing PR → In review, even before the label', () => {
  assert.equal(deriveStatus(open(['status:in-progress'], { hasBranch: true, openPr: true }), cfg), 'In review');
  assert.equal(deriveStatus(open(['status:in-review']), cfg), 'In review');
});
test('merged into the dev branch → On dev', () => {
  assert.equal(deriveStatus(closed('COMPLETED', { delivered: 'dev' }), cfg), 'On dev');
  // While develop is not the default branch, the issue stays open after the merge.
  assert.equal(deriveStatus(open(['status:in-review'], { delivered: 'dev', openPr: true }), cfg), 'On dev');
});
test('closing commit on staging → On staging', () => assert.equal(deriveStatus(closed('COMPLETED', { delivered: 'staging' }), cfg), 'On staging'));
test('closing commit in a release tag → Released', () => assert.equal(deriveStatus(closed('COMPLETED', { delivered: 'released' }), cfg), 'Released'));
test('status:blocked → Blocked, the owner label → Needs owner, both beat every open fact', () => {
  assert.equal(deriveStatus(open(['status:blocked'], { openPr: true, hasBranch: true }), cfg), 'Blocked');
  assert.equal(deriveStatus(open(['status:blocked', 'needs-owner']), cfg), 'Needs owner');
  assert.equal(deriveStatus(open(['status:needs-owner'], { delivered: 'dev' }), cfg), 'Needs owner');
});

test('moves back only when the fact goes away', () => {
  // The PR is closed without merge and the branch is still there.
  assert.equal(deriveStatus(open([], { hasBranch: true, openPr: false }), cfg), 'In progress');
  // The branch is deleted too, the ready label stayed.
  assert.equal(deriveStatus(open(['status:ready']), cfg), 'Ready');
});

test('closed: not planned and duplicate archived; completed with no merged change archived', () => {
  assert.equal(deriveStatus(closed('NOT_PLANNED', { delivered: 'released' }), cfg), ARCHIVE);
  assert.equal(deriveStatus(closed('DUPLICATE'), cfg), ARCHIVE);
  assert.equal(deriveStatus(closed('COMPLETED'), cfg), ARCHIVE);
});

test('owner labels come from the config', () => {
  assert.equal(deriveStatus(open(['needs-owner']), { ownerLabels: [] }), 'Inbox');
});

test('deliveryStage: furthest stage wins, an unknown commit is not delivered', () => {
  const on = (set) => (sha) => set.includes(sha);
  const refs = { onDev: on(['a', 'b', 'c']), onStaging: on(['b', 'c']), released: on(['c']) };
  assert.equal(deliveryStage('a', refs), 'dev');
  assert.equal(deliveryStage('b', refs), 'staging');
  assert.equal(deliveryStage('c', refs), 'released');
  assert.equal(deliveryStage('z', refs), null);
  assert.equal(deliveryStage(null, refs), null);
});

test('branchIssueNumbers: only <type>/<n>-<slug>', () => {
  const n = branchIssueNumbers(['refs/heads/chore/1248-board', 'refs/heads/feat/12-x', 'refs/heads/master', 'refs/heads/agent', 'refs/heads/dependabot/npm/x-1', 'refs/heads/Feat/9-x']);
  assert.deepEqual([...n].sort((a, b) => a - b), [12, 1248]);
});

test('priority: Constitution family first, then the wave family, most urgent wins', () => {
  assert.equal(derivePriority(['priority:P2-medium', 'priority:P1']), 'P2-medium');
  assert.equal(derivePriority(['priority:P1']), 'P1-high');
  assert.equal(derivePriority(['priority:P3-low', 'priority:P0-critical']), 'P0-critical');
  assert.equal(derivePriority(['type:bug']), null);
  assert.equal(derivePriority(['priority:P9']), null);
});

test('tier: one of the six values of D-032, nothing else', () => {
  assert.equal(deriveTier(['tier:3-ecosystem']), '3-ecosystem');
  assert.equal(deriveTier(['tier:7-made-up']), null);
  assert.equal(deriveTier([]), null);
});

import { deriveSprint, sprintEnd } from './derive.mjs';

test('sprint: started work without a sprint joins the current one; planned and idle work is left', () => {
  const cur = { id: 'S2', startDate: '2026-11-09', duration: 7 };
  const today = '2026-11-11';
  const open = (sprint) => ({ state: 'OPEN', sprint });
  assert.equal(deriveSprint('In progress', open(null), cur, today), 'S2');
  assert.equal(deriveSprint('In review', open(null), cur, today), 'S2');
  assert.equal(deriveSprint('Ready', open(null), cur, today), undefined);
  assert.equal(deriveSprint('Inbox', open(null), cur, today), undefined);
  // Planned in a future sprint: left alone, even if started early.
  assert.equal(deriveSprint('In progress', open({ id: 'S3', startDate: '2026-11-16', duration: 7 }), cur, today), undefined);
  assert.equal(deriveSprint('Ready', open({ id: 'S2', startDate: '2026-11-09', duration: 7 }), cur, today), undefined);
});

test('sprint: unfinished work of an ended sprint carries over; delivered and closed work keeps its sprint', () => {
  const cur = { id: 'S2', startDate: '2026-11-09', duration: 7 };
  const past = { id: 'S1', startDate: '2026-11-02', duration: 7 };
  const today = '2026-11-09';
  assert.equal(deriveSprint('Ready', { state: 'OPEN', sprint: past }, cur, today), 'S2');
  assert.equal(deriveSprint('Blocked', { state: 'OPEN', sprint: past }, cur, today), 'S2');
  assert.equal(deriveSprint('On dev', { state: 'OPEN', sprint: past }, cur, today), undefined);
  assert.equal(deriveSprint('Released', { state: 'CLOSED', sprint: past }, cur, today), undefined);
  assert.equal(deriveSprint('In progress', { state: 'OPEN', sprint: null }, null, today), undefined);
  assert.equal(sprintEnd(past), '2026-11-09');
});

// #1365: a label nothing in git backs stops counting after STALE_DAYS.
test('a stale in-progress / in-review label is ignored, a fresh one still counts', async () => {
  const { staleActiveLabel, STALE_DAYS } = await import('./derive.mjs');
  const now = Date.parse('2026-11-06T12:00:00Z');
  const old = new Date(now - (STALE_DAYS + 1) * 86400e3).toISOString();
  const fresh = new Date(now - 86400e3).toISOString();
  const idle = open(['status:ready', 'status:in-progress'], { updatedAt: old });
  assert.equal(staleActiveLabel(idle, now), true);
  assert.equal(deriveStatus({ ...idle, staleLabel: true }, cfg), 'Ready');
  assert.equal(deriveStatus({ ...open(['status:in-review'], { updatedAt: old }), staleLabel: true }, cfg), 'Inbox');
  assert.equal(staleActiveLabel(open(['status:in-progress'], { updatedAt: fresh }), now), false);
  // Git facts always win, whatever the age.
  assert.equal(staleActiveLabel(open(['status:in-progress'], { updatedAt: old, hasBranch: true }), now), false);
  assert.equal(staleActiveLabel(open(['status:in-review'], { updatedAt: old, openPr: true }), now), false);
  assert.equal(staleActiveLabel(open(['status:ready'], { updatedAt: old }), now), false);
  assert.equal(staleActiveLabel({ ...closed('COMPLETED'), labels: ['status:in-progress'], updatedAt: old }, now), false);
});
