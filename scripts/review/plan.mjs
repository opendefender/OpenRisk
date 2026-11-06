#!/usr/bin/env node
// Which reviewer jobs to start for a PR's head commit (#1271). Prints a JSON array of
// roles for the workflow's matrix, or [] when nothing is due:
// - the head moved since routing, or reviewers were not routed yet;
// - CI is not green on the head commit;
// - the PR already had its last round (MAX_ROUNDS);
// - every required role already has a verdict from the bot on this commit.
//
//   node scripts/review/plan.mjs --pr <n>

import { parseRequired, rounds, MAX_ROUNDS } from './lib.mjs';
import { R, rest, statuses, botComments, arg, BOT_LOGIN } from './gh.mjs';

const pr = Number(arg('pr'));
const out = (roles, why) => { console.error(`#${pr}: ${why}`); console.log(JSON.stringify(roles)); process.exit(0); };
const p = await rest('GET', `${R}/pulls/${pr}`);
if (!p || p.state !== 'open' || p.draft) out([], 'not an open, ready PR');
if (p.head.repo?.full_name !== p.base.repo.full_name) out([], 'fork PR: never reviewed with secrets');
const bodies = (await botComments(pr)).map((c) => c.body);
const req = parseRequired(bodies);
if (!req || req.sha !== p.head.sha) out([], 'reviewers not routed for the head commit yet');
if (rounds(bodies) >= MAX_ROUNDS) out([], `already ${MAX_ROUNDS} review rounds: blocked`);
const ci = await rest('GET', `${R}/commits/${p.head.sha}/check-runs?check_name=${encodeURIComponent('CI Status')}`);
const ciState = ci?.check_runs?.[0]?.conclusion;
if (ciState !== 'success') out([], `CI Status is ${ciState || 'not finished'} on the head commit: review waits for green`);
const st = await statuses(p.head.sha, { onlyFrom: BOT_LOGIN });
const due = req.roles.filter((r) => !['success', 'failure'].includes(st[`agent-review/${r}`]));
out(due, due.length ? `due: ${due.join(', ')}` : 'every required role already reviewed this commit');
