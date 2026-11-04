#!/usr/bin/env node
// Routes the reviewers of a PR's head commit (#1271) and records them in one comment.
// It sets `agent-review` to pending, and the reviewers then record their verdicts with
// verdict.mjs.
//
//   node scripts/review/route.mjs --pr <n> --worktree <dir with the PR head checked out> [--dry-run]
//
// Everything that runs is this checkout's code: the PR's commits are only read by git in
// <dir>. The reviewers come from .claude/hooks/route-reviewers.sh (#1005) and from the
// risk class (#1250); nothing is chosen by an agent.

import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { requiredRoles, requiredComment, REQUIRED_MARKER, closingIssue } from './lib.mjs';
import { classify, parseDiff } from '../risk/classify.mjs';
import { R, rest, botComments, arg } from './gh.mjs';
import { aggregatePr } from './aggregate.mjs';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const pr = Number(arg('pr'));
const wt = arg('worktree');
if (!pr || !wt) { console.error('usage: route.mjs --pr <n> --worktree <dir>'); process.exit(2); }

const p = await rest('GET', `${R}/pulls/${pr}`);
if (!p || p.state !== 'open') { console.log(`#${pr}: not open, skipped`); process.exit(0); }
const sha = p.head.sha;
const base = p.base.sha;
const git = (args) => execFileSync('git', ['-c', 'core.fsmonitor=false', ...args], { cwd: wt, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 });

const head = git(['rev-parse', 'HEAD']).trim();
if (head !== sha) { console.error(`route: worktree is at ${head.slice(0, 8)}, PR head is ${sha.slice(0, 8)}`); process.exit(2); }

// Labels of the closing issue feed route-reviewers.sh (it must not read them from a branch name).
const issue = closingIssue(p.body);
const labels = issue ? ((await rest('GET', `${R}/issues/${issue}`))?.labels ?? []).map((l) => l.name).join(',') : '';

let routed = [];
const reasons = [];
try {
  const out = execFileSync('bash', [join(ROOT, '.claude/hooks/route-reviewers.sh'), base, '--labels', labels], { cwd: wt, encoding: 'utf8' });
  routed = out.split('\n').map((s) => s.trim()).filter(Boolean);
  if (routed.length) reasons.push(`routed by the diff and labels: ${routed.join(', ')}`);
} catch (e) {
  reasons.push(`route-reviewers.sh failed (${String(e.message).split('\n')[0]}): devsecops added, failing closed`);
  routed = ['devsecops'];
}

let cls = 'red';
let files = [];
try {
  const mb = git(['merge-base', base, sha]).trim();
  const diff = parseDiff(git(['diff', '--no-ext-diff', '--no-textconv', '--no-renames', '-U0', `${mb}..${sha}`]));
  files = diff.map((f) => f.path);
  const rules = JSON.parse(readFileSync(join(ROOT, 'scripts/risk/rules.json'), 'utf8'));
  const fork = p.head.repo?.full_name !== `${p.base.repo.owner.login}/${p.base.repo.name}`;
  const v = classify({ files: diff, fork }, rules);
  cls = v.cls;
  reasons.push(`risk class: ${cls}${v.reasons.length ? ` (${v.reasons.slice(0, 3).join('; ')}${v.reasons.length > 3 ? '; …' : ''})` : ''}`);
} catch (e) {
  reasons.push(`risk class unreadable (${String(e.message).split('\n')[0]}): treated as red`);
}

const roles = requiredRoles({ routed, cls, files });
const text = requiredComment(sha, roles, reasons);
if (process.argv.includes('--dry-run')) {
  console.log(text);
  process.exit(0);
}
const mine = (await botComments(pr)).filter((c) => c.body?.includes(REQUIRED_MARKER));
if (!mine.some((c) => c.body === text)) await rest('POST', `${R}/issues/${pr}/comments`, { body: text });
const agg = await aggregatePr(pr);
console.log(`#${pr} ${sha.slice(0, 8)}: required ${roles.join(', ')} → agent-review ${agg.state}`);
