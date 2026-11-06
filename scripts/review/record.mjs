#!/usr/bin/env node
// Records one reviewer's verdict (#1271, D-095). Runs only in the Agent review workflow,
// with the delivery bot's token: the comment and the `agent-review/<role>` status are the
// bot's, and the gates believe no one else's. The verdict comes from the reviewer job's
// own output, never from an agent session.
//
//   node scripts/review/record.mjs --pr <n> --role <role> --sha <sha> --file <answer.txt>
//
// An answer that is not a valid verdict becomes the status `error`: the review is rerun,
// and it counts neither as an approval nor as a review round.

import { readFileSync } from 'node:fs';
import { parseAgentVerdict, parseRequired, verdictComment } from './lib.mjs';
import { R, rest, setStatus, botComments, arg } from './gh.mjs';
import { aggregatePr } from './aggregate.mjs';

const pr = Number(arg('pr'));
const role = arg('role');
const sha = arg('sha');
const file = arg('file');
if (!pr || !role || !/^[0-9a-f]{40}$/.test(sha || '') || !file) {
  console.error('usage: record.mjs --pr <n> --role <role> --sha <40-hex> --file <answer.txt>');
  process.exit(2);
}
if (!process.env.GITHUB_ACTIONS) {
  console.error('record: runs only inside GitHub Actions, as the delivery bot (D-095)');
  process.exit(2);
}

const p = await rest('GET', `${R}/pulls/${pr}`);
if (!p || p.state !== 'open') { console.log(`#${pr}: not open, nothing recorded`); process.exit(0); }
if (p.head.sha !== sha) { console.log(`#${pr}: head moved to ${p.head.sha.slice(0, 8)}; this review of ${sha.slice(0, 8)} is stale, nothing recorded`); process.exit(0); }
const req = parseRequired((await botComments(pr)).map((c) => c.body));
if (!req || req.sha !== sha || !req.roles.includes(role)) {
  console.log(`#${pr}: ${role} is not a required reviewer of ${sha.slice(0, 8)}; nothing recorded`);
  process.exit(0);
}

let answer = '';
try { answer = readFileSync(file, 'utf8'); } catch { answer = ''; }
const v = parseAgentVerdict(answer);
const url = process.env.GITHUB_SERVER_URL && `${process.env.GITHUB_SERVER_URL}/${process.env.GITHUB_REPOSITORY}/actions/runs/${process.env.GITHUB_RUN_ID}`;
if (!v.ok) {
  await setStatus(sha, `agent-review/${role}`, 'error', `${role}: reviewer gave no valid verdict (${v.error})`, url);
  console.log(`#${pr} ${role}: invalid answer (${v.error}) → error`);
} else {
  await rest('POST', `${R}/issues/${pr}/comments`, { body: verdictComment({ role, sha, verdict: v.verdict, blocking: v.blocking, findings: v.findings }) });
  await setStatus(sha, `agent-review/${role}`, v.verdict === 'pass' ? 'success' : 'failure',
    v.verdict === 'pass' ? `${role}: approved` : `${role}: ${v.blocking} blocking finding(s)`, url);
  console.log(`#${pr} ${role}: ${v.verdict} (${v.blocking} blocking)`);
}
const agg = await aggregatePr(pr);
console.log(`agent-review → ${agg.state} (${agg.description})`);
