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

// #1365: views are brought back to their declaration.
test('viewDiff reports only what differs', async () => {
  const { viewDiff } = await import('./setup.mjs');
  const live = { name: 'Board', layout: 'BOARD_LAYOUT', filter: null, fields: { nodes: [{ id: 'a' }, { id: 'b' }] } };
  assert.deepEqual(viewDiff(live, { name: 'Board', layout: 'BOARD_LAYOUT', filter: '', ids: ['a', 'b'] }), []);
  // Order is GitHub's, so it is ignored; a different set is a change; no ids means "leave them".
  assert.deepEqual(viewDiff(live, { name: 'Board', layout: 'BOARD_LAYOUT', filter: '', ids: ['b', 'a'] }), []);
  assert.deepEqual(viewDiff(live, { name: 'Flow', layout: 'BOARD_LAYOUT', filter: '-status:Ready', ids: ['a', 'c'] }), ['name', 'filter', 'fields']);
  assert.deepEqual(viewDiff(live, { name: 'Board', layout: 'ROADMAP_LAYOUT', filter: '', ids: [] }), ['layout']);
});
test('no view shows the fields of the stopped pipeline', async () => {
  const { VIEWS } = await import('./setup.mjs');
  for (const [name, , , visible] of VIEWS) for (const f of ['Risk class', 'Agent', 'Tokens']) assert.ok(!visible.includes(f), `${name} shows ${f}`);
});
