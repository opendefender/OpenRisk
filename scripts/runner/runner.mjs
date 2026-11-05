#!/usr/bin/env node
// The agent runner (#1252, #1270, D-094). One batch per run, started by a systemd user timer
// on the owner's machine (ops/runner/). It decides what to do from GitHub and git alone;
// a model is called only for the work itself.
//
// Order of a batch:
//   1. refine     Refinement issues, and Ready issues without a refinement marker (#1270)
//   2. fix        PRs the review gate sent back (#1271): the bot's agent-review is failure
//   3. implement  Ready AND refined issues (D-094), by priority, then tier, then age
//   4. steward    one merge-steward pass over PRs that are not moving (#1280)
//
//   node scripts/runner/runner.mjs --dry-run    print the plan, start nothing
//   node scripts/runner/runner.mjs              run one batch
//
// Stops cleanly when the subscription's usage window runs out, and backs off until the next
// tick. A second run while one is active exits at once. Kill switch:
// ~/.config/openrisk-runner/disabled

import { spawnSync, execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, appendFileSync, existsSync, mkdirSync, openSync, closeSync, unlinkSync, statSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gql } from '../board/lib.mjs';
import { R, rest, restAll, statuses, BOT_LOGIN } from '../review/gh.mjs';
import { pickRefinement, pickImplementation, turnBudget, hitUsageLimit, REFINED_MARKER } from './select.mjs';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const cfg = JSON.parse(readFileSync(join(ROOT, 'scripts/runner/config.json'), 'utf8'));
const DRY = process.argv.includes('--dry-run');
const STATE_DIR = join(homedir(), '.local/state/openrisk-runner');
const STATE = join(STATE_DIR, 'state.json');
const LOG = join(STATE_DIR, 'log.jsonl');
const LOCK = join(STATE_DIR, 'lock');
const [OWNER, NAME] = cfg.repo.split('/');
const today = () => new Date().toISOString().slice(0, 10);
const log = (entry) => { console.log(JSON.stringify(entry)); if (!DRY) appendFileSync(LOG, JSON.stringify({ ts: new Date().toISOString(), ...entry }) + '\n'); };
const readState = () => { try { return JSON.parse(readFileSync(STATE, 'utf8')); } catch { return {}; } };

// ---------- facts ----------
async function openIssues() {
  const out = [];
  let after = null;
  do {
    const d = await gql(`query($after: String) { repository(owner: "${OWNER}", name: "${NAME}") {
      issues(states: OPEN, first: 50, after: $after) { pageInfo { hasNextPage endCursor }
        nodes { number title authorAssociation milestone { title }
          labels(first: 50) { nodes { name } }
          comments(last: 40) { nodes { body } }
          closedByPullRequestsReferences(first: 5) { nodes { state } } } } } }`, { after });
    const page = d.repository.issues;
    for (const n of page.nodes) {
      const comments = n.comments.nodes.map((c) => c.body);
      const marker = [...comments].reverse().map((c) => REFINED_MARKER.exec(c) && /size=([SML?])/.exec(c)).find(Boolean);
      out.push({
        number: n.number, title: n.title, authorAssociation: n.authorAssociation,
        milestone: n.milestone?.title ?? null, labels: n.labels.nodes.map((l) => l.name), comments,
        hasOpenPr: n.closedByPullRequestsReferences.nodes.some((p) => p.state === 'OPEN'),
        size: marker?.[1] && marker[1] !== '?' ? marker[1] : undefined,
      });
    }
    after = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (after);
  return out;
}

async function sentBack() {
  const prs = await restAll(`${R}/pulls?state=open&base=${cfg.base}`);
  const out = [];
  for (const p of prs) {
    if (p.draft || p.head.repo?.full_name !== p.base.repo.full_name) continue;
    const st = await statuses(p.head.sha, { onlyFrom: BOT_LOGIN });
    const issue = /\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)\b/i.exec(p.body || '')?.[1];
    if (st['agent-review'] === 'failure' && issue) out.push({ pr: p.number, branch: p.head.ref, issue: Number(issue) });
  }
  return out;
}

async function stuckPrs() {
  const prs = await restAll(`${R}/pulls?state=open&base=${cfg.base}`);
  const out = [];
  for (const p of prs) {
    if (p.draft || p.head.repo?.full_name !== p.base.repo.full_name) continue;
    const full = await rest('GET', `${R}/pulls/${p.number}`);
    if (['behind', 'dirty'].includes(full?.mergeable_state)) out.push(p.number);
  }
  return out;
}

// ---------- doing ----------
function runAgent({ prompt, model, turns, tools, cwd }) {
  const args = ['-p', '--output-format', 'json', '--model', model, '--max-turns', String(turns),
    '--permission-mode', 'acceptEdits', '--allowedTools', tools];
  const r = spawnSync(cfg.cli, args, { cwd, input: prompt, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024, timeout: 3 * 60 * 60 * 1000 });
  let json = {};
  try { json = JSON.parse(r.stdout || '{}'); } catch { json = {}; }
  const text = `${r.stdout || ''}\n${r.stderr || ''}`;
  const usage = json.usage || {};
  return {
    ok: r.status === 0 && !json.is_error,
    limited: hitUsageLimit(text),
    maxTurns: json.subtype === 'error_max_turns',
    turns: json.num_turns ?? null,
    tokens: (usage.input_tokens || 0) + (usage.output_tokens || 0) + (usage.cache_creation_input_tokens || 0),
    result: String(json.result || r.stderr || '').slice(0, 2000),
  };
}

const fill = (tpl, vars) => Object.entries(vars).reduce((s, [k, v]) => s.replaceAll(`{{${k}}}`, String(v)), tpl);
const prompt = (name, vars) => fill(readFileSync(join(ROOT, 'scripts/runner/prompts', `${name}.md`), 'utf8'), vars);

async function addTokens(issue, tokens) {
  if (!tokens) return;
  try {
    const board = JSON.parse(readFileSync(join(ROOT, '.github/board.json'), 'utf8'));
    const field = board.fields?.Tokens?.id;
    if (!field) return;
    const d = await gql(`query { repository(owner: "${OWNER}", name: "${NAME}") { issue(number: ${issue}) {
      projectItems(first: 10) { nodes { id project { id } fieldValueByName(name: "Tokens") { ... on ProjectV2ItemFieldNumberValue { number } } } } } } }`);
    const item = d.repository.issue.projectItems.nodes.find((n) => n.project.id === board.projectId);
    if (!item) return;
    const total = (item.fieldValueByName?.number || 0) + tokens;
    await gql(`mutation { updateProjectV2ItemFieldValue(input: { projectId: "${board.projectId}", itemId: "${item.id}", fieldId: "${field}", value: { number: ${total} } }) { clientMutationId } }`);
  } catch (e) {
    log({ step: 'tokens', issue, error: e.message });
  }
}

async function blockForBudget(issue, what) {
  await rest('POST', `${R}/issues/${issue}/labels`, { labels: ['status:blocked'] });
  await rest('POST', `${R}/issues/${issue}/comments`, {
    body: `### agent-runner — ${today()}\n**Done** — ${what} stopped at its turn budget.\n**Verified** — nothing beyond what the agent pushed.\n**Next** — re-size the issue (refinement) or split it, then remove \`status:blocked\`.\n**Blocked on** — turn budget (#1252 criterion 3).`,
  });
}

// ---------- the batch ----------
async function batch() {
  const state = readState();
  if (existsSync(join(homedir(), '.config/openrisk-runner/disabled'))) return log({ step: 'batch', skipped: 'kill switch' });
  if (state.backoffUntil && Date.now() < Date.parse(state.backoffUntil)) return log({ step: 'batch', skipped: `usage window, back at ${state.backoffUntil}` });

  if (!DRY) execFileSync('git', ['-C', ROOT, 'fetch', '--quiet', 'origin', cfg.base]);
  const issues = await openIssues();
  const toRefine = pickRefinement(issues, cfg.batch.refine);
  const fixes = (await sentBack()).slice(0, cfg.batch.fix);
  const toBuild = pickImplementation(issues, cfg.batch.implement, { milestone: cfg.milestone });
  const stuck = (await stuckPrs()).slice(0, cfg.batch.steward ? 20 : 0);
  log({ step: 'plan', refine: toRefine.map((i) => i.number), fix: fixes.map((f) => f.pr), implement: toBuild.map((i) => `${i.number}:${i.size || 'M'}`), steward: stuck });
  if (DRY) return;

  const stop = (r, step, issue) => {
    if (!r.limited) return false;
    const until = new Date(Date.now() + cfg.usageBackoffMinutes * 60000).toISOString();
    writeFileSync(STATE, JSON.stringify({ ...state, backoffUntil: until, stoppedAt: { step, issue } }, null, 2));
    log({ step, issue, stopped: 'usage window ran out', backoffUntil: until });
    return true;
  };

  for (const i of toRefine) {
    const r = runAgent({ prompt: prompt('refine', { N: i.number, DATE: today() }), model: cfg.models.refine, turns: cfg.turns.refine, tools: cfg.refineTools, cwd: ROOT });
    log({ step: 'refine', issue: i.number, ok: r.ok, turns: r.turns, tokens: r.tokens });
    await addTokens(i.number, r.tokens);
    if (stop(r, 'refine', i.number)) return;
  }
  for (const f of fixes) {
    const wt = resolve(ROOT, cfg.worktrees, `pr-${f.pr}`);
    if (!existsSync(wt)) execFileSync('git', ['-C', ROOT, 'worktree', 'add', '--quiet', wt, f.branch]);
    else execFileSync('git', ['-C', wt, 'pull', '--quiet', '--no-rebase', 'origin', f.branch]);
    const r = runAgent({ prompt: prompt('fix', { PR: f.pr, N: f.issue, TURNS: cfg.turns.fix }), model: cfg.models.fix, turns: cfg.turns.fix, tools: cfg.implementTools, cwd: wt });
    log({ step: 'fix', pr: f.pr, issue: f.issue, ok: r.ok, turns: r.turns, tokens: r.tokens });
    await addTokens(f.issue, r.tokens);
    if (stop(r, 'fix', f.issue)) return;
    if (r.maxTurns) await blockForBudget(f.issue, `Fixing PR #${f.pr}`);
  }
  for (const i of toBuild) {
    const size = i.size || 'M';
    const turns = turnBudget(size, cfg.turns.implement);
    const model = size === 'L' ? cfg.models.large : cfg.models.implement;
    const r = runAgent({ prompt: prompt('implement', { N: i.number, TURNS: turns, SIZE: size }), model, turns, tools: cfg.implementTools, cwd: ROOT });
    log({ step: 'implement', issue: i.number, size, ok: r.ok, turns: r.turns, tokens: r.tokens });
    await addTokens(i.number, r.tokens);
    if (stop(r, 'implement', i.number)) return;
    if (r.maxTurns) await blockForBudget(i.number, `Implementation (size ${size})`);
  }
  if (stuck.length && cfg.batch.steward) {
    const r = runAgent({ prompt: `Run one merge-steward pass over these PRs into ${cfg.base}: ${stuck.map((n) => `#${n}`).join(', ')}. Follow .claude/agents/ops/merge-steward.md exactly.`, model: cfg.models.steward, turns: cfg.turns.steward, tools: cfg.stewardTools, cwd: ROOT });
    log({ step: 'steward', prs: stuck, ok: r.ok, turns: r.turns, tokens: r.tokens });
    stop(r, 'steward', null);
  }
  if (state.backoffUntil) writeFileSync(STATE, JSON.stringify({ ...state, backoffUntil: null }, null, 2));
}

mkdirSync(STATE_DIR, { recursive: true });
let fd = null;
try {
  if (!DRY) {
    if (existsSync(LOCK) && Date.now() - statSync(LOCK).mtimeMs > 4 * 3600 * 1000) unlinkSync(LOCK); // stale
    fd = openSync(LOCK, 'wx');
  }
  await batch();
} catch (e) {
  if (e.code === 'EEXIST') console.log('another batch is running; exiting');
  else { log({ step: 'batch', error: e.message }); process.exitCode = 1; }
} finally {
  if (fd !== null) { closeSync(fd); unlinkSync(LOCK); }
}
