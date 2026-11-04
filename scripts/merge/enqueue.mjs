#!/usr/bin/env node
// Puts a PR into the develop merge queue when, and only when, every condition of
// scripts/merge/conditions.mjs holds on its head commit (#1280, #1251, D-088).
// The merge queue then re-runs the required checks on the merged result and merges.
//
//   node scripts/merge/enqueue.mjs --pr <n> [--dry-run]
//   node scripts/merge/enqueue.mjs --all [--dry-run]      every open PR into develop
//
// Exit 0: enqueued, already queued, or (--all) done. Exit 3: refused, with every unmet
// condition printed. AUTOMERGE_ENABLED=false (the default until the owner switches it on)
// makes every PR refused.

import { gql } from '../board/lib.mjs';
import { R, rest, restAll, statuses, comments, arg, BOT_LOGIN } from '../review/gh.mjs';
import { mayEnqueue } from './conditions.mjs';

const DRY = process.argv.includes('--dry-run');
const ENABLED = (process.env.AUTOMERGE_ENABLED ?? 'false').toLowerCase() === 'true';

async function checkRuns(sha) {
  const out = {};
  let page = 1;
  for (;;) {
    const d = await rest('GET', `${R}/commits/${sha}/check-runs?per_page=100&page=${page}`);
    for (const c of d?.check_runs ?? []) {
      if (!(c.name in out)) out[c.name] = c.status === 'completed' ? c.conclusion : c.status; // newest first
    }
    if (!d || d.check_runs.length < 100) break;
    page += 1;
  }
  return out;
}

async function facts(pr) {
  const p = await rest('GET', `${R}/pulls/${pr}`);
  if (!p) throw new Error(`PR #${pr} not found`);
  const issueNo = /\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)\b/i.exec(p.body || '')?.[1];
  const issue = issueNo ? await rest('GET', `${R}/issues/${issueNo}`) : null;
  return {
    p,
    f: {
      enabled: ENABLED,
      pr: { draft: p.draft, fork: p.head.repo?.full_name !== p.base.repo.full_name, body: p.body, baseRef: p.base.ref },
      contexts: await checkRuns(p.head.sha),
      // D-095: agent-review and Red gate count only when the bot wrote them.
      statuses: await statuses(p.head.sha, { onlyFrom: BOT_LOGIN }),
      issueBody: issue?.body ?? '',
      issueComments: issueNo ? (await comments(issueNo)).map((c) => c.body) : [],
    },
  };
}

async function one(pr) {
  const { p, f } = await facts(pr);
  if (p.state !== 'open') return { pr, result: 'not open' };
  const q = await gql(`query($id: ID!) { node(id: $id) { ... on PullRequest { mergeQueueEntry { state position } } } }`, { id: p.node_id });
  if (q.node?.mergeQueueEntry) return { pr, result: `already queued (${q.node.mergeQueueEntry.state}, position ${q.node.mergeQueueEntry.position})` };
  const v = mayEnqueue(f);
  if (!v.ok) return { pr, result: 'refused', reasons: v.reasons };
  if (DRY) return { pr, result: 'would enqueue' };
  await gql(`mutation($id: ID!) { enqueuePullRequest(input: { pullRequestId: $id }) { mergeQueueEntry { position } } }`, { id: p.node_id });
  return { pr, result: 'enqueued' };
}

const print = (o) => {
  console.log(`#${o.pr}: ${o.result}`);
  for (const r of o.reasons ?? []) console.log(`  - ${r}`);
};

try {
  if (process.argv.includes('--all')) {
    const prs = await restAll(`${R}/pulls?state=open&base=develop`);
    for (const p of prs) print(await one(p.number));
  } else {
    const pr = Number(arg('pr'));
    if (!pr) { console.error('usage: enqueue.mjs --pr <n> | --all [--dry-run]'); process.exit(2); }
    const o = await one(pr);
    print(o);
    if (o.result === 'refused') process.exit(3);
  }
} catch (e) {
  console.error(`enqueue: ${e.message}`);
  process.exit(1);
}
