import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  validatePlan, parsePlan, sanitise, hasSecret, nextMonday, sprintsFor, composeBody, renderMonday, CAPS,
} from './plan.mjs';
import { sections, checkReady } from '../ready/check.mjs';

const GOOD = '## Problem\n\nUsers cannot export risks.\n\n## Acceptance criteria\n\n1. An export button exists.\n2. The CSV holds every column.\n\n## Definition of Done\n\nPR green, live check pasted.\n';
const BOT = { __typename: 'Bot', login: 'opendefender-delivery-bot' };
const PERSON = { __typename: 'User', login: 'alex' };
const MS20 = { number: 20, title: 'Autonomous delivery', dueOn: '2026-12-01T00:00:00Z' };
const LABELS = ['type:feature', 'area:backend', 'priority:P2-medium'];

const issue = (number, over = {}) => ({
  number, title: `issue ${number}`, body: 'old text', labels: [...LABELS, 'status:needs-refinement'], milestone: MS20,
  authorAssociation: 'OWNER', author: PERSON, createdAt: '2026-10-01T00:00:00Z', updatedAt: '2026-11-01T00:00:00Z',
  comments: [], hasOpenPr: false, hasBranch: false, everClaimed: false, mergedPr: false, size: null, sprint: null,
  itemId: `PVTI_${number}`, nodeId: `I_${number}`, ...over,
});
const facts = (issues = [issue(1), issue(2), issue(3)], over = {}) => ({
  today: '2026-11-08', monday: '2026-11-09',
  sprints: { current: { id: 'IT1', startDate: '2026-11-09', duration: 7 }, next: { id: 'IT2', startDate: '2026-11-16', duration: 7 } },
  brief: { number: 1321 }, ownerLabels: ['status:needs-owner', 'needs-owner'],
  milestones: [
    { number: 20, title: 'Autonomous delivery', dueOn: '2026-12-01T00:00:00Z', open: 5, closed: 3 },
    { number: 21, title: 'Wave 8', dueOn: null, open: 1, closed: 0 },
  ],
  merged: [{ number: 900, title: 'merged thing' }], waiting: [], owner: 'alex', issues, ...over,
});
const validate = (plan, f = facts(), exists = () => false) => validatePlan(plan, f, { exists });
const refine = (n, over = {}) => ({ issue: n, body: GOOD, size: 'S', labels: [], ...over });
const refusedFor = (r, kind) => r.refused.filter((x) => x.kind === kind);

test('an unknown key refuses the whole plan; so does a non-object', () => {
  const r = validate({ refine: [], delete_everything: [] });
  assert.equal(r.ok, false);
  assert.match(r.error, /delete_everything/);
  assert.equal(validate([]).ok, false);
  assert.equal(validate({ refine: 'nope' }).ok, false);
  assert.equal(validate({}).ok, true);
});

test('every cap refuses the whole plan when exceeded', () => {
  const many = (n, f) => Array.from({ length: n }, (_, k) => f(k + 1));
  assert.deepEqual(CAPS, { refine: 15, close: 5, split: 5, children: 5, start: 10 });
  assert.equal(validate({ refine: many(16, (k) => refine(k)) }).ok, false);
  assert.equal(validate({ close: many(6, (k) => ({ issue: k, reason: 'not_planned', proof: 'see docs/DECISIONS.md' })) }).ok, false);
  assert.equal(validate({ split: many(6, (k) => ({ parent: k, children: [] })) }).ok, false);
  const child = { title: 't', body: GOOD, labels: [] };
  assert.equal(validate({ split: [{ parent: 1, children: many(6, () => child) }] }).ok, false);
  assert.equal(validate({ monday: { start: many(11, (k) => ({ issue: k, why: 'x' })) } }).ok, false);
  assert.equal(validate({ refine: many(15, (k) => refine(k)) }).ok, true, 'at the cap is fine');
});

test('the plan is read from the last fenced json block of the answer', () => {
  assert.equal(parsePlan('no block').ok, false);
  assert.equal(parsePlan('```json\n{not json}\n```').ok, false);
  const r = parsePlan('thinking\n```json\n{"refine": []}\n```\nmore\n```json\n{"close": []}\n```\n');
  assert.deepEqual(r.plan, { close: [] });
});

test('refine, close and split are refused on work in flight, claimed once, with a PR or a branch, by outsiders, for owner labels, P0, the brief', () => {
  const cases = {
    'status:in-progress': { labels: [...LABELS, 'status:in-progress'] },
    'status:in-review': { labels: [...LABELS, 'status:in-review'] },
    'status:blocked': { labels: [...LABELS, 'status:blocked'] },
    'ever claimed': { everClaimed: true },
    'open PR': { hasOpenPr: true },
    'branch': { hasBranch: true },
    'outsider': { authorAssociation: 'NONE', author: { __typename: 'User', login: 'x' } },
    'contributor stranger': { authorAssociation: 'CONTRIBUTOR', author: { __typename: 'User', login: 'x' } },
    'owner label': { labels: [...LABELS, 'status:needs-owner'] },
    'P0': { labels: ['type:bug', 'area:backend', 'priority:P0-critical'] },
    'the brief': { number: 1321 },
  };
  const entries = (n) => ({
    refine: [refine(n)],
    close: [{ issue: n, reason: 'not_planned', proof: 'see docs/DECISIONS.md' }],
    split: [{ parent: n, children: [{ title: 'child', body: GOOD, labels: [] }] }],
  });
  for (const [name, over] of Object.entries(cases)) {
    const n = over.number ?? 1;
    for (const kind of ['refine', 'close', 'split']) {
      // One kind at a time: two kinds on one number would be refused as a conflict, for another reason.
      const r = validate({ [kind]: entries(n)[kind] }, facts([issue(n, over), issue(2)]), () => true);
      assert.equal(r.ok, true, `${name} ${kind}`);
      assert.equal(r.actions[kind].length, 0, `${name} ${kind}`);
      assert.equal(r.refused.length, 1, `${name} ${kind}`);
      assert.doesNotMatch(r.refused[0].why, /more than one/, `${name} ${kind}`);
    }
  }
  // The same entries on an idle issue go through.
  const idle = validate(entries(1), facts(), () => true);
  assert.equal(idle.refused.length, 3, 'three kinds on one number: a conflict');
  for (const kind of ['refine', 'close', 'split']) {
    assert.equal(validate({ [kind]: entries(1)[kind] }, facts(), () => true).actions[kind].length, 1, kind);
  }
});

test('the delivery bot counts as a team author; its split children pass the Ready check', () => {
  const f = facts([issue(1, { authorAssociation: 'CONTRIBUTOR', author: BOT })]);
  const r = validate({
    refine: [refine(1)],
  }, f);
  assert.equal(r.actions.refine.length, 1, 'a bot-authored issue is refined');
  const s = validate({ split: [{ parent: 1, children: [{ title: 'Part A', body: GOOD, labels: [] }] }] }, f);
  assert.equal(s.actions.split.length, 1);
  assert.equal(s.actions.split[0].children.length, 1);
  // A child inherits the parent's milestone and the labels it lacks, and says which issue it comes from.
  const c = s.actions.split[0].children[0];
  assert.equal(s.actions.split[0].milestone.number, 20);
  assert.deepEqual(c.labels.sort(), [...LABELS].sort());
  assert.match(c.body, /\n\nPart of #1$/);
  const stranger = facts([issue(1, { authorAssociation: 'CONTRIBUTOR', author: { __typename: 'User', login: 'opendefender-delivery-bot' } })]);
  assert.equal(validate({ refine: [refine(1)] }, stranger).actions.refine.length, 0, 'a human of the same name is not the bot');
});

test('a number in more than one of refine, close and split refuses every entry for it; the others apply', () => {
  const r = validate({
    refine: [refine(1), refine(2)],
    close: [{ issue: 1, reason: 'not_planned', proof: 'see docs/DECISIONS.md' }],
  }, facts(), () => true);
  assert.deepEqual(r.actions.refine.map((a) => a.issue), [2]);
  assert.equal(r.actions.close.length, 0);
  assert.equal(refusedFor(r, 'refine').length + refusedFor(r, 'close').length, 2);
  const twice = validate({ refine: [refine(1), refine(1)] });
  assert.equal(twice.actions.refine.length, 0, 'the same issue twice in one list is a conflict too');
});

test('every number must come from the facts', () => {
  const r = validate({
    refine: [refine(99)], sprint: [{ issue: 98, sprint: 'current' }],
    move: [{ issue: 97, milestone: 21, reason: 'planning the wave' }],
  });
  assert.equal(r.refused.length, 3);
  assert.ok(r.refused.every((x) => /not an open issue in the facts/.test(x.why)), JSON.stringify(r.refused));
});

test('refine: the body must pass the Ready check, with the milestone a move in the same plan gives', () => {
  const noMs = facts([issue(1, { milestone: null })]);
  const bad = validate({ refine: [refine(1, { body: GOOD.replace('1. An export', '- An export').replace('2. The CSV', '- The CSV') })] });
  assert.equal(bad.actions.refine.length, 0);
  assert.match(bad.refused[0].why, /numbered/);
  const without = validate({ refine: [refine(1)] }, noMs);
  assert.equal(without.actions.refine.length, 0);
  assert.match(without.refused[0].why, /milestone/);
  const withMove = validate({ refine: [refine(1)], move: [{ issue: 1, milestone: 21, reason: 'it belongs to the Wave 8 scope' }] }, noMs);
  assert.equal(withMove.actions.refine.length, 1, 'the move supplies the milestone');
  assert.equal(withMove.actions.move.length, 1);
  const missingLabels = facts([issue(1, { labels: ['status:needs-refinement'] })]);
  assert.equal(validate({ refine: [refine(1)] }, missingLabels).actions.refine.length, 0);
  assert.equal(validate({ refine: [refine(1, { labels: ['type:chore', 'area:infra', 'priority:P3-low'] })] }, missingLabels).actions.refine.length, 1, 'labels added in the same entry count');
});

test('refine: an issue that already carries a trusted marker is not refined again; a forged one does not count', () => {
  const marker = '<!-- refined: 2026-11-01 po-openrisk size=S -->';
  const done = facts([issue(1, { comments: [{ body: marker, authorAssociation: 'CONTRIBUTOR', author: BOT }] })]);
  assert.equal(validate({ refine: [refine(1)] }, done).actions.refine.length, 0);
  const forged = facts([issue(1, { comments: [{ body: marker, authorAssociation: 'NONE', author: PERSON }] })]);
  assert.equal(validate({ refine: [refine(1)] }, forged).actions.refine.length, 1);
});

test('labels: only from the taxonomy, at most one per prefix, never status, never P0, only for a prefix the issue lacks', () => {
  const f = facts([issue(1, { labels: ['type:bug', 'status:needs-refinement'] })]);
  const ok = validate({ refine: [refine(1, { labels: ['area:db', 'priority:P1-high', 'tier:0-trust', 'type:feature'] })] }, f);
  assert.deepEqual(ok.actions.refine[0].addLabels.sort(), ['area:db', 'priority:P1-high', 'tier:0-trust']);
  assert.deepEqual(ok.actions.refine[0].skipped, ['type:feature'], 'type: is already set: never replaced (D-003)');
  for (const bad of [['status:ready'], ['wave:7'], ['area:db', 'area:backend'], ['priority:P0-critical'], ['tier:9-nope'], 'type:bug']) {
    assert.equal(validate({ refine: [refine(1, { labels: bad })] }, f).actions.refine.length, 0, JSON.stringify(bad));
  }
});

test('text the bot publishes: HTML comments stripped, @ neutralised, secrets refuse the entry', () => {
  const marker = '<!-- refined: 2026-11-09 po-openrisk size=S --><!-- ready-gate -->';
  const r = validate({
    close: [{ issue: 1, reason: 'not_planned', proof: `see docs/DECISIONS.md ${marker} thanks @alex-dembele` }],
    refine: [refine(2, { body: `${GOOD}\n<!-- refined: 2026-11-09 x -->\nping @victim\n<!-- unterminated` })],
  }, facts(), () => true);
  const proof = r.actions.close[0].proof;
  assert.ok(!proof.includes('<!--') && !/@[a-z]/i.test(proof), proof);
  const body = r.actions.refine[0].body;
  assert.ok(!body.includes('<!--') && !/@[a-z]/i.test(body));
  for (const secret of ['ghs_abcdefghijklmnopqrstuvwx', 'ghp_x', 'github_pat_11AAA', 'sk-ant-oat01-zzz', '-----BEGIN PRIVATE KEY-----', 'gho_abc']) {
    const s = validate({ refine: [refine(3, { body: `${GOOD}\n${secret}` })] });
    assert.equal(s.actions.refine.length, 0, secret);
    assert.match(s.refused[0].why, /secret/);
  }
  assert.ok(hasSecret('token ghs_123') && !hasSecret('a normal sentence'));
  assert.equal(sanitise('a <!-- x --> b <!-- never closed').text, 'a  b');
  assert.equal(sanitise('<!<!-- x -->-- y -->z').text.includes('<!--'), false);
});

test('the original text is kept quoted, so its headings are not sections and the 60000 limit holds', () => {
  const original = '## Acceptance criteria\n\n- not numbered\n\n## Problem\n\n</details> sneaky\n';
  const body = composeBody(GOOD, original);
  assert.match(body, /<details><summary>Original text<\/summary>/);
  assert.ok(body.split('\n').filter((l) => /^>/.test(l)).length >= 7, 'every original line is quoted');
  assert.deepEqual(sections(body).map((s) => s.title), ['Problem', 'Acceptance criteria', 'Definition of Done']);
  assert.ok(checkReady({ body, milestone: 'M', labels: LABELS }).ready);
  assert.equal((body.match(/<\/details>/g) || []).length, 1, 'the original cannot close the block early');
  const huge = facts([issue(1, { body: 'x'.repeat(61000) })]);
  const r = validate({ refine: [refine(1)] }, huge);
  assert.equal(r.actions.refine.length, 0);
  assert.match(r.refused[0].why, /60000/);
});

test('close: reason and proof as scripts/refine/lib.mjs; a duplicate names an open kept issue; not_planned cites a real #n or path', () => {
  const exists = (p) => p === 'docs/DECISIONS.md';
  const ok = validate({
    close: [
      { issue: 1, reason: 'duplicate', of: 2, proof: 'same request as #2, facts moved' },
      { issue: 3, reason: 'not_planned', proof: 'overturned by docs/DECISIONS.md D-102' },
    ],
  }, facts(), exists);
  assert.equal(ok.actions.close.length, 2);
  assert.equal(ok.actions.close[0].of, 2);
  const bad = validate({
    close: [
      { issue: 1, reason: 'duplicate', proof: 'same request as another' },
      { issue: 2, reason: 'duplicate', of: 2, proof: 'a duplicate of itself, no' },
      { issue: 3, reason: 'duplicate', of: 555, proof: 'duplicate of a closed one' },
      { issue: 1, reason: 'not_planned', proof: 'x' },
    ],
  }, facts(), exists);
  assert.equal(bad.actions.close.length, 0);
  const planned = validate({
    close: [
      { issue: 1, reason: 'not_planned', proof: 'I do not like it very much' },
      { issue: 2, reason: 'not_planned', proof: 'see #4242 which is not there' },
      { issue: 3, reason: 'not_planned', proof: 'see ../../etc/passwd and nothing else' },
    ],
  }, facts(), exists);
  assert.equal(planned.actions.close.length, 0, 'no proof that exists');
  const cites = validate({ close: [{ issue: 1, reason: 'not_planned', proof: 'merged in #900 already' }] }, facts(), exists);
  assert.equal(cites.actions.close.length, 1, 'a merged PR from the facts is a real number');
  const kept = validate({ close: [{ issue: 1, reason: 'duplicate', of: 2, proof: 'same request as #2, facts moved' }, { issue: 2, reason: 'not_planned', proof: 'see docs/DECISIONS.md' }] }, facts(), exists);
  assert.equal(kept.actions.close.length, 1, 'the kept issue cannot be closed in the same plan');
  assert.equal(kept.actions.close[0].issue, 2);
});

test('split: parent gate, 1 to 5 children each passing the Ready check, labels filled from the parent', () => {
  const r = validate({ split: [{ parent: 1, children: [{ title: 'Part A', body: GOOD, labels: ['area:db'] }, { title: 'Part B', body: 'no sections', labels: [] }] }] });
  assert.equal(r.actions.split.length, 0, 'one bad child refuses the whole split');
  assert.match(r.refused[0].why, /child 2/);
  assert.equal(validate({ split: [{ parent: 1, children: [] }] }).actions.split.length, 0);
  const ok = validate({ split: [{ parent: 1, children: [{ title: 'Part A', body: GOOD, labels: ['area:db'] }] }] });
  assert.deepEqual(ok.actions.split[0].children[0].labels.sort(), ['area:db', 'priority:P2-medium', 'type:feature']);
});

test('sprint: current or next, never on work already moving, and the iteration must exist', () => {
  const f = facts([issue(1), issue(2, { hasBranch: true }), issue(3, { labels: [...LABELS, 'status:in-review'] }), issue(4, { labels: [...LABELS, 'status:blocked'] }), issue(5, { labels: [...LABELS, 'status:needs-owner'] })]);
  const r = validate({ sprint: [1, 2, 3, 4, 5].map((n) => ({ issue: n, sprint: 'next' })) }, f);
  assert.deepEqual(r.actions.sprint.map((a) => [a.issue, a.iterationId]), [[1, 'IT2']]);
  assert.equal(r.refused.length, 4);
  assert.equal(validate({ sprint: [{ issue: 1, sprint: 'last' }] }, f).actions.sprint.length, 0);
  assert.equal(validate({ sprint: [{ issue: 1, sprint: 'current' }] }, facts([issue(1)], { sprints: { current: null, next: null } })).actions.sprint.length, 0);
  const closing = validate({ sprint: [{ issue: 1, sprint: 'current' }], close: [{ issue: 1, reason: 'not_planned', proof: 'see docs/DECISIONS.md' }] }, facts(), () => true);
  assert.equal(closing.actions.sprint.length, 0, 'not on an issue closed in the same plan');
});

test('milestones: a due date in the next year, a move to an open milestone with a written reason', () => {
  const r = validate({
    milestone_due: [
      { milestone: 20, due_on: '2026-12-15', reason: 'two weeks of slack for the review' },
      { milestone: 21, due_on: '2026-11-01', reason: 'in the past, refused' },
      { milestone: 21, due_on: '2028-01-01', reason: 'too far away, refused' },
      { milestone: 99, due_on: '2026-12-15', reason: 'unknown milestone' },
      { milestone: 21, due_on: '2026-02-30', reason: 'not a real date' },
    ],
    move: [
      { issue: 1, milestone: 21, reason: 'belongs to the Wave 8 scope' },
      { issue: 2, milestone: 20, reason: 'already there' },
      { issue: 3, milestone: 21, reason: 'short' },
      { issue: 3, milestone: 77, reason: 'a milestone that does not exist' },
    ],
  });
  assert.deepEqual(r.actions.milestone_due.map((a) => [a.milestone, a.due_on]), [[20, '2026-12-15']]);
  assert.equal(r.actions.milestone_due[0].from, '2026-12-01');
  assert.deepEqual(r.actions.move.map((a) => [a.issue, a.to.number]), [[1, 21]]);
  assert.equal(r.actions.move[0].from, 'Autonomous delivery');
  assert.equal(r.refused.length, 7);
});

test('the Monday list: only launchable, refined, scheduled issues, counting what this plan does', () => {
  const marker = [{ body: '<!-- refined: 2026-11-02 po-openrisk size=S -->', authorAssociation: 'CONTRIBUTOR', author: BOT }];
  const ready = (n, over = {}) => issue(n, { labels: [...LABELS, 'status:ready'], comments: marker, sprint: { iterationId: 'IT1', startDate: '2026-11-09', duration: 7 }, ...over });
  const f = facts([
    ready(1),
    ready(2, { comments: [] }),
    ready(3, { sprint: null }),
    ready(4, { hasOpenPr: true }),
    issue(5),
    ready(6, { sprint: { iterationId: 'OLD', startDate: '2026-10-26', duration: 7 } }),
    ready(7, { sprint: null }),
    issue(8),
  ]);
  const r = validate({
    refine: [refine(8)],
    sprint: [{ issue: 3, sprint: 'current' }, { issue: 8, sprint: 'next' }],
    monday: { start: [1, 2, 3, 4, 5, 6, 7, 8].map((n) => ({ issue: n, why: `because ${n}` })), notes: 'a note' },
  }, f);
  assert.deepEqual(r.actions.monday.start.map((s) => s.issue), [1, 3, 8], 'ready+refined (8 once refined and scheduled this run)');
  assert.equal(refusedFor(r, 'monday').length, 5);
  assert.ok(r.actions.monday.start[0].reasons.length > 0);
  const dup = validate({ monday: { start: [{ issue: 1, why: 'first' }, { issue: 1, why: 'second' }] } }, f);
  assert.equal(dup.actions.monday.start.length, 1, 'once only');
});

test('the target date is the Monday after the run, and the sprints follow it', () => {
  assert.equal(nextMonday('2026-11-08'), '2026-11-09', 'Sunday evening');
  assert.equal(nextMonday('2026-11-09'), '2026-11-09', 'on the Monday itself');
  assert.equal(nextMonday('2026-11-05'), '2026-11-09', 'a Thursday dispatch');
  assert.equal(nextMonday('2026-12-31'), '2027-01-04', 'across a year');
  const its = [{ id: 'A', startDate: '2026-11-02', duration: 7 }, { id: 'B', startDate: '2026-11-09', duration: 7 }, { id: 'C', startDate: '2026-11-16', duration: 7 }];
  assert.deepEqual([sprintsFor(its, '2026-11-09').current.id, sprintsFor(its, '2026-11-09').next.id], ['B', 'C'], 'the sprint that starts on the Monday');
  assert.equal(sprintsFor(its, '2026-11-08').current.id, 'A', 'whereas the Sunday itself is still the old sprint');
  assert.deepEqual(sprintsFor([its[0]], '2026-11-16'), { current: null, next: null });
  assert.equal(sprintsFor([its[2]], '2026-11-09').next.id, 'C', 'no current: the next one upcoming');
});

test('the Monday comment: title with the target date, the order, computed carry-over and milestones, moves under their own headings', () => {
  const f = facts([
    issue(1, { title: '@evil [x](http://y) fix', labels: [...LABELS, 'status:ready'] }),
    issue(2, { title: 'carried', sprint: { iterationId: 'OLD', startDate: '2026-11-02', duration: 7 } }),
    issue(3, { title: 'delivered', mergedPr: true, sprint: { iterationId: 'OLD', startDate: '2026-11-02', duration: 7 } }),
  ], { waiting: ['D-120 — Pick a font'] });
  const r = validate({
    move: [{ issue: 1, milestone: 21, reason: 'belongs to the Wave 8 scope' }],
    milestone_due: [{ milestone: 20, due_on: '2026-12-15', reason: 'two weeks of slack for the review' }],
    monday: { start: [], notes: 'Focus on trust.\n@everyone' },
  }, f);
  const md = renderMonday({ facts: f, actions: r.actions, degraded: null });
  assert.match(md, /^### Liste du lundi — 2026-11-09/);
  assert.match(md, /@alex/);
  assert.match(md, /Reporté[\s\S]*#2 — carried/);
  assert.ok(!/#3 — delivered/.test(md), 'delivered work is not carried over');
  assert.match(md, /« Autonomous delivery »[\s\S]*3\/8/);
  assert.match(md, /Échéances modifiées[\s\S]*2026-12-01 → 2026-12-15[\s\S]*two weeks/);
  assert.match(md, /Tickets déplacés[\s\S]*#1[\s\S]*Wave 8[\s\S]*Wave 8 scope/);
  assert.match(md, /D-120 — Pick a font/);
  assert.ok(!/@everyone|@evil/.test(md) && !md.includes('](http'));
});

test('a plan that did not run still gives a degraded Monday list, in brief order, saying so', () => {
  const marker = [{ body: '<!-- refined: 2026-11-02 po-openrisk size=S -->', authorAssociation: 'OWNER', author: PERSON }];
  const f = facts([
    issue(1, { labels: [...LABELS.map((l) => (l === 'priority:P2-medium' ? 'priority:P1-high' : l)), 'status:ready'], comments: marker }),
    issue(2, { labels: [...LABELS, 'status:ready'], comments: marker }),
    issue(3),
  ]);
  const md = renderMonday({ facts: f, actions: null, degraded: 'the agent gave no valid plan' });
  assert.match(md, /^### Liste du lundi — 2026-11-09/);
  assert.match(md, /planification n'a pas tourné.*the agent gave no valid plan/);
  assert.ok(md.indexOf('#1 — issue 1') < md.indexOf('#2 — issue 2'), 'brief order');
  assert.ok(!md.includes('#3 — issue 3\n   - Pourquoi'), 'not refined, not proposed');
  assert.match(md, /`\/work 1`/);
});
