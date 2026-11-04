#!/usr/bin/env node
// The only way a reviewer agent records its review (#1271).
//
//   node scripts/review/verdict.mjs --pr <n> --role <role> --verdict pass|fail \
//        --blocking <count> --file findings.md
//
// findings.md holds numbered findings, each marked `blocking` or `nit`, with file and line.
// The script refuses:
// - a role that is not required for the PR's current head commit;
// - a stale review: the head moved since routing;
// - `pass` with blocking findings, or `fail` with none.
// Nits never fail a review (criterion 9). Reviewers never push code; implementers never
// call this script. The runner (#1252) only gives it to reviewer runs.

import { readFileSync } from 'node:fs';
import { parseRequired, verdictComment } from './lib.mjs';
import { R, rest, setStatus, comments, arg } from './gh.mjs';
import { aggregatePr } from './aggregate.mjs';

function fail(msg) { console.error(`verdict: ${msg}`); process.exit(2); }

const pr = Number(arg('pr'));
const role = arg('role');
const verdict = arg('verdict');
const blocking = Number(arg('blocking') ?? NaN);
const file = arg('file');
if (!pr || !role || !['pass', 'fail'].includes(verdict) || Number.isNaN(blocking) || !file) {
  fail('usage: --pr <n> --role <role> --verdict pass|fail --blocking <n> --file findings.md');
}
if (verdict === 'pass' && blocking > 0) fail('a review with blocking findings cannot pass');
if (verdict === 'fail' && blocking === 0) fail('a failing review needs at least one blocking finding; nits never fail a review');

const findings = readFileSync(file, 'utf8');
if (!findings.trim()) fail('the findings file is empty; say what was checked, even on a pass');

const p = await rest('GET', `${R}/pulls/${pr}`);
if (!p || p.state !== 'open') fail(`PR #${pr} is not open`);
const req = parseRequired((await comments(pr)).map((c) => c.body));
if (!req) fail('reviewers were not routed for this PR yet (run route.mjs or wait for the workflow)');
if (req.sha !== p.head.sha) fail(`stale: reviewers were routed for ${req.sha.slice(0, 8)} but the head is ${p.head.sha.slice(0, 8)}; review the new commit`);
if (!req.roles.includes(role)) fail(`${role} is not a required reviewer here (required: ${req.roles.join(', ')})`);

await rest('POST', `${R}/issues/${pr}/comments`, { body: verdictComment({ role, sha: p.head.sha, verdict, blocking, findings }) });
await setStatus(p.head.sha, `agent-review/${role}`, verdict === 'pass' ? 'success' : 'failure',
  verdict === 'pass' ? `${role}: approved` : `${role}: ${blocking} blocking finding(s)`);
const agg = await aggregatePr(pr);
console.log(`${role} ${verdict} on ${p.head.sha.slice(0, 8)} → agent-review ${agg.state} (${agg.description})`);
