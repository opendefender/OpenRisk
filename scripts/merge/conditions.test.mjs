import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mayEnqueue, latestCriteria, criteriaCount, criteriaOf, exemptCriteria, followUpRefs, bodyAtClaim, REQUIRED_CONTEXTS } from './conditions.mjs';

const ISSUE = '## Problem\n\nx\n\n## Acceptance criteria\n\n1. a\n2. b\n3. c\n\n## Definition of Done\n\nd\n';
const REPORT = '### backend-go — 2026-11-05\n**Done** — x\n**Criteria** — 1 ✅ · 2 ✅ · 3 ✅\n**Next** — none';
const green = () => ({
  enabled: true,
  pr: { draft: false, fork: false, body: 'Closes #42', baseRef: 'develop' },
  contexts: { ...Object.fromEntries(REQUIRED_CONTEXTS.map((c) => [c, 'success'])), 'Risk class': 'success' },
  statuses: { 'agent-review': 'success', 'Red gate': 'success' },
  issueBody: ISSUE,
  issueComments: [REPORT],
});

test('every condition met → may enqueue', () => {
  assert.deepEqual(mayEnqueue(green()), { ok: true, reasons: [] });
});

test('one fixture per refusal', () => {
  const cases = [
    ['kill switch', (f) => { f.enabled = false; }, /switched off/],
    ['fork', (f) => { f.pr.fork = true; }, /fork/],
    ['draft', (f) => { f.pr.draft = true; }, /draft/],
    ['wrong base', (f) => { f.pr.baseRef = 'master'; }, /targets master/],
    ['no closing ref', (f) => { f.pr.body = 'Refs #42'; }, /Closes/],
    ['a required check failing', (f) => { f.contexts['CI Status'] = 'failure'; }, /CI Status" is failure/],
    ['a required check missing', (f) => { delete f.contexts['Secret Detection']; }, /Secret Detection" is missing/],
    ['agent-review pending', (f) => { f.statuses['agent-review'] = 'pending'; }, /agent-review is pending/],
    ['risk class failed', (f) => { f.contexts['Risk class'] = 'failure'; }, /Risk class is failure/],
    ['red gate pending', (f) => { f.statuses['Red gate'] = 'pending'; }, /Red gate is pending/],
    ['a criterion not ✅', (f) => { f.issueComments = ['**Criteria** — 1 ✅ · 2 ❌ (x) · 3 ✅']; }, /2 ❌/],
    ['a criterion not reported', (f) => { f.issueComments = ['**Criteria** — 1 ✅ · 2 ✅']; }, /3 missing/],
    ['no report', (f) => { f.issueComments = ['just a comment']; }, /no progress report/],
    ['no numbered criteria', (f) => { f.issueBody = '## Problem\nx\n'; }, /no numbered/],
  ];
  for (const [name, mutate, re] of cases) {
    const f = green();
    mutate(f);
    const r = mayEnqueue(f);
    assert.equal(r.ok, false, name);
    assert.match(r.reasons.join(' | '), re, name);
  }
});

test('the latest report wins; skipped required checks count as passing', () => {
  const f = green();
  f.issueComments = ['**Criteria** — 1 ❌ · 2 ❌ · 3 ❌', REPORT];
  f.contexts['Frontend Design Tokens'] = 'skipped';
  assert.equal(mayEnqueue(f).ok, true);
});

test('criteria parsing: count and marks', () => {
  assert.equal(criteriaCount(ISSUE), 3);
  assert.equal(criteriaCount("## Critères d'acceptation\n1. a\n2) b\n   3. nested is ignored\n"), 2);
  assert.deepEqual([...latestCriteria(['**Criteria** — 1 ✅ · 2 🟡 · 3 ⏳']).entries()], [[1, '✅'], [2, '🟡'], [3, '⏳']]);
  assert.equal(latestCriteria(['nothing']), null);
});

// #1304: (after merge) criteria.
const body = (third, heading = '## Acceptance criteria') => `## Problem\n\nx\n\n${heading}\n\n1. a\n2. b\n3. ${third}\n\n## Definition of Done\n\nd\n`;
const DEFERRED = 'measured a day later (after merge)';
const deferredPr = () => {
  const f = green();
  f.issueBody = body(DEFERRED);
  f.issueBodyAtClaim = body(DEFERRED);
  f.issueComments = ['**Criteria** — 1 ✅ · 2 ✅ · 3 ⏳'];
  f.pr.body = 'Closes #42\n\nAfter merge: #77\n';
  f.followUps = { 77: 'open' };
  return f;
};

test('after merge: a criterion marked at claim time does not hold the merge; the others still do', () => {
  assert.deepEqual(mayEnqueue(deferredPr()), { ok: true, reasons: [] });
  const f = deferredPr();
  f.issueComments = ['**Criteria** — 1 ✅ · 2 ❌ · 3 ⏳'];
  assert.match(mayEnqueue(f).reasons.join(), /2 ❌/);
  const g = deferredPr();
  g.issueComments = ['**Criteria** — 1 ✅ · 3 ⏳'];
  assert.match(mayEnqueue(g).reasons.join(), /2 missing/);
});

test('after merge: the French spellings count, through mayEnqueue', () => {
  for (const mark of ['(après merge)', '(après le merge)', '(After Merge)']) {
    const f = deferredPr();
    f.issueBody = f.issueBodyAtClaim = body(`mesuré le lendemain ${mark}`, "## Critères d'acceptation");
    assert.equal(mayEnqueue(f).ok, true, mark);
  }
});

test('after merge: a marker added after the claim, or a criterion reworded, still holds the merge', () => {
  const added = deferredPr();
  added.issueBodyAtClaim = body('measured a day later');
  assert.match(mayEnqueue(added).reasons.join(), /3 ⏳ \(marked after merge only after the issue was claimed/);
  const reworded = deferredPr();
  reworded.issueBody = body('something easier (after merge)');
  assert.match(mayEnqueue(reworded).reasons.join(), /3 ⏳/);
  const unknown = deferredPr();
  unknown.issueBodyAtClaim = null;
  assert.equal(mayEnqueue(unknown).ok, false);
});

test('after merge: the marker must end the line', () => {
  const f = deferredPr();
  f.issueBody = f.issueBodyAtClaim = body('the (after merge) path must not be accepted');
  assert.match(mayEnqueue(f).reasons.join(), /3 ⏳/);
  assert.equal(criteriaOf(body('x (after merge)  ')).afterMerge.size, 1, 'trailing spaces are fine');
});

test('after merge: every criterion marked → refused', () => {
  const all = '## Acceptance criteria\n\n1. a (after merge)\n2. b (after merge)\n';
  const f = deferredPr();
  f.issueBody = f.issueBodyAtClaim = all;
  f.issueComments = ['**Criteria** — 1 ⏳ · 2 ⏳'];
  assert.match(mayEnqueue(f).reasons.join(), /every criterion is marked/);
});

test('after merge: a deferred criterion needs an open follow-up issue named in the PR', () => {
  const none = deferredPr();
  none.pr.body = 'Closes #42';
  assert.match(mayEnqueue(none).reasons.join(), /names no open follow-up issue/);
  const closed = deferredPr();
  closed.followUps = { 77: 'closed' };
  assert.match(mayEnqueue(closed).reasons.join(), /names no open follow-up issue/);
  const self = deferredPr();
  self.pr.body = 'Closes #42\n\nAfter merge: #42\n';
  self.followUps = { 42: 'open' };
  assert.match(mayEnqueue(self).reasons.join(), /names no open follow-up issue/);
  const done = deferredPr();
  done.pr.body = 'Closes #42';
  done.issueComments = ['**Criteria** — 1 ✅ · 2 ✅ · 3 ✅'];
  assert.equal(mayEnqueue(done).ok, true, 'met before merge: no follow-up needed');
  assert.deepEqual(followUpRefs('Closes #1\nAfter merge: #9\n after merge: #10 and more'), [9, 10]);
});

test('bodyAtClaim: the revision in force when status:in-progress was first added', () => {
  const labelEvents = [
    { createdAt: '2026-11-01T10:00:00Z', label: 'type:feature' },
    { createdAt: '2026-11-02T10:00:00Z', label: 'status:in-progress' },
    { createdAt: '2026-11-03T10:00:00Z', label: 'status:in-progress' },
  ];
  const edits = [
    { editedAt: '2026-11-04T09:00:00Z', diff: 'v3 (marker added during the work)' },
    { editedAt: '2026-11-01T12:00:00Z', diff: 'v2' },
    { editedAt: '2026-11-01T09:00:00Z', diff: 'v1 original' },
  ];
  assert.equal(bodyAtClaim({ body: 'v3', edits, totalEdits: 3, labelEvents }), 'v2');
  assert.equal(bodyAtClaim({ body: 'only', edits: [], totalEdits: 0, labelEvents }), 'only', 'never edited');
  assert.equal(bodyAtClaim({ body: 'v3', edits: edits.slice(0, 2), totalEdits: 3, labelEvents }), 'v2', 'a later page is not needed when found');
  assert.equal(bodyAtClaim({ body: 'v3', edits: edits.slice(0, 1), totalEdits: 3, labelEvents }), null, 'history not fully read');
  assert.equal(bodyAtClaim({ body: 'v3', edits, totalEdits: 3, labelEvents: labelEvents.slice(0, 1) }), null, 'never claimed');
  const deleted = [{ editedAt: '2026-11-04T09:00:00Z', diff: 'v3' }, { editedAt: '2026-11-01T12:00:00Z', diff: null }];
  assert.equal(bodyAtClaim({ body: 'v3', edits: deleted, totalEdits: 2, labelEvents }), null, 'revision deleted');
});

test('exemptCriteria: same number and same line at claim time only', () => {
  assert.deepEqual([...exemptCriteria(body(DEFERRED), body(DEFERRED))], [3]);
  assert.deepEqual([...exemptCriteria(body(DEFERRED), body('other (after merge)'))], []);
  assert.deepEqual([...exemptCriteria(body(DEFERRED), null)], []);
});
