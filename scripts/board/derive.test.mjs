import { test } from 'node:test';
import assert from 'node:assert/strict';
import { deriveStatus, derivePriority, deriveTier, ARCHIVE } from './derive.mjs';

const cfg = { ownerLabels: ['status:needs-owner', 'needs-owner'] };
const open = (...labels) => ({ state: 'OPEN', labels });

test('open issue: one status per label, Inbox without one', () => {
  assert.equal(deriveStatus(open(), cfg), 'Inbox');
  assert.equal(deriveStatus(open('type:bug'), cfg), 'Inbox');
  assert.equal(deriveStatus(open('status:needs-refinement'), cfg), 'Refinement');
  assert.equal(deriveStatus(open('status:ready'), cfg), 'Ready');
  assert.equal(deriveStatus(open('status:in-progress'), cfg), 'In progress');
  assert.equal(deriveStatus(open('status:in-review'), cfg), 'In review');
  assert.equal(deriveStatus(open('status:blocked'), cfg), 'Blocked');
  assert.equal(deriveStatus(open('needs-owner'), cfg), 'Needs owner');
  assert.equal(deriveStatus(open('status:needs-owner'), cfg), 'Needs owner');
});

test('open issue: owner beats blocked beats the delivery labels', () => {
  assert.equal(deriveStatus(open('status:blocked', 'status:ready'), cfg), 'Blocked');
  assert.equal(deriveStatus(open('status:blocked', 'needs-owner'), cfg), 'Needs owner');
  assert.equal(deriveStatus(open('status:in-review', 'status:in-progress'), cfg), 'In review');
  assert.equal(deriveStatus(open('status:ready', 'status:needs-refinement'), cfg), 'Ready');
});

test('owner labels come from the config', () => {
  assert.equal(deriveStatus(open('needs-owner'), { ownerLabels: [] }), 'Inbox');
});

test('closed issue: not planned and duplicate are archived, completed is left to the facts', () => {
  assert.equal(deriveStatus({ state: 'CLOSED', stateReason: 'NOT_PLANNED', labels: [] }, cfg), ARCHIVE);
  assert.equal(deriveStatus({ state: 'CLOSED', stateReason: 'DUPLICATE', labels: [] }, cfg), ARCHIVE);
  assert.equal(deriveStatus({ state: 'CLOSED', stateReason: 'COMPLETED', labels: ['status:ready'] }, cfg), null);
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
