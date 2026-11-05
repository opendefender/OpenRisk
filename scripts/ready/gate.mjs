#!/usr/bin/env node
// The scripted Ready gate (#1249). Sets status:ready or status:needs-refinement from the
// Ready definition of CLAUDE.md, and keeps one comment saying what is missing.
// It never touches status:in-progress, status:in-review or status:blocked.
//
//   node scripts/ready/gate.mjs --issue 123            apply to one issue
//   node scripts/ready/gate.mjs --issue 123 --dry-run  print what it would do
//   node scripts/ready/gate.mjs --all --dry-run         table for every open issue
//   node scripts/ready/gate.mjs --all --promote-only    apply the promotions of that table (labels only, no comment)
//
// Token: BOARD_TOKEN / GH_TOKEN / GITHUB_TOKEN, else `gh auth token` (scripts/board/lib.mjs).

import { gql, restAll, token } from '../board/lib.mjs';
import { checkReady, statusChange, gateComment, MARKER } from './check.mjs';

const args = process.argv.slice(2);
const DRY = args.includes('--dry-run');
const ALL = args.includes('--all');
const PROMOTE = args.includes('--promote-only');
const one = args.includes('--issue') ? Number(args[args.indexOf('--issue') + 1]) : null;
const [OWNER, REPO] = (process.env.GITHUB_REPOSITORY || 'opendefender/OpenRisk').split('/');

async function rest(method, path, body) {
  const res = await fetch(`https://api.github.com${path}`, {
    method,
    headers: { authorization: `bearer ${token()}`, accept: 'application/vnd.github+json', 'user-agent': 'openrisk-ready-gate', 'content-type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok && res.status !== 404) throw new Error(`${method} ${path}: HTTP ${res.status} ${await res.text()}`);
  return res.status === 204 ? null : res.json().catch(() => null);
}

const toIssue = (n) => ({
  number: n.number,
  body: n.body,
  milestone: n.milestone?.title ?? null,
  labels: (n.labels?.nodes ?? n.labels ?? []).map((l) => l.name ?? l),
  authorAssociation: n.author_association ?? n.authorAssociation ?? null,
  author: n.author ?? n.user ?? null,
  isPR: Boolean(n.pull_request),
  state: (n.state || '').toUpperCase(),
});

async function openIssues() {
  const out = [];
  let after = null;
  do {
    const d = await gql(`query($after: String) { repository(owner: "${OWNER}", name: "${REPO}") {
      issues(states: OPEN, first: 100, after: $after) { pageInfo { hasNextPage endCursor }
        nodes { number body authorAssociation author { __typename login } milestone { title } labels(first: 50) { nodes { name } } } } } }`, { after });
    out.push(...d.repository.issues.nodes.map((n) => toIssue({ ...n, state: 'open' })));
    after = d.repository.issues.pageInfo.hasNextPage ? d.repository.issues.pageInfo.endCursor : null;
  } while (after);
  return out;
}

async function apply(issue) {
  const result = checkReady(issue);
  const change = statusChange(issue.labels, result.ready);
  const verdict = { number: issue.number, ready: result.ready, missing: result.missing, ...change };
  if (DRY) return verdict;

  for (const l of change.add) await rest('POST', `/repos/${OWNER}/${REPO}/issues/${issue.number}/labels`, { labels: [l] });
  for (const l of change.remove) await rest('DELETE', `/repos/${OWNER}/${REPO}/issues/${issue.number}/labels/${encodeURIComponent(l)}`);

  // One comment per issue, kept up to date. Written only when the verdict changes, so
  // editing an issue does not notify everyone each time.
  const comments = await restAll(`/repos/${OWNER}/${REPO}/issues/${issue.number}/comments`);
  const mine = comments.find((c) => c.body?.startsWith(MARKER));
  const text = gateComment(result);
  if (mine && mine.body !== text) await rest('PATCH', `/repos/${OWNER}/${REPO}/issues/comments/${mine.id}`, { body: text });
  if (!mine && !result.ready) await rest('POST', `/repos/${OWNER}/${REPO}/issues/${issue.number}/comments`, { body: text });
  return verdict;
}

async function main() {
  if (!token({ required: false })) {
    console.log('::notice::no token; Ready gate skipped.');
    return;
  }
  if (one) {
    const n = await rest('GET', `/repos/${OWNER}/${REPO}/issues/${one}`);
    const issue = toIssue(n);
    if (issue.isPR || issue.state !== 'OPEN') return console.log(`#${one}: not an open issue, skipped`);
    const v = await apply(issue);
    console.log(`#${v.number} ${v.ready ? 'READY' : 'NOT READY'}${v.missing.length ? ' — missing: ' + v.missing.join('; ') : ''}`);
    console.log(`  labels: +[${v.add.join(', ')}] -[${v.remove.join(', ')}]${DRY ? ' (dry run)' : ''}`);
    return;
  }
  if (!ALL) throw new Error('use --issue <n> or --all');
  if (!DRY && !PROMOTE) throw new Error('--all runs with --dry-run, or with --promote-only once the dry run was reviewed (#1249 criterion 6)');
  const issues = await openIssues();
  const verdicts = [];
  for (const i of issues) {
    const result = checkReady(i);
    const change = statusChange(i.labels, result.ready);
    verdicts.push({ number: i.number, ready: result.ready, missing: result.missing, ...change });
    // Bulk mode only promotes, and posts no comment: 200 comments at once would notify
    // the whole team for nothing. Issues that fail get their comment on their next event.
    if (PROMOTE && !DRY && change.add.includes('status:ready')) {
      await rest('POST', `/repos/${OWNER}/${REPO}/issues/${i.number}/labels`, { labels: ['status:ready'] });
      for (const l of change.remove) await rest('DELETE', `/repos/${OWNER}/${REPO}/issues/${i.number}/labels/${encodeURIComponent(l)}`);
    }
  }
  const tally = { ready: 0, notReady: 0, toReady: 0, toRefine: 0, untouched: 0 };
  const reasons = {};
  for (const v of verdicts) {
    v.ready ? tally.ready++ : tally.notReady++;
    if (v.add.includes('status:ready')) tally.toReady++;
    else if (v.add.includes('status:needs-refinement')) tally.toRefine++;
    else tally.untouched++;
    for (const m of v.missing) reasons[m] = (reasons[m] || 0) + 1;
  }
  console.log(`open issues: ${verdicts.length}`);
  console.log(`pass the gate: ${tally.ready} · do not: ${tally.notReady}`);
  console.log(`would become Ready: ${tally.toReady} · would go to Refinement: ${tally.toRefine} · unchanged: ${tally.untouched}`);
  console.log('missing, by item:');
  for (const [m, c] of Object.entries(reasons).sort((a, b) => b[1] - a[1])) console.log(`  ${String(c).padStart(4)}  ${m}`);
  const demoted = verdicts.filter((v) => v.remove.includes('status:ready')).map((v) => `#${v.number}`);
  if (demoted.length) console.log(`Ready → Refinement (${demoted.length}): ${demoted.join(' ')}`);
}

main().catch((e) => {
  console.error(`ready-gate: ${e.message}`);
  process.exit(1);
});
