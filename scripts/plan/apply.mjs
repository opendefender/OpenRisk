#!/usr/bin/env node
// Applies the weekly plan (#1326, D-104) as the delivery bot, in the third job of
// .github/workflows/plan.yml. The plan was written by `po-openrisk` in a job that had no
// token; here it is validated (scripts/plan/plan.mjs), each action re-checked against the
// live issue, applied, and the Monday list is posted on « Brief du jour ».
//
//   node scripts/plan/apply.mjs --facts <facts.json> --facts-sha256 <hex> --answer <plan.json> [--dry-run] [--repo-root <dir>]
//
// Only inside GitHub Actions unless --dry-run (which writes nothing, whatever the token).
// Exit 1 when the plan is not valid (a degraded list is posted instead) or a write failed.

import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { createHash } from 'node:crypto';
import { gql, lit } from '../board/lib.mjs';
import { R, rest, restAll, BOT_LOGIN, arg } from '../review/gh.mjs';
import { closeIssue } from '../refine/lib.mjs';
import { parsePlan, validatePlan, renderMonday, markerComment, splitComment, hasSecret } from './plan.mjs';

const labelKey = (labels) => [...labels].sort().join('\n');
const snapshot = (i) => ({ updated: i.updated_at, labels: labelKey(i.labels.map((l) => l.name ?? l)) });

// The last line of defence before a write: no string of what the bot sends may look like a
// secret, as written or once a comment or an invisible character is taken out of it.
export function assertClean(value) {
  if (typeof value === 'string') { if (hasSecret(value)) throw new Error('refusing to send secret-shaped text'); return; }
  if (Array.isArray(value)) { value.forEach(assertClean); return; }
  if (value && typeof value === 'object') Object.values(value).forEach(assertClean);
}

// facts.json travels between jobs as an artifact: it must be the file the prepare job hashed.
export function verifyFactsHash(raw, expected) {
  if (!expected) throw new Error('the facts hash is missing: refusing to apply a plan against unverified facts');
  const actual = createHash('sha256').update(raw).digest('hex');
  if (actual !== String(expected).toLowerCase()) throw new Error('facts.json does not match the hash recorded by the prepare job');
}

// Writes go through here: the secret check is not left to the callers.
const send = (method, path, body) => { if (body) assertClean(body); return rest(method, path, body); };

export async function runApply({ facts, answer, dryRun, exists = () => false, log = console.log }) {
  const target = facts.brief;
  if (!target) throw new Error('no pinned issue « Brief du jour »: the list has nowhere to go, nothing was changed');
  const tag = dryRun ? '[dry run] ' : '';

  // 1. The plan, or why there is none.
  let degraded = null;
  let validated = null;
  if (typeof answer !== 'string' || !answer.trim()) degraded = 'the agent gave no answer';
  else {
    const p = parsePlan(answer);
    if (!p.ok) degraded = p.error;
    else {
      validated = validatePlan(p.plan, facts, { exists });
      if (!validated.ok) degraded = validated.error;
    }
  }

  const refused = validated?.ok ? [...validated.refused] : [];
  const failed = [];
  let applied = null;

  if (!degraded) {
    const A = validated.actions;
    applied = { refine: [], close: [], split: [], sprint: [], move: [], milestone_due: [], monday: A.monday };
    const bad = new Set(); // issues whose action did not happen: they leave the Monday list

    // The live issue must still be what the facts said, or the action is refused.
    const seen = new Map(facts.issues.map((i) => [i.number, { updated: i.updatedAt, labels: labelKey(i.labels) }]));
    const check = async (kind, n) => {
      const i = await rest('GET', `${R}/issues/${n}`);
      if (!i || i.state !== 'open' || i.pull_request) { refused.push({ kind, ref: n, why: `#${n} is not an open issue any more` }); bad.add(n); return false; }
      const now = snapshot(i);
      const was = seen.get(n);
      if (was && (was.updated !== now.updated || was.labels !== now.labels)) {
        refused.push({ kind, ref: n, why: `#${n} changed since the facts were read (updated_at or labels differ)` });
        bad.add(n);
        return false;
      }
      return true;
    };
    // What would be published must be clean; otherwise the action is refused, not failed.
    const clean = (kind, ref, ...strings) => {
      try { assertClean(strings); return true; } catch {
        refused.push({ kind, ref, why: 'secret-shaped text in what would be published' });
        bad.add(ref);
        return false;
      }
    };
    const moveFailed = new Set();
    const msOf = new Map(facts.issues.map((i) => [i.number, i.milestone?.number ?? null]));
    // An action checked against the milestone a move gives it is skipped when that move failed.
    const reliesOnFailedMove = (kind, n, milestone) => {
      if (!moveFailed.has(n) || milestone === msOf.get(n)) return false;
      refused.push({ kind, ref: n, why: `skipped: the move of #${n} to its milestone failed` });
      bad.add(n);
      return true;
    };
    const resnap = async (n) => { if (!dryRun) seen.set(n, snapshot(await rest('GET', `${R}/issues/${n}`))); };
    const attempt = async (kind, ref, fn) => {
      try { await fn(); return true; } catch (e) {
        failed.push({ kind, ref, why: e.message });
        if (Number.isInteger(ref)) bad.add(ref);
        return false;
      }
    };

    // Project fields (Size, Sprint) go through GraphQL, on the board item of the issue.
    const itemOf = async (a) => {
      if (a.itemId) return a.itemId;
      const d = await gql(`mutation { a: addProjectV2ItemById(input: { projectId: ${lit(facts.board.projectId)}, contentId: ${lit(a.nodeId)} }) { item { id } } }`);
      return d.a.item.id;
    };
    const setField = async (itemId, fieldId, value) => {
      await gql(`mutation { u: updateProjectV2ItemFieldValue(input: { projectId: ${lit(facts.board.projectId)}, itemId: ${lit(itemId)}, fieldId: ${lit(fieldId)}, value: { ${value} } }) { clientMutationId } }`);
    };
    const itemFor = new Map(facts.issues.map((i) => [i.number, { itemId: i.itemId, nodeId: i.nodeId }]));

    // 2. Milestones first: the refinements below were checked against them.
    for (const a of A.milestone_due) {
      const m = await rest('GET', `${R}/milestones/${a.milestone}`);
      if (!m || (m.due_on?.slice(0, 10) ?? null) !== a.from) { refused.push({ kind: 'milestone_due', ref: a.milestone, why: 'the milestone changed since the facts were read' }); continue; }
      log(`${tag}milestone « ${a.title} » due ${a.from ?? 'none'} → ${a.due_on}`);
      if (dryRun || await attempt('milestone_due', a.milestone, () => send('PATCH', `${R}/milestones/${a.milestone}`, { due_on: `${a.due_on}T12:00:00Z` }))) applied.milestone_due.push(a);
    }
    for (const a of A.move) {
      if (!clean('move', a.issue, a.reason, a.to.title) || !await check('move', a.issue)) continue;
      log(`${tag}move #${a.issue} « ${a.from ?? 'none'} » → « ${a.to.title} »`);
      if (dryRun || await attempt('move', a.issue, async () => {
        await send('PATCH', `${R}/issues/${a.issue}`, { milestone: a.to.number });
        await send('POST', `${R}/issues/${a.issue}/comments`, { body: `Moved from « ${a.from ?? 'no milestone'} » to « ${a.to.title} » by the weekly planning (#1326).\n\n**Reason** — ${a.reason}` });
        await resnap(a.issue);
      })) applied.move.push(a);
      else if (!dryRun) moveFailed.add(a.issue);
    }
    for (const a of A.refine) {
      if (!clean('refine', a.issue, a.body, a.addLabels) || reliesOnFailedMove('refine', a.issue, a.milestone) || !await check('refine', a.issue)) continue;
      log(`${tag}refine #${a.issue} (size ${a.size}${a.addLabels.length ? `, +${a.addLabels.join(' +')}` : ''}${a.skipped.length ? `, kept ${a.skipped.join(' ')} as they were` : ''})`);
      if (dryRun || await attempt('refine', a.issue, async () => {
        await send('PATCH', `${R}/issues/${a.issue}`, { body: a.body });
        if (a.addLabels.length) await send('POST', `${R}/issues/${a.issue}/labels`, { labels: a.addLabels });
        // Written by bot code only: the marker is what makes the issue count as refined (D-094, D-104).
        await send('POST', `${R}/issues/${a.issue}/comments`, { body: markerComment(facts.monday, a.size) });
        const it = itemFor.get(a.issue);
        const itemId = await itemOf({ itemId: it?.itemId, nodeId: it?.nodeId });
        itemFor.set(a.issue, { ...it, itemId });
        await setField(itemId, facts.board.size.fieldId, `singleSelectOptionId: ${lit(facts.board.size.options[a.size])}`);
        await resnap(a.issue);
      })) applied.refine.push(a);
    }
    for (const a of A.split) {
      if (!clean('split', a.parent, a.children) || reliesOnFailedMove('split', a.parent, a.milestone.number) || !await check('split', a.parent)) continue;
      log(`${tag}split #${a.parent} into ${a.children.length}: ${a.children.map((c) => `« ${c.title} »`).join(', ')}`);
      if (dryRun) { applied.split.push(a); continue; }
      const made = [];
      const ok = await attempt('split', a.parent, async () => {
        for (const c of a.children) {
          const child = await send('POST', `${R}/issues`, { title: c.title, body: c.body, labels: c.labels, milestone: a.milestone.number });
          made.push(child.number);
        }
      });
      if (made.length) {
        await attempt('split', a.parent, async () => {
          await send('POST', `${R}/issues/${a.parent}/comments`, { body: splitComment(facts.monday, made) });
          await resnap(a.parent);
        });
      }
      if (ok) applied.split.push(a);
    }
    for (const a of A.close) {
      if (!clean('close', a.issue, a.proof) || !await check('close', a.issue)) continue;
      log(`${tag}close #${a.issue} (${a.reason}${a.of ? ` of #${a.of}` : ''})`);
      if (dryRun || await attempt('close', a.issue, () => closeIssue(a.issue, a.reason, a.of, a.proof))) applied.close.push(a);
    }
    for (const a of A.sprint) {
      if (!await check('sprint', a.issue)) continue;
      log(`${tag}sprint #${a.issue} → ${a.which}`);
      if (dryRun || await attempt('sprint', a.issue, async () => {
        const it = itemFor.get(a.issue);
        const itemId = await itemOf({ itemId: it?.itemId ?? a.itemId, nodeId: it?.nodeId ?? a.nodeId });
        await setField(itemId, facts.board.sprintFieldId, `iterationId: ${lit(a.iterationId)}`);
      })) applied.sprint.push(a);
    }
    applied.monday = { ...A.monday, start: A.monday.start.filter((s) => !bad.has(s.issue)) };
    for (const s of A.monday.start) if (bad.has(s.issue)) refused.push({ kind: 'monday', ref: s.issue, why: 'left out: an action on it did not happen' });
  }

  // 3. The Monday list, planned or degraded.
  const body = renderMonday({ facts, actions: applied, degraded });
  log(`${tag}Monday comment for ${facts.monday} on #${target.number}:\n${body}`);
  if (!dryRun) {
    const title = `### Liste du lundi — ${facts.monday}`;
    const already = (await restAll(`${R}/issues/${target.number}/comments`)).some((c) => c.user?.login === BOT_LOGIN && c.body.startsWith(title));
    if (already) log(`the list for ${facts.monday} is already on #${target.number}`);
    else {
      try { await send('POST', `${R}/issues/${target.number}/comments`, { body }); } catch (e) { failed.push({ kind: 'monday', ref: '-', why: e.message }); }
    }
  }

  const counts = applied
    ? `refine ${applied.refine.length} · close ${applied.close.length} · split ${applied.split.length} · sprint ${applied.sprint.length} · move ${applied.move.length} · due ${applied.milestone_due.length} · start ${applied.monday.start.length}`
    : 'nothing applied';
  log(`${tag}${counts}`);
  if (refused.length) {
    log(`refused (${refused.length}):`);
    for (const r of refused) log(`  ${r.kind} ${r.ref}: ${r.why}`);
  }
  if (degraded) log(`::error::the plan was not applied: ${degraded}`);
  for (const f of failed) log(`::error::${f.kind} ${f.ref} failed: ${f.why}`);
  return { exitCode: degraded || failed.length ? 1 : 0, body, refused, failed, applied, degraded };
}

async function main() {
  const dryRun = process.argv.includes('--dry-run');
  if (!dryRun && !process.env.GITHUB_ACTIONS) {
    console.error('apply: writes only inside GitHub Actions, as the delivery bot (use --dry-run elsewhere)');
    process.exit(2);
  }
  const factsFile = arg('facts');
  if (!factsFile) { console.error('usage: apply.mjs --facts <facts.json> --facts-sha256 <hex> --answer <plan.json> [--dry-run] [--repo-root <dir>]'); process.exit(2); }
  const root = arg('repo-root') || process.cwd();
  const raw = readFileSync(factsFile, 'utf8');
  try { verifyFactsHash(raw, arg('facts-sha256') || process.env.FACTS_SHA256); } catch (e) { console.error(`apply: ${e.message}`); process.exit(2); }
  const facts = JSON.parse(raw);
  let answer = '';
  try { answer = readFileSync(arg('answer') || '', 'utf8'); } catch { answer = ''; }
  const r = await runApply({ facts, answer, dryRun, exists: (p) => existsSync(join(root, p)) });
  process.exit(r.exitCode);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((e) => { console.error(`apply: ${e.message}`); process.exit(1); });
}
