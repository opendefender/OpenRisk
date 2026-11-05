// What the daily brief (#1320) and the weekly planning (#1326) read from GitHub. I/O only;
// the ranking and the rendering are pure and live in rank.mjs and scripts/plan/plan.mjs.
// Importing this module has no side effect.

import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { gql, loadConfig, ROOT } from '../board/lib.mjs';
import { R, OWNER, REPO, restAll } from '../review/gh.mjs';
import { openDecisions, safe } from './rank.mjs';

export const BRIEF_TITLE = 'Brief du jour';

const WHO = 'author { __typename login }';

// Every open issue. `detail` adds what the planning needs: body, dates, node ids, the label
// history (has the issue ever been claimed?), closing PRs of any state, the Size field.
export async function openIssues({ detail = false, cfg = loadConfig() } = {}) {
  const out = [];
  let after = null;
  const extra = detail ? `id body createdAt updatedAt
    labelEvents: timelineItems(itemTypes: [LABELED_EVENT], last: 100) { totalCount nodes { ... on LabeledEvent { label { name } } } }` : '';
  do {
    const d = await gql(`query($after: String) { repository(owner: "${OWNER}", name: "${REPO}") {
      issues(states: OPEN, first: ${detail ? 30 : 50}, after: $after) { pageInfo { hasNextPage endCursor }
        nodes { number title authorAssociation ${WHO} ${extra} milestone { number title dueOn }
          labels(first: 50) { nodes { name } }
          comments(last: 40) { nodes { body authorAssociation ${WHO} } }
          closedByPullRequestsReferences(first: ${detail ? 10 : 5}${detail ? ', includeClosedPrs: true' : ''}) { nodes { number state baseRefName } }
          projectItems(first: 10) { nodes { id project { id }
            fieldValueByName(name: "Sprint") { ... on ProjectV2ItemFieldIterationValue { iterationId startDate duration } }
            size: fieldValueByName(name: "Size") { ... on ProjectV2ItemFieldSingleSelectValue { name } } } } } } } }`, { after });
    const page = d.repository.issues;
    for (const n of page.nodes) {
      const item = n.projectItems.nodes.find((p) => p.project.id === cfg.projectId);
      const prs = n.closedByPullRequestsReferences.nodes;
      const issue = {
        number: n.number, title: n.title, authorAssociation: n.authorAssociation, author: n.author,
        milestone: n.milestone, labels: n.labels.nodes.map((l) => l.name),
        comments: n.comments.nodes,
        hasOpenPr: prs.some((p) => p.state === 'OPEN'),
        sprint: item?.fieldValueByName?.startDate ? item.fieldValueByName : null,
      };
      if (detail) {
        Object.assign(issue, {
          nodeId: n.id, itemId: item?.id ?? null, body: n.body ?? '', createdAt: n.createdAt, updatedAt: n.updatedAt,
          size: item?.size?.name ?? null,
          openPrs: prs.filter((p) => p.state === 'OPEN').map((p) => p.number),
          // Merged into the dev branch while the issue stays open: delivered, not carried over.
          mergedPr: prs.some((p) => p.state === 'MERGED' && p.baseRefName === cfg.branches.dev),
          // The label history is capped at 100 events: past that, fail closed (claimed).
          everClaimed: n.labelEvents.totalCount > n.labelEvents.nodes.length ||
            n.labelEvents.nodes.some((e) => e.label?.name === 'status:in-progress'),
        });
      }
      out.push(issue);
    }
    after = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (after);
  return out;
}

// The pinned issue « Brief du jour », or null. Only an issue written by the organisation's
// own people: whoever wrote it is @-mentioned every day. On an organisation repository the
// owner shows as MEMBER, not OWNER.
export async function briefIssue() {
  const d = await gql(`query { repository(owner: "${OWNER}", name: "${REPO}") {
    pinnedIssues(first: 3) { nodes { issue { number title state authorAssociation author { login } } } } } }`);
  return d.repository.pinnedIssues.nodes.map((n) => n.issue)
    .find((i) => i.title === BRIEF_TITLE && i.state === 'OPEN' && ['OWNER', 'MEMBER'].includes(i.authorAssociation)) ?? null;
}

// What only the owner can do: the staging → master release PR, issues carrying an owner
// label, open entries of docs/DECISIONS.md. One line each, already made inert by safe().
export async function waitingOnOwner(issues, cfg = loadConfig()) {
  const waiting = [];
  // Only the release PR the bot opens (rule 3.2b): a fork can open a PR into master too.
  for (const p of await restAll(`${R}/pulls?state=open&base=master`)) {
    if (p.head.repo?.full_name !== `${OWNER}/${REPO}` || p.head.ref !== 'staging') continue;
    waiting.push(`PR #${p.number} « ${safe(p.title)} » \`staging → master\` — à merger par toi`);
  }
  for (const i of issues.filter((x) => x.labels.some((l) => cfg.ownerLabels.includes(l)))) waiting.push(`#${i.number} — ${safe(i.title)}`);
  for (const d of openDecisions(readFileSync(join(ROOT, 'docs/DECISIONS.md'), 'utf8'))) waiting.push(`${d} (\`docs/DECISIONS.md\`, \`/decide\`)`);
  return waiting;
}
