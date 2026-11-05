#!/usr/bin/env node
// Carries the review gate into the merge queue (#1311). For every PR of a merge group, the
// bot's `agent-review` and `Red gate` statuses on that PR's head commit must be success.
// The verdict is then written, as the bot, on the merge-group commit, where the ruleset
// requires it. A PR queued by hand without the agents' approval is ejected.
//
//   node scripts/review/queue-gate.mjs --sha <merge group head sha> [--dry-run]

import { gql } from '../board/lib.mjs';
import { R, rest, statuses, setStatus, arg, BOT_LOGIN, OWNER, REPO } from './gh.mjs';

// The PRs a merge-group commit contains: the entry with that head and every entry ahead.
export function groupPrs(entries, sha) {
  const mine = entries.find((e) => e.headCommit?.oid === sha);
  if (!mine) return null;
  return entries.filter((e) => e.position <= mine.position).map((e) => e.pullRequest.number);
}

// verdicts: [{ pr, review, red }] → { state, description }
export function groupVerdict(verdicts) {
  if (!verdicts.length) return { state: 'failure', description: 'no PR found for this merge group' };
  const bad = verdicts.filter((v) => v.review !== 'success' || v.red !== 'success');
  if (!bad.length) return { state: 'success', description: `bot approved ${verdicts.map((v) => `#${v.pr}`).join(', ')}` };
  return { state: 'failure', description: `not approved by the review gate: ${bad.map((v) => `#${v.pr} (agent-review ${v.review || 'missing'}, Red gate ${v.red || 'missing'})`).join('; ')}`.slice(0, 140) };
}

if (process.argv[1]?.endsWith('queue-gate.mjs')) {
  const sha = arg('sha');
  if (!/^[0-9a-f]{40}$/.test(sha || '')) { console.error('usage: queue-gate.mjs --sha <40-hex>'); process.exit(2); }
  const d = await gql(`query { repository(owner: "${OWNER}", name: "${REPO}") { mergeQueue(branch: "develop") {
    entries(first: 50) { nodes { position headCommit { oid } pullRequest { number headRefOid } } } } } }`);
  const entries = d.repository.mergeQueue?.entries.nodes ?? [];
  const prs = groupPrs(entries, sha);
  let v;
  if (!prs) v = { state: 'failure', description: 'this commit is not a merge group of the develop queue' };
  else {
    const verdicts = [];
    for (const n of prs) {
      const p = await rest('GET', `${R}/pulls/${n}`);
      const st = await statuses(p.head.sha, { onlyFrom: BOT_LOGIN });
      verdicts.push({ pr: n, review: st['agent-review'], red: st['Red gate'] });
    }
    v = groupVerdict(verdicts);
  }
  console.log(`${sha.slice(0, 8)}: ${v.state} — ${v.description}`);
  if (!process.argv.includes('--dry-run')) {
    await setStatus(sha, 'agent-review', v.state, v.description);
    await setStatus(sha, 'Red gate', v.state, v.state === 'success' ? 'every PR in the group passed its Red gate' : v.description);
  }
}
