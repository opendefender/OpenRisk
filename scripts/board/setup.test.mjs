import { test } from 'node:test';
import assert from 'node:assert/strict';
import { needsOptions, optionInput, sprintPlan } from './setup.mjs';

const field = { options: [{ id: 'a1', name: 'Ready' }, { id: 'b2', name: 'Inbox' }] };

test('options: order does not matter, a missing name does', () => {
  assert.equal(needsOptions(field, ['Inbox', 'Ready']), false);
  assert.equal(needsOptions(field, ['Inbox', 'Ready', 'Blocked']), true);
});

test('options: an existing option is sent back with its id, so its values survive', () => {
  assert.match(optionInput(field, 'Ready', 'BLUE', 'x'), /^\{ id: "a1", name: "Ready"/);
  assert.doesNotMatch(optionInput(field, 'Blocked', 'RED', ''), /id:/);
});

test('sprints: weekly from the start, six after the current one', () => {
  const plan = sprintPlan('2026-11-02', '2026-11-04');
  assert.equal(plan[0].startDate, '2026-11-02');
  assert.equal(plan.length, 7);
  assert.ok(plan.every((s) => s.duration === 7));
});
