import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mayEnqueue, latestCriteria, criteriaCount, REQUIRED_CONTEXTS } from './conditions.mjs';

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
