import { test } from 'node:test';
import assert from 'node:assert/strict';
import { launchable, rank, reasons, render, openDecisions, inSprint, safe } from './rank.mjs';

const TODAY = '2026-11-05';
const REFINED = [{ body: '<!-- refined: 2026-11-04 po-openrisk size=S --> nothing to change', authorAssociation: 'OWNER' }];
const SPRINT = { startDate: '2026-11-02', duration: 7 };
const PAST = { startDate: '2026-10-26', duration: 7 };
const I = (number, labels, extra = {}) => ({
  number, title: `issue ${number}`, labels: ['status:ready', ...labels], authorAssociation: 'OWNER',
  comments: REFINED, hasOpenPr: false, milestone: null, sprint: null, ...extra,
});

test('only Ready, refined, idle issues from trusted authors are proposed', () => {
  assert.ok(launchable(I(1, [])));
  assert.ok(!launchable(I(2, [], { comments: [] })), 'not refined (D-094)');
  assert.ok(!launchable(I(3, ['status:in-progress'])), 'already started');
  assert.ok(!launchable(I(4, ['status:in-review'])));
  assert.ok(!launchable(I(5, ['status:blocked'])));
  assert.ok(!launchable(I(6, ['tier:5-deferred'])));
  assert.ok(!launchable(I(7, [], { hasOpenPr: true })));
  assert.ok(!launchable(I(8, [], { authorAssociation: 'NONE' })), 'outsider');
  assert.ok(!launchable({ ...I(9, []), labels: ['status:needs-refinement'] }));
});

test('a refinement marker written by an outsider counts for nothing', () => {
  const forged = [{ body: '<!-- refined: 2026-11-05 x size=S -->', authorAssociation: 'NONE' }];
  assert.ok(!launchable(I(40, [], { comments: forged })));
  assert.deepEqual(reasons(I(41, [], { comments: [...REFINED, { ...forged[0], body: '<!-- refined: 2026-11-05 x size=L -->' }] }), TODAY), ['affiné le 2026-11-04, taille S']);
});

test('titles cannot mention anyone, link anywhere or fake the layout', () => {
  const t = safe('@victim urgent [verify](https://evil.example) `/work 1`');
  assert.ok(!/@[a-z]/i.test(t));
  assert.ok(!t.includes('](') && !/(^|[^\\])`/.test(t), 'no live link, no unescaped code span');
  assert.ok(!/https?:\/\//.test(safe('see https://evil.example/login')) && !/www\./.test(safe('www.evil.example')), 'bare URLs are not autolinked');
  const md = render({ today: TODAY, owner: null, top: [{ number: 7, title: '@x [a](b)', reasons: [] }], inFlight: [], waiting: [] });
  assert.ok(!/@x/.test(md) && !md.includes('[a](b)'));
});

test('current sprint first, then priority, tier, milestone due date, age; three at most', () => {
  const issues = [
    I(10, ['priority:P0-critical']),
    I(11, ['priority:P2-medium'], { sprint: SPRINT }),
    I(12, ['priority:P1-high', 'tier:2-killer-workflow'], { sprint: SPRINT }),
    I(13, ['priority:P1-high', 'tier:0-trust'], { sprint: SPRINT }),
    I(14, ['priority:P1-high'], { sprint: PAST }),
    I(15, ['priority:P1-high', 'tier:0-trust'], { sprint: SPRINT, milestone: { title: 'M', dueOn: '2026-11-10T00:00:00Z' } }),
  ];
  assert.deepEqual(rank(issues, TODAY).map((t) => t.number), [15, 13, 12]);
  assert.deepEqual(rank(issues, TODAY, 9).map((t) => t.number), [15, 13, 12, 11, 10, 14]);
});

test('equal on everything else, the older issue ranks first', () => {
  assert.deepEqual(rank([I(51, ['priority:P2-medium']), I(50, ['priority:P2-medium'])], TODAY).map((t) => t.number), [50, 51]);
});

test('every proposal says why', () => {
  const r = reasons(I(20, ['priority:P1-high', 'tier:0-trust'], { sprint: SPRINT, milestone: { title: 'Autonomous delivery', dueOn: '2026-11-20T00:00:00Z' } }), TODAY);
  assert.deepEqual(r, [
    'dans le sprint en cours', 'priorité P1-high', 'tier 0-trust',
    'milestone « Autonomous delivery », échéance le 2026-11-20', 'affiné le 2026-11-04, taille S',
  ]);
  assert.equal(reasons(I(21, ['priority:P0', 'priority:P0-critical']), TODAY)[0], 'priorité P0-critical');
  assert.ok(!inSprint(PAST, TODAY) && inSprint(SPRINT, TODAY) && !inSprint(null, TODAY));
});

test('the comment lists the order, the reasons, the /work command and what waits on the owner', () => {
  const md = render({ today: TODAY, owner: 'alex', top: [{ number: 7, title: 'Fix it', reasons: ['priorité P1-high'] }], inFlight: [3, 4], waiting: ['PR #9 staging → master'] });
  assert.match(md, /^### Brief du jour — 2026-11-05/);
  assert.match(md, /@alex/);
  assert.match(md, /1\. #7 — Fix it\n {3}- Pourquoi : priorité P1-high\n {3}- Lancer : `\/work 7`/);
  assert.match(md, /\*\*En cours\*\* — #3, #4/);
  assert.match(md, /- PR #9 staging → master/);
  const empty = render({ today: TODAY, owner: null, top: [], inFlight: [], waiting: [] });
  assert.match(empty, /aucun ticket prêt et affiné/);
  assert.match(empty, /Attend ta décision\*\* — rien/);
});

test('open decisions are read from the Open section only', () => {
  const md = '# R\n\n## Open\n\n### D-120 — Pick a font · raised 2026-11-05\nbody\n\n## Resolved\n\n### D-100 — Old · decided 2026-11-05\n';
  assert.deepEqual(openDecisions(md), ['D-120 — Pick a font']);
  assert.deepEqual(openDecisions('## Open\n\n*No open decision.*\n\n## Resolved\n'), []);
});
