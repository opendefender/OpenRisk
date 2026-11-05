#!/usr/bin/env node
// Posts the daily brief (#1320, D-101) on the pinned issue « Brief du jour », as the delivery
// bot from .github/workflows/brief.yml. The owner wrote that issue, so GitHub emails them
// every comment the bot adds. No model is called: the ranking is scripts/brief/rank.mjs.
//
//   node scripts/brief/post.mjs --dry-run    print the comment, post nothing
//   node scripts/brief/post.mjs              post it, once per day

import { loadConfig } from '../board/lib.mjs';
import { R, rest, restAll, BOT_LOGIN } from '../review/gh.mjs';
import { rank, render } from './rank.mjs';
import { openIssues, briefIssue, waitingOnOwner, BRIEF_TITLE } from './data.mjs';

const DRY = process.argv.includes('--dry-run');
const TITLE = BRIEF_TITLE;
const today = new Date().toISOString().slice(0, 10);
const cfg = loadConfig();

const issues = await openIssues({ cfg });
const top = rank(issues, today);
const inFlight = issues.filter((i) => i.labels.some((l) => l === 'status:in-progress' || l === 'status:in-review')).map((i) => i.number).sort((a, b) => a - b);

const waiting = await waitingOnOwner(issues, cfg);

const target = await briefIssue();
const body = render({ today, owner: target?.author?.login ?? null, top, inFlight, waiting });

if (DRY) {
  console.log(target ? `target: #${target.number}` : `target: none — no open pinned issue titled « ${TITLE} »`);
  console.log(body);
} else {
  if (!target) throw new Error(`no open pinned issue titled « ${TITLE} »: create it as the owner and pin it`);
  const already = (await restAll(`${R}/issues/${target.number}/comments`))
    .some((c) => c.user?.login === BOT_LOGIN && c.body.startsWith(`### Brief du jour — ${today}`));
  if (already) console.log(`brief for ${today} already on #${target.number}`);
  else {
    await rest('POST', `${R}/issues/${target.number}/comments`, { body });
    console.log(`brief posted on #${target.number}: ${top.map((t) => `#${t.number}`).join(', ') || 'nothing to start'}`);
  }
}
