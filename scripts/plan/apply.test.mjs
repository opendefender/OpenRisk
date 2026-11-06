import { test, beforeEach, afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { runApply, assertClean, verifyFactsHash } from './apply.mjs';
import { createHash } from 'node:crypto';

const GOOD = '## Problem\n\nUsers cannot export risks.\n\n## Acceptance criteria\n\n1. An export button exists.\n\n## Definition of Done\n\nPR green.\n';
const PERSON = { __typename: 'User', login: 'alex' };
const BOTUSER = { login: 'opendefender-delivery-bot[bot]' };
const LABELS = ['type:feature', 'area:backend', 'priority:P2-medium'];
const MS20 = { number: 20, title: 'Autonomous delivery', dueOn: '2026-12-01T00:00:00Z' };
const UPDATED = '2026-11-01T00:00:00Z';
const MARKER = [{ body: '<!-- refined: 2026-11-02 po-openrisk size=S -->', authorAssociation: 'CONTRIBUTOR', author: { __typename: 'Bot', login: 'opendefender-delivery-bot' } }];

const issue = (number, over = {}) => ({
  number, title: `issue ${number}`, body: 'old text', labels: [...LABELS, 'status:needs-refinement'], milestone: MS20,
  authorAssociation: 'OWNER', author: PERSON, createdAt: '2026-10-01T00:00:00Z', updatedAt: UPDATED,
  comments: [], hasOpenPr: false, hasBranch: false, everClaimed: false, mergedPr: false, size: null, sprint: null,
  itemId: `PVTI_${number}`, nodeId: `I_${number}`, ...over,
});
const FACTS = () => ({
  today: '2026-11-08', monday: '2026-11-09',
  sprints: { current: { id: 'IT1', startDate: '2026-11-09', duration: 7 }, next: { id: 'IT2', startDate: '2026-11-16', duration: 7 } },
  board: { projectId: 'PVT_1', size: { fieldId: 'F_SIZE', options: { S: 'o_s', M: 'o_m', L: 'o_l' } }, sprintFieldId: 'F_SPRINT' },
  brief: { number: 1321 }, ownerLabels: ['status:needs-owner'], owner: 'alex', waiting: [], merged: [],
  milestones: [
    { number: 20, title: 'Autonomous delivery', dueOn: '2026-12-01T00:00:00Z', open: 5, closed: 3 },
    { number: 21, title: 'Wave 8', dueOn: null, open: 1, closed: 0 },
  ],
  issues: [
    issue(1),
    issue(2),
    issue(3, { labels: [...LABELS, 'status:ready'], comments: MARKER }),
    issue(4, { labels: [...LABELS, 'status:ready'], comments: MARKER, sprint: { iterationId: 'IT1', startDate: '2026-11-09', duration: 7 } }),
  ],
});
const PLAN = () => ({
  refine: [{ issue: 1, body: GOOD, size: 'M', labels: [] }],
  close: [{ issue: 2, reason: 'duplicate', of: 3, proof: 'same request as #3, facts moved' }],
  sprint: [{ issue: 3, sprint: 'current' }, { issue: 1, sprint: 'next' }],
  move: [{ issue: 4, milestone: 21, reason: 'belongs to the Wave 8 scope' }],
  milestone_due: [{ milestone: 20, due_on: '2026-12-15', reason: 'two weeks of slack for the review' }],
  monday: { start: [{ issue: 3, why: 'unblocks the review gate' }, { issue: 1, why: 'refined today' }, { issue: 4, why: 'already scheduled' }], notes: 'Focus on trust.' },
});
const answer = (plan) => `Thinking about the week.\n\n\`\`\`json\n${JSON.stringify(plan)}\n\`\`\`\n`;

// A GitHub that remembers what was asked of it.
let calls;
let state;
const realFetch = globalThis.fetch;
const json = (x, status = 200) => new Response(JSON.stringify(x), { status, headers: { 'content-type': 'application/json' } });

beforeEach(() => {
  process.env.BOARD_TOKEN = 'test-token';
  calls = [];
  state = { fail: new Set(), drift: new Set(), briefComments: [], dueOn: { 20: '2026-12-01T00:00:00Z', 21: null } };
  globalThis.fetch = async (url, init = {}) => {
    const method = (init.method || 'GET').toUpperCase();
    const u = new URL(url);
    const body = init.body ? JSON.parse(init.body) : null;
    calls.push({ method, path: u.pathname, body });
    if (u.pathname === '/graphql') {
      if (/addProjectV2ItemById/.test(body.query)) return json({ data: { a: { item: { id: 'PVTI_new' } } } });
      return json({ data: { u: { clientMutationId: null } } });
    }
    const m = /^\/repos\/[^/]+\/[^/]+(\/.*)$/.exec(u.pathname);
    const p = m ? m[1] : u.pathname;
    let g;
    if (state.fail.has(`${method} ${p}`)) return json({ message: 'boom' }, 500);
    if (method === 'GET' && (g = /^\/issues\/(\d+)$/.exec(p))) {
      const n = Number(g[1]);
      const f = FACTS().issues.find((i) => i.number === n);
      if (!f) return json({ message: 'nope' }, 404);
      return json({
        number: n, state: 'open', title: f.title, body: f.body, labels: f.labels.map((name) => ({ name })),
        updated_at: state.drift.has(n) ? '2026-11-08T12:00:00Z' : UPDATED, milestone: { number: 20, title: 'Autonomous delivery' },
      });
    }
    if (method === 'GET' && (g = /^\/milestones\/(\d+)$/.exec(p))) return json({ number: Number(g[1]), due_on: state.dueOn[g[1]] });
    if (method === 'GET' && /^\/issues\/1321\/comments$/.test(p)) return json(state.briefComments);
    if (method === 'POST' && p === '/issues') return json({ number: 5000 + calls.filter((c) => c.method === 'POST' && c.path.endsWith('/issues')).length });
    return json({});
  };
});
afterEach(() => { globalThis.fetch = realFetch; });

const gets = () => calls.filter((c) => c.method === 'GET');
const logs = () => { const out = []; return { out, log: (...a) => out.push(a.join(' ')) }; };

test('a dry run reads the live issues, writes nothing, and prints the actions and the Monday comment', async () => {
  const { out, log } = logs();
  const r = await runApply({ facts: FACTS(), answer: answer(PLAN()), dryRun: true, exists: () => false, log });
  assert.equal(r.exitCode, 0);
  assert.deepEqual(calls.filter((c) => c.method !== 'GET'), [], 'no write of any kind');
  assert.ok(gets().some((c) => c.path.endsWith('/issues/1')), 'the live issue was re-read');
  const text = out.join('\n');
  assert.match(text, /dry run/i);
  assert.match(text, /refine #1/);
  assert.match(text, /close #2/);
  assert.match(r.body, /^### Liste du lundi — 2026-11-09/);
  assert.match(r.body, /#3 — issue 3/);
  assert.match(r.body, /2026-12-01 → 2026-12-15/);
  assert.deepEqual(r.refused, []);
});

test('a live run applies each action as asked, keeps the original text, and posts the Monday list once', async () => {
  const { log } = logs();
  const r = await runApply({ facts: FACTS(), answer: answer(PLAN()), dryRun: false, exists: () => false, log });
  assert.equal(r.exitCode, 0);
  const w = calls.filter((c) => c.method !== 'GET');
  const edit = w.find((c) => c.method === 'PATCH' && c.path.endsWith('/issues/1'));
  assert.match(edit.body.body, /Users cannot export risks/);
  assert.match(edit.body.body, /<details><summary>Original text<\/summary>[\s\S]*> old text/);
  const marker = w.find((c) => c.path.endsWith('/issues/1/comments'));
  assert.match(marker.body.body, /^<!-- refined: 2026-11-09 po-openrisk size=M -->/);
  const size = w.filter((c) => c.path === '/graphql').map((c) => c.body.query).find((q) => /F_SIZE/.test(q));
  assert.match(size, /o_m/);
  const sprint = w.filter((c) => c.path === '/graphql').map((c) => c.body.query).find((q) => /F_SPRINT/.test(q));
  assert.match(sprint, /IT1/);
  assert.ok(w.some((c) => c.path.endsWith('/issues/2/comments') && /duplicate of #3/.test(c.body.body)));
  assert.ok(w.some((c) => c.method === 'PATCH' && c.path.endsWith('/issues/2') && c.body.state === 'closed' && c.body.state_reason === 'duplicate'));
  assert.ok(w.some((c) => c.method === 'PATCH' && c.path.endsWith('/issues/4') && c.body.milestone === 21));
  assert.ok(w.some((c) => c.path.endsWith('/issues/4/comments') && /Wave 8 scope/.test(c.body.body)));
  assert.ok(w.some((c) => c.method === 'PATCH' && c.path.endsWith('/milestones/20') && /^2026-12-15T/.test(c.body.due_on)));
  const posted = w.filter((c) => c.path.endsWith('/issues/1321/comments'));
  assert.equal(posted.length, 1);
  assert.match(posted[0].body.body, /^### Liste du lundi — 2026-11-09/);
  assert.ok(!w.some((c) => /status:/.test(JSON.stringify(c.body || {})) && c.path.endsWith('/labels')), 'status labels are never written');
  // Idempotent: the same Monday is not posted twice.
  state.briefComments = [{ user: BOTUSER, body: posted[0].body.body }];
  calls.length = 0;
  const again = await runApply({ facts: FACTS(), answer: answer({ monday: PLAN().monday }), dryRun: false, exists: () => false, log });
  assert.equal(again.exitCode, 0);
  assert.equal(calls.filter((c) => c.method === 'POST' && c.path.endsWith('/issues/1321/comments')).length, 0);
});

test('an issue that changed since the facts was read is left alone, reported, and dropped from the list', async () => {
  const { out, log } = logs();
  state.drift.add(1); // someone commented on #1 after the facts were read
  const r = await runApply({ facts: FACTS(), answer: answer(PLAN()), dryRun: false, exists: () => false, log });
  const w = calls.filter((c) => c.method !== 'GET');
  assert.ok(!w.some((c) => c.path.endsWith('/issues/1') || c.path.endsWith('/issues/1/comments')), 'nothing written on #1');
  assert.ok(w.some((c) => c.method === 'PATCH' && c.path.endsWith('/issues/2')), 'the others still apply');
  assert.ok(r.refused.some((x) => x.ref === 1 && /changed since/.test(x.why)));
  assert.match(out.join('\n'), /refused.*#1/s);
  const monday = w.find((c) => c.path.endsWith('/issues/1321/comments')).body.body;
  assert.ok(!/#1 — issue 1/.test(monday), 'a refine that did not happen does not appear as a start');
  assert.equal(r.exitCode, 0, 'a refusal is reported, not a failure');
});

test('an invalid plan applies nothing, still posts a degraded Monday list, and fails the job', async () => {
  const { log } = logs();
  for (const bad of ['I could not do it, sorry.', answer({ refine: [], destroy: [] }), '', undefined]) {
    calls.length = 0;
    const r = await runApply({ facts: FACTS(), answer: bad, dryRun: false, exists: () => false, log });
    assert.equal(r.exitCode, 1, String(bad).slice(0, 20));
    const w = calls.filter((c) => c.method !== 'GET');
    assert.equal(w.length, 1, 'only the degraded list is written');
    assert.match(w[0].path, /\/issues\/1321\/comments$/);
    assert.match(w[0].body.body, /La planification n'a pas tourné/);
    assert.match(w[0].body.body, /#4 — issue 4/, 'the brief order still lists what is ready');
  }
});

test('with no pinned « Brief du jour » nothing is written at all', async () => {
  const { log } = logs();
  const facts = { ...FACTS(), brief: null };
  await assert.rejects(() => runApply({ facts, answer: answer(PLAN()), dryRun: false, exists: () => false, log }), /Brief du jour/);
  assert.deepEqual(calls.filter((c) => c.method !== 'GET'), []);
});

test('a child issue is created with the parent milestone and the part-of line, then the parent is told', async () => {
  const { log } = logs();
  const plan = { split: [{ parent: 1, children: [{ title: 'Part A', body: GOOD, labels: [] }, { title: 'Part B', body: GOOD, labels: ['area:db'] }] }] };
  const r = await runApply({ facts: FACTS(), answer: answer(plan), dryRun: false, exists: () => false, log });
  assert.equal(r.exitCode, 0);
  const created = calls.filter((c) => c.method === 'POST' && /\/issues$/.test(c.path));
  assert.equal(created.length, 2);
  assert.equal(created[0].body.milestone, 20);
  assert.match(created[0].body.body, /\n\nPart of #1$/);
  assert.deepEqual(created[1].body.labels.sort(), ['area:db', 'priority:P2-medium', 'type:feature']);
  const told = calls.find((c) => c.method === 'POST' && c.path.endsWith('/issues/1/comments'));
  assert.match(told.body.body, /#5001/);
  assert.match(told.body.body, /#5002/);
});

// --- review round 1 (#1326) -------------------------------------------------------------

test('B1: the bot refuses to send a secret-shaped string, whatever hides it, and nothing is written for that issue', async () => {
  for (const t of ['sk-<!---->ant-oat01-abc', 'gh\u0001p_abc', 'ghp_plain']) {
    assert.throws(() => assertClean({ body: { nested: [`x ${t}`] } }), /secret-shaped/);
  }
  assertClean({ body: 'a normal body <!-- comment -->' });
  // The original text of an issue is GitHub's, not the model's: it is quoted into the new body, so it is checked too.
  const facts = FACTS();
  facts.issues[0].body = 'old text with ghp_abcdef0123456789';
  const { out, log } = logs();
  const r = await runApply({ facts, answer: answer(PLAN()), dryRun: false, exists: () => false, log });
  const w = calls.filter((c) => c.method !== 'GET');
  assert.ok(!w.some((c) => c.path.endsWith('/issues/1') || c.path.endsWith('/issues/1/comments')), 'nothing written on #1');
  assert.ok(r.refused.some((x) => x.ref === 1 && /secret/.test(x.why)));
  assert.ok(!out.join('\n').includes('ghp_abcdef'), 'the string is not printed either');
  assert.equal(r.exitCode, 0);
});

test('B1: a refusal in the log carries an integer or a dash, never a string the model chose', async () => {
  const { out, log } = logs();
  const plan = { refine: [{ issue: 'ghp_x', body: 'x', size: 'S' }], sprint: [{ issue: 3, sprint: 'sk-ant-zzz' }] };
  await runApply({ facts: FACTS(), answer: answer(plan), dryRun: true, exists: () => false, log });
  const text = out.join('\n');
  assert.ok(!/ghp_|sk-ant/.test(text), text);
  assert.match(text, /refine -:/);
});

test('M4: when a move fails, the refinement that relied on its milestone is skipped', async () => {
  const facts = FACTS();
  facts.issues[0] = issue(1, { milestone: null });
  state.fail.add('PATCH /issues/1');
  const { log } = logs();
  const plan = { move: [{ issue: 1, milestone: 21, reason: 'belongs to the Wave 8 scope' }], refine: [{ issue: 1, body: GOOD, size: 'S' }] };
  const r = await runApply({ facts, answer: answer(plan), dryRun: false, exists: () => false, log });
  const patches = calls.filter((c) => c.method === 'PATCH' && c.path.endsWith('/issues/1'));
  assert.equal(patches.length, 1, 'only the move was attempted');
  assert.equal(patches[0].body.milestone, 21);
  assert.ok(!calls.some((c) => c.path.endsWith('/issues/1/comments')), 'no marker comment');
  assert.ok(r.refused.some((x) => x.kind === 'refine' && /move/.test(x.why)));
  assert.equal(r.exitCode, 1, 'the failed move is a failure');
});

test('L5: facts.json must hash to what the prepare job recorded', () => {
  const raw = '{"a":1}';
  const sha = createHash('sha256').update(raw).digest('hex');
  verifyFactsHash(raw, sha);
  assert.throws(() => verifyFactsHash(raw + ' ', sha), /does not match/);
  assert.throws(() => verifyFactsHash(raw, ''), /does not match|missing/);
  assert.throws(() => verifyFactsHash(raw, undefined), /missing/);
});

test('M1: the split comment on the parent carries the bot marker', async () => {
  const { log } = logs();
  const plan = { split: [{ parent: 1, children: [{ title: 'Part A', body: GOOD, labels: [] }] }] };
  await runApply({ facts: FACTS(), answer: answer(plan), dryRun: false, exists: () => false, log });
  const told = calls.find((c) => c.method === 'POST' && c.path.endsWith('/issues/1/comments'));
  assert.match(told.body.body, /^<!-- split: 2026-11-09 po-openrisk children=#5001 -->/);
});
