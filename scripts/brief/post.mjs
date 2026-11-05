#!/usr/bin/env node
// Posts the daily brief (#1320, D-101) on the pinned issue « Brief du jour », as the delivery
// bot from .github/workflows/brief.yml. The owner wrote that issue, so GitHub emails them
// every comment the bot adds. No model is called: the ranking is scripts/brief/rank.mjs.
//
//   node scripts/brief/post.mjs --dry-run    print the comment, post nothing
//   node scripts/brief/post.mjs              post it, once per day

import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { gql, loadConfig, ROOT } from '../board/lib.mjs';
import { R, OWNER, REPO, rest, restAll, BOT_LOGIN } from '../review/gh.mjs';
import { rank, render, openDecisions } from './rank.mjs';

const DRY = process.argv.includes('--dry-run');
const TITLE = 'Brief du jour';
const today = new Date().toISOString().slice(0, 10);
const cfg = loadConfig();

async function openIssues() {
  const out = [];
  let after = null;
  do {
    const d = await gql(`query($after: String) { repository(owner: "${OWNER}", name: "${REPO}") {
      issues(states: OPEN, first: 50, after: $after) { pageInfo { hasNextPage endCursor }
        nodes { number title authorAssociation milestone { title dueOn }
          labels(first: 50) { nodes { name } }
          comments(last: 40) { nodes { body } }
          closedByPullRequestsReferences(first: 5) { nodes { state } }
          projectItems(first: 10) { nodes { project { id }
            fieldValueByName(name: "Sprint") { ... on ProjectV2ItemFieldIterationValue { startDate duration } } } } } } } }`, { after });
    const page = d.repository.issues;
    for (const n of page.nodes) {
      const item = n.projectItems.nodes.find((p) => p.project.id === cfg.projectId);
      out.push({
        number: n.number, title: n.title, authorAssociation: n.authorAssociation,
        milestone: n.milestone, labels: n.labels.nodes.map((l) => l.name),
        comments: n.comments.nodes.map((c) => c.body),
        hasOpenPr: n.closedByPullRequestsReferences.nodes.some((p) => p.state === 'OPEN'),
        sprint: item?.fieldValueByName?.startDate ? item.fieldValueByName : null,
      });
    }
    after = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (after);
  return out;
}

async function briefIssue() {
  const d = await gql(`query { repository(owner: "${OWNER}", name: "${REPO}") {
    pinnedIssues(first: 3) { nodes { issue { number title state author { login } } } } } }`);
  return d.repository.pinnedIssues.nodes.map((n) => n.issue).find((i) => i.title === TITLE && i.state === 'OPEN') ?? null;
}

const issues = await openIssues();
const top = rank(issues, today);
const inFlight = issues.filter((i) => i.labels.some((l) => l === 'status:in-progress' || l === 'status:in-review')).map((i) => i.number).sort((a, b) => a - b);

const waiting = [];
for (const p of await restAll(`${R}/pulls?state=open&base=master`)) waiting.push(`PR #${p.number} « ${p.title} » vers \`master\` — à merger par toi`);
for (const i of issues.filter((x) => x.labels.some((l) => cfg.ownerLabels.includes(l)))) waiting.push(`#${i.number} — ${i.title}`);
for (const d of openDecisions(readFileSync(join(ROOT, 'docs/DECISIONS.md'), 'utf8'))) waiting.push(`${d} (\`docs/DECISIONS.md\`, \`/decide\`)`);

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
