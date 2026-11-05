#!/usr/bin/env node
// The agent runner (#1252, #1270, D-094). One batch per run, started by a systemd user timer
// on the owner's machine (ops/runner/). It decides what to do from GitHub and git alone;
// a model is called only for the work itself.
//
// It never starts an implementation: the owner starts every piece of work with /work (D-101,
// #1320), and the daily brief (scripts/brief/) tells them which. The runner only carries on
// around that work. Order of a batch:
//   1. resume     a fix that an earlier batch did not finish (#1294)
//   2. refine     Refinement issues, and Ready issues without a refinement marker (#1270)
//   3. fix        PRs the review gate sent back (#1271): the bot's agent-review is failure
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
import { pickRefinement, hitUsageLimit, resumeTarget, isIssueBranch } from './select.mjs';

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
          comments(last: 40) { nodes { body authorAssociation } }
          closedByPullRequestsReferences(first: 5) { nodes { state } } } } } }`, { after });
    const page = d.repository.issues;
    for (const n of page.nodes) {
      out.push({
        number: n.number, title: n.title, authorAssociation: n.authorAssociation,
        milestone: n.milestone?.title ?? null, labels: n.labels.nodes.map((l) => l.name),
        comments: n.comments.nodes,
        hasOpenPr: n.closedByPullRequestsReferences.nodes.some((p) => p.state === 'OPEN'),
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
    if (p.draft || p.head.repo?.full_name !== p.base.repo.full_name || !isIssueBranch(p.head.ref)) continue;
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

// ---------- interrupted work (#1294) ----------
// The runner only ever writes in its own fix worktrees, cfg.worktrees/pr-<n>. The /work
// worktrees (../OpenRisk-wt-<n>) belong to sessions the owner started (D-101), and they, the
// owner's checkouts and any other session's worktrees are never committed or pushed.
const fixPath = (pr) => resolve(ROOT, cfg.worktrees, `pr-${pr}`);

// Commits and pushes whatever an interrupted fix left in its worktree, so nothing lives only
// on this machine. `run` is the state's `current` or `resume`: { issue, pr, branch }.
// Returns the branches it saved.
function saveWip(run, reason) {
  const { issue, pr, branch } = run || {};
  const wt = Number.isInteger(pr) ? fixPath(pr) : null;
  if (!Number.isInteger(issue) || !isIssueBranch(branch) || !wt || !existsSync(wt)) return []; // never "all"
  try {
    const g = (args) => execFileSync('git', ['-C', wt, ...args], { encoding: 'utf8' }).trim();
    if (g(['status', '--porcelain'])) {
      g(['add', '-A']);
      g(['commit', '-s', '-q', '-m', `wip(#${issue}): runner stopped — ${reason}; resume`]);
    }
    g(['push', '-q', 'origin', `HEAD:refs/heads/${branch}`]);
    return [{ issue, branch }];
  } catch (e) {
    log({ step: 'wip', issue, pr, error: e.message.split('\n')[0] });
    return [];
  }
}

async function reportWip(saved, reason) {
  for (const s of saved) {
    await rest('POST', `${R}/issues/${s.issue}/comments`, {
      body: `### agent-runner — ${today()}\n**Done** — fix interrupted (${reason}). Everything in the runner's worktree was committed and pushed to \`${s.branch}\`.\n**Next** — the next batch resumes this fix first if the review gate still refuses the PR (#1294).\n**Blocked on** — nothing; resuming automatically.`,
    });
  }
}

// The worktree a fix runs in, under cfg.worktrees, on the tip of the PR's branch. Detached:
// the branch itself is usually checked out in the owner's /work worktree, and git refuses a
// second checkout of it. The agent pushes with `git push origin HEAD:<branch>`.
function fixWorktree(pr, branch) {
  const wt = fixPath(pr);
  execFileSync('git', ['-C', ROOT, 'fetch', '--quiet', 'origin', branch]);
  if (!existsSync(wt)) execFileSync('git', ['-C', ROOT, 'worktree', 'add', '--quiet', '--detach', wt, `origin/${branch}`]);
  else execFileSync('git', ['-C', wt, 'merge', '--quiet', '--ff-only', `origin/${branch}`]);
  return wt;
}

// ---------- the batch ----------
async function batch() {
  const state = readState();
  if (existsSync(join(homedir(), '.config/openrisk-runner/disabled'))) return log({ step: 'batch', skipped: 'kill switch' });
  if (state.backoffUntil && Date.now() < Date.parse(state.backoffUntil)) return log({ step: 'batch', skipped: `usage window, back at ${state.backoffUntil}` });

  if (!DRY) execFileSync('git', ['-C', ROOT, 'fetch', '--quiet', 'origin', cfg.base]);
  const save = (patch) => { if (!DRY) writeFileSync(STATE, JSON.stringify({ ...readState(), ...patch }, null, 2)); };

  // Recovery: the machine stopped mid-run last time (no clean stop was recorded). Save
  // whatever was left on disk, and resume the step that was running.
  if (state.current && !DRY) {
    const saved = saveWip(state.current, 'the machine or the runner stopped');
    await reportWip(saved, 'the machine or the runner stopped');
    save({ resume: state.current, current: null });
    state.resume = state.current;
  }
  const issues = await openIssues();
  const refused = await sentBack();
  // An interrupted fix is resumed only while the review gate still refuses that PR: if the
  // owner fixed it by hand, or the review passed meanwhile, there is nothing to resume.
  const target = resumeTarget(state, issues);
  const resume = target && refused.find((f) => f.pr === target.pr) ? { ...target, branch: refused.find((f) => f.pr === target.pr).branch } : null;
  if (!resume && state.resume) save({ resume: null }); // stale, or left by a runner from before D-101
  const toRefine = pickRefinement(issues, cfg.batch.refine);
  const fixes = refused.filter((f) => f.pr !== resume?.pr).slice(0, cfg.batch.fix);
  const stuck = (await stuckPrs()).slice(0, cfg.batch.steward ? 20 : 0);
  if (resume) log({ step: 'plan', resume: `fix:#${resume.pr}` });
  log({ step: 'plan', refine: toRefine.map((i) => i.number), fix: fixes.map((f) => f.pr), steward: stuck });
  if (DRY) return;

  // Every agent run is bracketed: `current` is set before and cleared after, so a run that
  // never returns (power cut, crash) is found and recovered by the next batch.
  const begin = (step, issue, f) => save({ current: { step, issue, pr: f?.pr, branch: f?.branch, at: new Date().toISOString() } });
  const end = () => save({ current: null });
  const stop = async (r, step, issue, f) => {
    if (!r.limited) return false;
    const until = new Date(Date.now() + cfg.usageBackoffMinutes * 60000).toISOString();
    const run = { step, issue, pr: f?.pr, branch: f?.branch };
    const saved = saveWip(run, 'usage window ran out');
    await reportWip(saved, 'usage window ran out');
    save({ backoffUntil: until, current: null, resume: issue ? run : null });
    log({ step, issue, stopped: 'usage window ran out', backoffUntil: until, saved: saved.map((s) => s.branch) });
    return true;
  };
  // One fix, resumed or new, in the runner's own worktree. A PR whose worktree cannot be
  // prepared is logged and skipped; it does not stop the batch.
  const fix = async (f, name) => {
    let wt;
    try { wt = fixWorktree(f.pr, f.branch); }
    catch (e) { log({ step: name, pr: f.pr, issue: f.issue, error: e.message.split('\n')[0] }); return false; }
    begin('fix', f.issue, f);
    const r = runAgent({ prompt: prompt(name, { PR: f.pr, N: f.issue, BRANCH: f.branch, TURNS: cfg.turns.fix }), model: cfg.models.fix, turns: cfg.turns.fix, tools: cfg.fixTools, cwd: wt });
    end();
    log({ step: name, pr: f.pr, issue: f.issue, ok: r.ok, turns: r.turns, tokens: r.tokens });
    await addTokens(f.issue, r.tokens);
    if (await stop(r, 'fix', f.issue, f)) return true;
    if (r.maxTurns) await blockForBudget(f.issue, `Fixing PR #${f.pr}`);
    return false;
  };

  // An interrupted fix goes first, before any new work.
  if (resume) {
    if (await fix(resume, 'resume')) return;
    save({ resume: null });
  }

  for (const i of toRefine) {
    begin('refine', i.number);
    const r = runAgent({ prompt: prompt('refine', { N: i.number, DATE: today() }), model: cfg.models.refine, turns: cfg.turns.refine, tools: cfg.refineTools, cwd: ROOT });
    end();
    log({ step: 'refine', issue: i.number, ok: r.ok, turns: r.turns, tokens: r.tokens });
    await addTokens(i.number, r.tokens);
    if (await stop(r, 'refine', i.number)) return;
  }
  for (const f of fixes) {
    if (await fix(f, 'fix')) return;
  }
  if (stuck.length && cfg.batch.steward) {
    const r = runAgent({ prompt: `Run one merge-steward pass over these PRs into ${cfg.base}: ${stuck.map((n) => `#${n}`).join(', ')}. Follow .claude/agents/ops/merge-steward.md exactly.`, model: cfg.models.steward, turns: cfg.turns.steward, tools: cfg.stewardTools, cwd: ROOT });
    log({ step: 'steward', prs: stuck, ok: r.ok, turns: r.turns, tokens: r.tokens });
    await stop(r, 'steward', null);
  }
  if (state.backoffUntil) save({ backoffUntil: null });
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
