// The weekly planning (#1326, D-104), as pure functions. No I/O here.
//
// `po-openrisk` runs in a sealed job with no token and no write tool, and answers with one
// JSON plan. validatePlan decides what of it the bot may apply: the five kinds of action of
// the issue's criterion 3 and nothing else. An entry that fails is refused and reported, the
// others still apply; a plan that is not a plan (unknown key, a cap exceeded) applies nothing.
//
// Everything the bot publishes passes through sanitise() first: the model's text is untrusted
// output, and the bot is a trusted writer (its markers count, D-104).

import { checkReady } from '../ready/check.mjs';
import { isTrusted, BOT } from '../ready/trust.mjs';
import { deriveStatus, sprintEnd } from '../board/derive.mjs';
import { isRefined, launchable, rank, reasons as rankReasons, safe } from '../brief/rank.mjs';
import { closeProblem } from '../refine/lib.mjs';

export const CAPS = { refine: 15, close: 5, split: 5, children: 5, start: 10 };
export const MAX_BODY = 60000; // GitHub's limit is 65536
const KEYS = ['refine', 'close', 'split', 'sprint', 'milestone_due', 'move', 'monday'];
const BOT_ACTOR = { __typename: 'Bot', login: BOT };

// The label taxonomy of CLAUDE.md. `status:` is never the agent's: the Ready gate owns it.
const TAXONOMY = {
  'type:': ['feature', 'bug', 'chore', 'security', 'docs', 'design', 'debt'],
  'area:': ['backend', 'frontend', 'infra', 'design', 'marketing', 'docs', 'db'],
  'priority:': ['P0-critical', 'P1-high', 'P2-medium', 'P3-low'],
  'tier:': ['0-trust', '1-product-engine', '2-killer-workflow', '3-ecosystem', '4-ux', '5-deferred'],
};
const prefixOf = (label) => Object.keys(TAXONOMY).find((p) => String(label).startsWith(p));

// --- text the bot publishes -------------------------------------------------------------

const SECRET = /ghs_|ghp_|gho_|ghu_|ghr_|github_pat_|sk-ant-|-----BEGIN|xox[abprs]-|AKIA[0-9A-Z]{16}/;
export const hasSecret = (text) => SECRET.test(String(text ?? ''));

// HTML comments removed (only bot code writes markers), `@` neutralised (no mention, no
// notification), control characters dropped. Unterminated comments lose everything after them.
export function sanitise(text) {
  let t = String(text ?? '').replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/g, '');
  let prev;
  do { prev = t; t = t.replace(/<!--[\s\S]*?-->/g, ''); } while (t !== prev);
  t = t.replace(/<!--[\s\S]*$/, '').replace(/@/g, '@​');
  return { text: t.trim() };
}

// The original body, kept line by line behind `> `, so that no heading of it is a section
// for the Ready check or the criteria parser, and it cannot close its own block.
export function composeBody(newBody, original) {
  const quoted = String(original ?? '').replace(/\r\n?/g, '\n').split('\n')
    .map((l) => `> ${l.replace(/<\/details/gi, '&lt;/details').replace(/<!--/g, '&lt;!--')}`.trimEnd())
    .join('\n');
  return `${String(newBody).trim()}\n\n<details><summary>Original text</summary>\n\n${quoted}\n\n</details>\n`;
}

export const markerComment = (date, size) => `<!-- refined: ${date} po-openrisk size=${size} -->\nRefined by the weekly planning (#1326). The previous text is kept in the issue body, under « Original text ».`;

// --- the answer, the dates ---------------------------------------------------------------

// The agent ends its answer with one fenced JSON block: the plan.
export function parsePlan(text) {
  const blocks = [...String(text || '').matchAll(/```json\s*([\s\S]*?)```/g)];
  if (!blocks.length) return { ok: false, error: 'no fenced json block in the answer' };
  try { return { ok: true, plan: JSON.parse(blocks[blocks.length - 1][1]) }; } catch (e) { return { ok: false, error: `invalid json: ${e.message}` }; }
}

const DAY = 86400e3;
const day = (ms) => new Date(ms).toISOString().slice(0, 10);

// The Monday the list is for: the run's own date when it is a Monday, else the next one.
// Everything of one run (sprints, carry-over, title, idempotency) uses this one date.
export function nextMonday(iso) {
  const ms = Date.parse(`${iso}T00:00:00Z`);
  return day(ms + ((8 - new Date(ms).getUTCDay()) % 7) * DAY);
}

// { current, next } from the Sprint field's iterations, as seen on `monday`.
export function sprintsFor(iterations, monday) {
  const its = [...iterations].sort((a, b) => a.startDate.localeCompare(b.startDate));
  const current = its.find((i) => i.startDate <= monday && sprintEnd(i) > monday) ?? null;
  const after = current ? sprintEnd(current) : monday;
  const next = its.find((i) => (current ? i.startDate >= after : i.startDate > after)) ?? null;
  return { current, next };
}

// --- validation --------------------------------------------------------------------------

const isObj = (v) => v && typeof v === 'object' && !Array.isArray(v);
const shaped = (e, keys, required) => isObj(e) && Object.keys(e).every((k) => keys.includes(k)) && required.every((k) => e[k] !== undefined);
const validDate = (s) => /^\d{4}-\d{2}-\d{2}$/.test(s) && day(Date.parse(`${s}T00:00:00Z`)) === s;

// A string the bot will publish: type-, length- and secret-checked, then sanitised.
function text(v, name, { min = 0, max, oneLine = false }) {
  if (typeof v !== 'string') return { why: `${name} must be a string` };
  if (v.length > max) return { why: `${name} is longer than ${max} characters` };
  if (hasSecret(v)) return { why: `${name} contains a secret-shaped string` };
  let t = sanitise(v).text;
  if (oneLine) t = t.replace(/\s+/g, ' ').trim();
  if (t.length < min) return { why: `${name} needs at least ${min} characters` };
  return { text: t };
}

// → { labels } or { why }. Taxonomy only, one per prefix, no P0 (the owner's call).
function pickLabels(labels, name = 'labels') {
  if (!Array.isArray(labels) || labels.length > 8) return { why: `${name} must be a short list` };
  const seen = new Set();
  for (const l of labels) {
    const p = typeof l === 'string' ? prefixOf(l) : null;
    if (!p || !TAXONOMY[p].includes(l.slice(p.length))) return { why: `label ${JSON.stringify(l)} is not in the taxonomy` };
    if (seen.has(p)) return { why: `more than one ${p} label` };
    if (l.startsWith('priority:P0')) return { why: 'priority:P0-critical is the owner\'s call' };
    seen.add(p);
  }
  return { labels };
}

const FLIGHT = ['status:in-progress', 'status:in-review', 'status:blocked'];
const MOVING = new Set(['In progress', 'In review', 'Blocked', 'Needs owner', 'On dev']);

// Why a refine, a split or a close must not touch this issue, or null.
function frozen(i, facts) {
  if (!isTrusted(i)) return 'its author is not trusted (outside the team)';
  if (facts.brief?.number === i.number) return 'it is the pinned brief';
  if (i.labels.some((l) => FLIGHT.includes(l))) return 'it is in flight (status:in-progress, in-review or blocked)';
  if (i.everClaimed) return 'it has been claimed: its criteria were frozen at the claim (D-103)';
  if (i.hasOpenPr) return 'a PR is open for it';
  if (i.hasBranch) return 'a branch exists for it';
  if ((facts.ownerLabels || []).some((l) => i.labels.includes(l))) return 'it waits on the owner';
  if (i.labels.some((l) => /^priority:P0/.test(l))) return 'it is P0';
  return null;
}

const empty = () => ({ refine: [], close: [], split: [], sprint: [], move: [], milestone_due: [], monday: { start: [], notes: '' } });

// validatePlan(plan, facts, { exists }) → { ok, error?, actions, refused: [{ kind, ref, why }] }
// facts: see scripts/plan/prepare.mjs. exists(path) says whether a path is in the checkout.
export function validatePlan(plan, facts, { exists = () => false } = {}) {
  const fail = (error) => ({ ok: false, error, actions: empty(), refused: [] });
  if (!isObj(plan)) return fail('the plan must be a JSON object');
  const unknown = Object.keys(plan).filter((k) => !KEYS.includes(k));
  if (unknown.length) return fail(`unknown key(s) in the plan: ${unknown.join(', ')}`);
  for (const k of KEYS.filter((x) => x !== 'monday')) {
    if (plan[k] !== undefined && !Array.isArray(plan[k])) return fail(`"${k}" must be a list`);
  }
  if (plan.monday !== undefined && !isObj(plan.monday)) return fail('"monday" must be an object');
  const over = (k, n, cap) => (n > cap ? `too many entries in ${k}: ${n} > ${cap}` : null);
  const start = plan.monday?.start;
  if (start !== undefined && !Array.isArray(start)) return fail('"monday.start" must be a list');
  const capError = over('refine', (plan.refine || []).length, CAPS.refine) || over('close', (plan.close || []).length, CAPS.close) ||
    over('split', (plan.split || []).length, CAPS.split) || over('monday.start', (start || []).length, CAPS.start) ||
    (plan.split || []).map((s) => (Array.isArray(s?.children) ? over('split children', s.children.length, CAPS.children) : null)).find(Boolean);
  if (capError) return fail(capError);

  const actions = empty();
  const refused = [];
  const refuse = (kind, ref, why) => refused.push({ kind, ref, why });
  const byNumber = new Map(facts.issues.map((i) => [i.number, i]));
  const milestones = new Map(facts.milestones.map((m) => [m.number, m]));
  const known = (kind, n) => {
    const i = Number.isInteger(n) ? byNumber.get(n) : null;
    if (!i) refuse(kind, n, `#${n} is not an open issue in the facts`);
    return i;
  };

  // An issue number may appear in only one of refine, close, split, once.
  const counts = new Map();
  for (const n of [...(plan.refine || []).map((e) => e?.issue), ...(plan.close || []).map((e) => e?.issue), ...(plan.split || []).map((e) => e?.parent)]) {
    counts.set(n, (counts.get(n) || 0) + 1);
  }
  const conflicted = (kind, n) => {
    if ((counts.get(n) || 0) > 1) { refuse(kind, n, `#${n} is in more than one of refine, close and split`); return true; }
    return false;
  };
  const closing = new Set((plan.close || []).map((e) => e?.issue));

  // close
  for (const e of plan.close || []) {
    const ref = e?.issue;
    if (!shaped(e, ['issue', 'reason', 'of', 'proof'], ['issue', 'reason', 'proof'])) { refuse('close', ref, 'malformed entry'); continue; }
    const i = known('close', e.issue);
    if (!i || conflicted('close', e.issue)) continue;
    const why0 = frozen(i, facts);
    if (why0) { refuse('close', e.issue, `refused: ${why0}`); continue; }
    const proof = text(e.proof, 'proof', { min: 10, max: 600, oneLine: true });
    if (proof.why) { refuse('close', e.issue, proof.why); continue; }
    const of = e.of === undefined || e.of === null ? null : e.of;
    const problem = closeProblem(e.reason, of, proof.text);
    if (problem) { refuse('close', e.issue, problem); continue; }
    if (e.reason === 'duplicate') {
      if (!Number.isInteger(of) || of === e.issue || !byNumber.has(of) || closing.has(of)) { refuse('close', e.issue, `the kept issue #${of} must be another open issue that stays open`); continue; }
    } else {
      const cited = [...proof.text.matchAll(/#(\d+)/g)].some((m) => byNumber.has(Number(m[1])) || (facts.merged || []).some((p) => p.number === Number(m[1])));
      const paths = [...proof.text.matchAll(/(?<![\w./-])((?:[\w.-]+\/)+[\w.-]+)/g)].map((m) => m[1])
        .filter((p) => !p.includes('..') && !p.startsWith('/'));
      if (!cited && !paths.some((p) => exists(p))) { refuse('close', e.issue, 'a not_planned proof must cite an existing #n or an existing path of the repository'); continue; }
    }
    actions.close.push({ issue: e.issue, reason: e.reason, of: e.reason === 'duplicate' ? of : null, proof: proof.text });
  }

  // move: an issue to another open milestone, with a written reason
  const moves = new Map();
  const moveSeen = new Set();
  for (const e of plan.move || []) {
    const ref = e?.issue;
    if (!shaped(e, ['issue', 'milestone', 'reason'], ['issue', 'milestone', 'reason'])) { refuse('move', ref, 'malformed entry'); continue; }
    const i = known('move', e.issue);
    if (!i) continue;
    if (moveSeen.has(e.issue)) { refuse('move', e.issue, `#${e.issue} is moved twice`); continue; }
    moveSeen.add(e.issue);
    if (!isTrusted(i) || facts.brief?.number === i.number) { refuse('move', e.issue, 'its author is not trusted, or it is the pinned brief'); continue; }
    if (closing.has(e.issue)) { refuse('move', e.issue, 'it is closed by this plan'); continue; }
    const m = milestones.get(e.milestone);
    if (!m) { refuse('move', e.issue, `milestone ${e.milestone} is not an open milestone`); continue; }
    if (i.milestone?.number === m.number) { refuse('move', e.issue, 'it is already in that milestone'); continue; }
    const reason = text(e.reason, 'reason', { min: 10, max: 400, oneLine: true });
    if (reason.why) { refuse('move', e.issue, reason.why); continue; }
    const a = { issue: e.issue, from: i.milestone?.title ?? null, to: { number: m.number, title: m.title }, reason: reason.text };
    moves.set(e.issue, a);
    actions.move.push(a);
  }

  // milestone_due
  const dueSeen = new Set();
  for (const e of plan.milestone_due || []) {
    const ref = e?.milestone;
    if (!shaped(e, ['milestone', 'due_on', 'reason'], ['milestone', 'due_on', 'reason'])) { refuse('milestone_due', ref, 'malformed entry'); continue; }
    const m = milestones.get(e.milestone);
    if (!m) { refuse('milestone_due', ref, `milestone ${ref} is not an open milestone`); continue; }
    if (dueSeen.has(ref)) { refuse('milestone_due', ref, 'the same milestone twice'); continue; }
    dueSeen.add(ref);
    if (typeof e.due_on !== 'string' || !validDate(e.due_on)) { refuse('milestone_due', ref, 'due_on must be a real YYYY-MM-DD date'); continue; }
    const t = Date.parse(`${e.due_on}T00:00:00Z`);
    const today = Date.parse(`${facts.today}T00:00:00Z`);
    if (t <= today || t > today + 366 * DAY) { refuse('milestone_due', ref, 'due_on must be in the future and within a year'); continue; }
    const reason = text(e.reason, 'reason', { min: 10, max: 400, oneLine: true });
    if (reason.why) { refuse('milestone_due', ref, reason.why); continue; }
    actions.milestone_due.push({ milestone: m.number, title: m.title, from: m.dueOn?.slice(0, 10) ?? null, due_on: e.due_on, reason: reason.text });
  }

  // The milestone and labels an issue has once this plan is applied.
  const milestoneAfter = (i) => (moves.has(i.number) ? moves.get(i.number).to : i.milestone);

  // refine
  for (const e of plan.refine || []) {
    const ref = e?.issue;
    if (!shaped(e, ['issue', 'body', 'size', 'labels'], ['issue', 'body', 'size'])) { refuse('refine', ref, 'malformed entry'); continue; }
    const i = known('refine', e.issue);
    if (!i || conflicted('refine', e.issue)) continue;
    const why0 = frozen(i, facts);
    if (why0) { refuse('refine', e.issue, `refused: ${why0}`); continue; }
    if (isRefined(i)) { refuse('refine', e.issue, 'it already carries a refined marker; it is not refined twice'); continue; }
    if (!['S', 'M', 'L'].includes(e.size)) { refuse('refine', e.issue, 'size must be S, M or L'); continue; }
    const picked = pickLabels(e.labels ?? []);
    if (picked.why) { refuse('refine', e.issue, picked.why); continue; }
    const body = text(e.body, 'body', { min: 1, max: 20000 });
    if (body.why) { refuse('refine', e.issue, body.why); continue; }
    const addLabels = picked.labels.filter((l) => !i.labels.some((x) => x.startsWith(prefixOf(l))));
    const skipped = picked.labels.filter((l) => !addLabels.includes(l));
    const full = composeBody(body.text, i.body);
    if (full.length > MAX_BODY) { refuse('refine', e.issue, `the new body with the original text is longer than ${MAX_BODY} characters`); continue; }
    const ready = checkReady({ body: full, milestone: milestoneAfter(i)?.title ?? null, labels: [...i.labels, ...addLabels], authorAssociation: i.authorAssociation, author: i.author });
    if (!ready.ready) { refuse('refine', e.issue, `the body does not pass the Ready check: missing ${ready.missing.join('; ')}`); continue; }
    actions.refine.push({ issue: e.issue, body: full, size: e.size, addLabels, skipped });
  }

  // split
  for (const e of plan.split || []) {
    const ref = e?.parent;
    if (!shaped(e, ['parent', 'children'], ['parent', 'children'])) { refuse('split', ref, 'malformed entry'); continue; }
    const i = known('split', e.parent);
    if (!i || conflicted('split', e.parent)) continue;
    const why0 = frozen(i, facts);
    if (why0) { refuse('split', e.parent, `refused: ${why0}`); continue; }
    if (!Array.isArray(e.children) || !e.children.length) { refuse('split', e.parent, 'a split needs 1 to 5 children'); continue; }
    const ms = milestoneAfter(i);
    const children = [];
    let bad = null;
    e.children.forEach((c, k) => {
      if (bad) return;
      const who = `child ${k + 1}`;
      if (!shaped(c, ['title', 'body', 'labels'], ['title', 'body'])) { bad = `${who} is malformed`; return; }
      const title = text(c.title, `${who} title`, { min: 3, max: 150, oneLine: true });
      const body = text(c.body, `${who} body`, { min: 1, max: 20000 });
      const picked = pickLabels(c.labels ?? [], `${who} labels`);
      const why = title.why || body.why || picked.why;
      if (why) { bad = why.startsWith(who) ? why : `${who}: ${why}`; return; }
      // A child says what it is, and takes the parent's type, area, priority and tier it lacks.
      const labels = [...picked.labels];
      for (const p of Object.keys(TAXONOMY)) {
        if (labels.some((l) => l.startsWith(p))) continue;
        const inherited = i.labels.find((l) => l.startsWith(p));
        if (inherited) labels.push(inherited);
      }
      const full = `${body.text}\n\nPart of #${i.number}`;
      const ready = checkReady({ body: full, milestone: ms?.title ?? null, labels, authorAssociation: 'CONTRIBUTOR', author: BOT_ACTOR });
      if (!ready.ready) { bad = `${who} does not pass the Ready check: missing ${ready.missing.join('; ')}`; return; }
      children.push({ title: title.text, body: full, labels });
    });
    if (bad) { refuse('split', e.parent, bad); continue; }
    actions.split.push({ parent: e.parent, milestone: { number: ms.number, title: ms.title }, children });
  }

  // sprint: current or next, never on work already moving (the board's deriveSprint owns that)
  const sprintSeen = new Set();
  for (const e of plan.sprint || []) {
    const ref = e?.issue;
    if (!shaped(e, ['issue', 'sprint'], ['issue', 'sprint'])) { refuse('sprint', ref, 'malformed entry'); continue; }
    const i = known('sprint', e.issue);
    if (!i) continue;
    if (sprintSeen.has(e.issue)) { refuse('sprint', e.issue, `#${e.issue} is scheduled twice`); continue; }
    sprintSeen.add(e.issue);
    if (!['current', 'next'].includes(e.sprint)) { refuse('sprint', e.issue, 'sprint must be "current" or "next"'); continue; }
    const it = facts.sprints?.[e.sprint];
    if (!it) { refuse('sprint', e.issue, `there is no ${e.sprint} sprint on the board`); continue; }
    if (!isTrusted(i) || facts.brief?.number === i.number) { refuse('sprint', e.issue, 'its author is not trusted, or it is the pinned brief'); continue; }
    if (closing.has(e.issue)) { refuse('sprint', e.issue, 'it is closed by this plan'); continue; }
    const status = deriveStatus({ state: 'OPEN', labels: i.labels, openPr: i.hasOpenPr, hasBranch: i.hasBranch, delivered: i.mergedPr ? 'dev' : null }, { ownerLabels: facts.ownerLabels });
    if (MOVING.has(status)) { refuse('sprint', e.issue, `it is ${status}: the board owns its sprint`); continue; }
    actions.sprint.push({ issue: e.issue, which: e.sprint, iterationId: it.id, itemId: i.itemId ?? null, nodeId: i.nodeId ?? null });
  }

  // monday
  if (plan.monday) {
    const m = plan.monday;
    if (!shaped(m, ['start', 'notes'], [])) { refuse('monday', '-', 'malformed monday block'); }
    else {
      const notes = m.notes === undefined ? { text: '' } : text(m.notes, 'notes', { max: 1500 });
      if (notes.why) refuse('monday', 'notes', notes.why);
      else actions.monday.notes = notes.text;
      const refined = new Map(actions.refine.map((a) => [a.issue, a]));
      const sprints = new Map(actions.sprint.map((a) => [a.issue, a]));
      const closed = new Set(actions.close.map((a) => a.issue));
      const ids = [facts.sprints?.current?.id, facts.sprints?.next?.id].filter(Boolean);
      const seen = new Set();
      for (const s of m.start || []) {
        const ref = s?.issue;
        if (!shaped(s, ['issue', 'why'], ['issue', 'why'])) { refuse('monday', ref, 'malformed entry'); continue; }
        const i = known('monday', s.issue);
        if (!i) continue;
        if (seen.has(s.issue)) { refuse('monday', s.issue, 'listed twice'); continue; }
        seen.add(s.issue);
        const why = text(s.why, 'why', { min: 3, max: 400, oneLine: true });
        if (why.why) { refuse('monday', s.issue, why.why); continue; }
        // The issue as it will be once the plan is applied and the Ready gate has run.
        const r = refined.get(i.number);
        const sim = {
          ...i,
          labels: r ? [...i.labels.filter((l) => l !== 'status:needs-refinement'), ...r.addLabels, 'status:ready'] : i.labels,
          milestone: moves.has(i.number) ? { ...(milestones.get(moves.get(i.number).to.number)) } : i.milestone,
          comments: r ? [...(i.comments || []), { body: `<!-- refined: ${facts.monday} po-openrisk size=${r.size} -->`, authorAssociation: 'CONTRIBUTOR', author: BOT_ACTOR }] : i.comments,
          sprint: sprints.has(i.number) ? facts.sprints[sprints.get(i.number).which] : i.sprint,
        };
        const sid = sim.sprint?.iterationId ?? sim.sprint?.id;
        const problems = [];
        if (closed.has(i.number)) problems.push('closed by this plan');
        else {
          if (!launchable(sim)) problems.push('not Ready and refined, or in flight, or has an open PR');
          if (i.hasBranch) problems.push('a branch exists');
          if (!sid || !ids.includes(sid)) problems.push('not in the current or next sprint');
        }
        if (problems.length) { refuse('monday', s.issue, problems.join('; ')); continue; }
        actions.monday.start.push({ issue: i.number, title: i.title, why: why.text, reasons: rankReasons(sim, facts.monday) });
      }
    }
  }
  return { ok: true, actions, refused };
}

// --- the Monday comment ------------------------------------------------------------------

const line = (s) => safe(String(s).replace(/\s+/g, ' ').trim());

// renderMonday({ facts, actions, degraded }) → the comment the bot posts on « Brief du jour ».
// `actions` is what was applied (null when there is no plan); `degraded` says why there is
// none. Carry-over, milestone health and the owner's list are computed from the facts: the
// model writes only the order and the why.
export function renderMonday({ facts, actions, degraded = null }) {
  const monday = facts.monday;
  const out = [`### Liste du lundi — ${monday}`, ''];
  if (facts.owner) out.push(`@${facts.owner}`, '');
  if (degraded || !actions) {
    out.push(`> **La planification n'a pas tourné** : ${line(degraded || 'pas de plan')}. Rien n'a été modifié ; la liste ci-dessous vient du classement du brief, sans le jugement du modèle.`, '');
  }
  const trusted = facts.issues.filter((i) => isTrusted(i));
  const start = degraded || !actions
    ? rank(trusted, monday, CAPS.start).map((t) => ({ issue: t.number, title: t.title, why: null, reasons: t.reasons }))
    : actions.monday.start;
  if (start.length) {
    out.push(`**À lancer cette semaine, dans l'ordre** (${start.length})`, '');
    start.forEach((t, k) => {
      out.push(`${k + 1}. #${t.issue} — ${line(t.title)}`);
      out.push(`   - Pourquoi : ${t.why ? line(t.why) : line(t.reasons.join(' · ') || 'prêt et affiné')}`);
      if (t.why && t.reasons.length) out.push(`   - Repères : ${line(t.reasons.join(' · '))}`);
      out.push(`   - Lancer : \`/work ${t.issue}\``);
    });
  } else out.push("**À lancer cette semaine** — aucun ticket prêt, affiné et planifié.");
  out.push('');

  const carried = trusted.filter((i) => i.sprint && sprintEnd(i.sprint) <= monday && !i.mergedPr).sort((a, b) => a.number - b.number);
  if (carried.length) {
    out.push(`**Reporté de la semaine passée** (${carried.length})`, '');
    for (const i of carried.slice(0, 15)) out.push(`- #${i.number} — ${line(i.title)}`);
    if (carried.length > 15) out.push(`- … ${carried.length - 15} de plus`);
  } else out.push('**Reporté de la semaine passée** — rien.');
  out.push('');

  out.push('**Jalons**', '');
  for (const m of facts.milestones) {
    const total = m.open + m.closed;
    const pct = total ? Math.round((m.closed / total) * 100) : 0;
    const due = m.dueOn?.slice(0, 10);
    out.push(`- « ${line(m.title)} » — échéance ${due ?? 'aucune'} — ${m.closed}/${total} fermés (${pct} %)${due && due < monday && m.open ? ' — **en retard**' : ''}`);
  }
  if (!facts.milestones.length) out.push('- aucun jalon ouvert');
  out.push('');

  if (actions?.milestone_due.length) {
    out.push('**Échéances modifiées**', '');
    for (const d of actions.milestone_due) out.push(`- « ${line(d.title)} » : ${d.from ?? 'aucune'} → ${d.due_on} — ${line(d.reason)}`);
    out.push('');
  }
  if (actions?.move.length) {
    out.push('**Tickets déplacés de milestone**', '');
    for (const m of actions.move) out.push(`- #${m.issue} : « ${line(m.from ?? 'aucun')} » → « ${line(m.to.title)} » — ${line(m.reason)}`);
    out.push('');
  }
  if (actions) {
    const nums = (a, f) => (a.length ? a.map(f).join(', ') : 'rien');
    out.push(`**Affinage de la semaine** — affinés : ${nums(actions.refine, (a) => `#${a.issue}`)} · fermés : ${nums(actions.close, (a) => `#${a.issue}`)} · découpés : ${nums(actions.split, (a) => `#${a.parent} (${a.children.length})`)} · planifiés : ${nums(actions.sprint, (a) => `#${a.issue}`)}`, '');
    if (actions.monday.notes) out.push('**Notes de po-openrisk**', '', ...actions.monday.notes.split(/\n+/).map((l) => `> ${line(l)}`), '');
  }

  if (facts.waiting?.length) {
    out.push('**Attend ta décision**', '');
    for (const w of facts.waiting) out.push(`- ${w}`);
  } else out.push('**Attend ta décision** — rien.');
  out.push('', "<sub>Plan de po-openrisk validé et appliqué par scripts/plan/ (#1326). Rien ne démarre sans toi.</sub>");
  return out.join('\n');
}
