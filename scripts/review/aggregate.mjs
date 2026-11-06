#!/usr/bin/env node
// Recomputes the `agent-review` status of a PR's head commit from the per-role statuses
// (#1271), and acts on a failure: send back to an implementer, or block after the last
// round. Safe to run any number of times.
//
//   node scripts/review/aggregate.mjs --pr <n> [--dry-run]

import { aggregate, parseRequired, rounds, afterAggregate, closingIssue, MAX_ROUNDS } from './lib.mjs';
import { R, rest, setStatus, statuses, botComments, swapLabels, arg, BOT_LOGIN } from './gh.mjs';

export async function aggregatePr(pr, { dry = false } = {}) {
  const p = await rest('GET', `${R}/pulls/${pr}`);
  if (!p || p.state !== 'open') return { pr, skipped: 'not open' };
  const sha = p.head.sha;
  const bodies = (await botComments(pr)).map((c) => c.body);
  const req = parseRequired(bodies);
  const st = await statuses(sha, { onlyFrom: BOT_LOGIN }); // D-095: only the bot's statuses count

  let result;
  if (!req || req.sha !== sha) result = { state: 'pending', description: 'reviewers not routed for this commit yet' };
  else {
    const per = Object.fromEntries(req.roles.map((r) => [r, st[`agent-review/${r}`]]));
    result = aggregate(req.roles, per);
  }
  const issue = closingIssue(p.body);
  const failed = rounds(bodies);
  const next = afterAggregate(result.state, failed);
  const out = { pr, sha, issue, ...result, rounds: failed, action: next.action };
  if (dry) return out;

  if (st['agent-review'] !== result.state) await setStatus(sha, 'agent-review', result.state, result.description);
  // A red-class PR (devsecops required) passes its Red gate (#1250) once the stricter review
  // passed: devsecops and tech-lead both approved this commit, on top of the required checks.
  if (req && req.sha === sha && req.roles.includes('devsecops') && req.roles.includes('tech-lead')) {
    const red = result.state === 'success' ? 'success' : 'pending';
    if (st['Red gate'] !== red) {
      await setStatus(sha, 'Red gate', red, red === 'success'
        ? 'red class: devsecops and tech-lead approved this commit'
        : 'red class: waiting for devsecops + tech-lead review (#1271)');
    }
  }

  // Act once per commit: the marker in the comment makes a rerun a no-op.
  const marker = `<!-- agent-review:after sha=${sha} action=${next.action} -->`;
  if (next.action !== 'none' && !bodies.some((b) => b?.includes(marker))) {
    if (issue) await swapLabels(issue, next.labels);
    const text = next.action === 'block'
      ? `${marker}\n**Review gate: blocked after ${failed} rounds** (max ${MAX_ROUNDS}). The open findings are in the reviews above; the issue is \`status:blocked\` and needs a decision before more work.`
      : `${marker}\n**Review gate: sent back** (round ${failed} of ${MAX_ROUNDS}). ${result.description}. An implementer fixes the blocking findings on this branch and answers each one; review runs again on the new commit.`;
    await rest('POST', `${R}/issues/${pr}/comments`, { body: text });
    if (issue && next.action === 'block') {
      await rest('POST', `${R}/issues/${issue}/comments`, { body: `${marker}\nPR #${pr} is blocked by the review gate after ${failed} rounds. See the PR for the open findings.` });
    }
  }
  return out;
}

if (process.argv[1]?.endsWith('aggregate.mjs')) {
  const pr = Number(arg('pr'));
  if (!pr) { console.error('usage: aggregate.mjs --pr <n> [--dry-run]'); process.exit(2); }
  aggregatePr(pr, { dry: process.argv.includes('--dry-run') })
    .then((o) => console.log(JSON.stringify(o)))
    .catch((e) => { console.error(`aggregate: ${e.message}`); process.exit(1); });
}
