import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkReady, statusChange, sections, gateComment, MARKER } from './check.mjs';

const GOOD = `## Problem

Users cannot export risks.

## Acceptance criteria

1. An export button exists.
2. The CSV holds every column.

## Definition of Done

PR green, live check pasted.
`;
const LABELS = ['type:feature', 'area:backend', 'priority:P2-medium'];
const ok = (over = {}) => ({ body: GOOD, milestone: 'Wave 1', labels: LABELS, ...over });

test('a complete issue passes', () => {
  assert.deepEqual(checkReady(ok()), { ready: true, missing: [] });
});

test('problem: missing or empty fails; French and synonyms pass', () => {
  assert.ok(!checkReady(ok({ body: GOOD.replace('## Problem', '## Notes') })).ready);
  assert.ok(!checkReady(ok({ body: GOOD.replace('Users cannot export risks.', '') })).ready);
  for (const t of ['Problème', 'Goal', 'Outcome', 'Contexte', 'Context']) {
    assert.ok(checkReady(ok({ body: GOOD.replace('## Problem', `## ${t}`) })).ready, t);
  }
});

test('criteria: missing, or not numbered, fails; French heading and 1) pass', () => {
  assert.match(checkReady(ok({ body: GOOD.replace('## Acceptance criteria', '## Plan') })).missing.join(), /Acceptance criteria/);
  const bullets = GOOD.replace('1. An export', '- An export').replace('2. The CSV', '- The CSV');
  assert.match(checkReady(ok({ body: bullets })).missing.join(), /numbered/);
  assert.ok(checkReady(ok({ body: GOOD.replace('## Acceptance criteria', "## Critères d'acceptation") })).ready);
  assert.ok(checkReady(ok({ body: GOOD.replace('1. An export', '1) An export') })).ready);
});

test('definition of done: missing or empty fails', () => {
  assert.ok(!checkReady(ok({ body: GOOD.replace('## Definition of Done', '## Later') })).ready);
  assert.ok(!checkReady(ok({ body: GOOD.replace('PR green, live check pasted.', '<!-- todo -->') })).ready);
});

test('milestone and the three label prefixes, either family', () => {
  assert.match(checkReady(ok({ milestone: null })).missing.join(), /milestone/);
  assert.match(checkReady(ok({ labels: ['area:backend', 'priority:P2'] })).missing.join(), /type:/);
  assert.match(checkReady(ok({ labels: ['type:bug', 'priority:P2'] })).missing.join(), /area:/);
  assert.match(checkReady(ok({ labels: ['type:bug', 'area:backend'] })).missing.join(), /priority:/);
  assert.ok(checkReady(ok({ labels: ['type:bug', 'area:foundation', 'priority:P1'] })).ready);
});

test('headings: level 2 and 3, emphasis stripped, level 1 and 4 ignored', () => {
  assert.deepEqual(sections('# Title\n## **Problem**\nx\n### Scope\ny\n#### deep\nz').map((s) => s.title), ['Problem', 'Scope']);
});

test('status: sets ready or needs-refinement, swaps the other, never touches work in flight', () => {
  assert.deepEqual(statusChange(['type:bug'], true), { add: ['status:ready'], remove: [] });
  assert.deepEqual(statusChange(['status:needs-refinement'], true), { add: ['status:ready'], remove: ['status:needs-refinement'] });
  assert.deepEqual(statusChange(['status:ready'], false), { add: ['status:needs-refinement'], remove: ['status:ready'] });
  assert.deepEqual(statusChange(['status:ready'], true), { add: [], remove: [] });
  for (const s of ['status:in-progress', 'status:in-review', 'status:blocked']) {
    assert.deepEqual(statusChange([s, 'status:ready'], false), { add: [], remove: [] }, s);
  }
});

test('comment: one marker, lists what is missing', () => {
  const c = gateComment({ ready: false, missing: ['a milestone'] });
  assert.ok(c.startsWith(MARKER));
  assert.match(c, /- a milestone/);
  assert.match(gateComment({ ready: true, missing: [] }), /passes/);
});

test('an epic: listed child issues stand for its acceptance criteria', () => {
  const epic = `## Problem\n\nThe console is inconsistent.\n\n## Child issues, in order\n\n- #900 tokens\n- #901 shell\n\n## Definition of done for the epic\n\nEvery child merged.\n`;
  assert.ok(checkReady(ok({ body: epic })).ready);
  assert.ok(!checkReady(ok({ body: epic.replace('- #900 tokens\n- #901 shell', 'to be listed') })).ready);
});

test('an issue from outside the team never becomes Ready; a team issue does', () => {
  for (const a of ['NONE', 'CONTRIBUTOR', 'FIRST_TIME_CONTRIBUTOR', 'FIRST_TIMER']) {
    assert.match(checkReady(ok({ authorAssociation: a })).missing.join(), /maintainer/, a);
  }
  for (const a of ['OWNER', 'MEMBER', 'COLLABORATOR']) assert.ok(checkReady(ok({ authorAssociation: a })).ready, a);
});
