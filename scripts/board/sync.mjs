#!/usr/bin/env node
// Brings the delivery board in line with the repository (#1247, #1248).
//
// Every run recomputes every item from the facts instead of applying the event
// that triggered it, so a missed or failed run is repaired by the next one.
// It only writes what differs. Safe to run at any time, from CI or a laptop.
//
//   node scripts/board/sync.mjs             apply
//   node scripts/board/sync.mjs --dry-run   print the changes and the counts

import { gql, lit, loadConfig, mutateBatched, token, restAll, git, gitOk } from './lib.mjs';
import { deriveStatus, deriveSprint, sprintEnd, deliveryStage, branchIssueNumbers, derivePriority, deriveTier, ARCHIVE } from './derive.mjs';

const DRY = process.argv.includes('--dry-run');
const cfg = loadConfig();

const ISSUE_FIELDS = `id number title state stateReason
  repository { nameWithOwner }
  labels(first: 50) { nodes { name } }
  closedByPullRequestsReferences(first: 10, includeClosedPrs: true) {
    nodes { number state isDraft baseRefName mergeCommit { oid } } }
  timelineItems(itemTypes: [CLOSED_EVENT], last: 1) { nodes { ... on ClosedEvent {
    closer { __typename ... on PullRequest { mergeCommit { oid } } ... on Commit { oid } } } } }`;

const FIELD_VALUES = `fieldValues(first: 20) { nodes {
  ... on ProjectV2ItemFieldSingleSelectValue { optionId field { ... on ProjectV2SingleSelectField { id } } }
  ... on ProjectV2ItemFieldIterationValue { iterationId startDate duration field { ... on ProjectV2IterationField { id } } } } }`;

// The sprint containing today, from the Sprint field's configuration.
async function currentSprint(today) {
  if (!cfg.fields.Sprint) return null;
  const d = await gql(`query($id: ID!) { node(id: $id) { ... on ProjectV2IterationField {
    configuration { iterations { id startDate duration } } } } }`, { id: cfg.fields.Sprint.id });
  return d.node.configuration.iterations.find((i) => i.startDate <= today && sprintEnd(i) > today) ?? null;
}

// The board as seen from each issue. ProjectV2.items does not return every item
// (on 2026-11-04 it listed 395 of 460, the missing ones present on the issue side),
// so the issue side is the reference and the project listing only adds stragglers.
const ISSUE_WITH_ITEM = `${ISSUE_FIELDS}
  projectItems(first: 20, includeArchived: true) { nodes { id isArchived project { id } ${FIELD_VALUES} } }`;

function itemOf(issue) {
  const it = issue.projectItems?.nodes?.find((n) => n.project?.id === cfg.projectId);
  return it ? { id: it.id, isArchived: it.isArchived, content: issue, fieldValues: it.fieldValues } : null;
}

// --- where commits are, from git (the checkout in CI, the clone on a laptop) ---
const DEV = cfg.branches.dev;
const STAGING = cfg.branches.staging;
const RELEASE = new RegExp(cfg.releaseTags);
function fetchRefs() {
  const refs = [DEV, STAGING].filter(Boolean).map((b) => `+refs/heads/${b}:refs/remotes/origin/${b}`);
  if (!gitOk(['fetch', '--quiet', '--tags', '--force', 'origin', ...refs])) {
    console.log('::warning::git fetch failed; delivery stages use the refs already present.');
  }
}
const memo = (fn) => { const m = new Map(); return (k) => (m.has(k) ? m.get(k) : (m.set(k, fn(k)), m.get(k))); };
const known = memo((sha) => gitOk(['cat-file', '-e', `${sha}^{commit}`]));
const onRef = (ref) => memo((sha) => !!ref && known(sha) && gitOk(['merge-base', '--is-ancestor', sha, `refs/remotes/origin/${ref}`]));
const stagePredicates = {
  onDev: onRef(DEV),
  onStaging: onRef(STAGING),
  released: memo((sha) => known(sha) && git(['tag', '--contains', sha]).split('\n').some((t) => RELEASE.test(t))),
};

let branchNumbers = new Set();

function facts(issue) {
  const prs = issue.closedByPullRequestsReferences?.nodes ?? [];
  const merged = prs.filter((p) => p.state === 'MERGED' && p.mergeCommit);
  let sha = null;
  if (issue.state === 'CLOSED') {
    const closer = issue.timelineItems?.nodes?.[0]?.closer;
    sha = closer?.mergeCommit?.oid ?? closer?.oid ?? merged[0]?.mergeCommit.oid ?? null;
  } else {
    // While develop is not the default branch, a merge into it leaves the issue open.
    sha = merged.find((p) => p.baseRefName === DEV)?.mergeCommit.oid ?? null;
  }
  let delivered = deliveryStage(sha, stagePredicates);
  // Merged but the commit is not on the dev branch (merged elsewhere, or not fetched):
  // a closed issue still counts as delivered to dev, never further.
  if (!delivered && sha && issue.state === 'CLOSED') delivered = 'dev';
  return {
    number: issue.number,
    state: issue.state,
    stateReason: issue.stateReason,
    labels: issue.labels.nodes.map((l) => l.name),
    delivered,
    openPr: prs.some((p) => p.state === 'OPEN'),
    hasBranch: branchNumbers.has(issue.number),
  };
}

async function boardItems() {
  const items = [];
  let after = null;
  do {
    const d = await gql(`query($p: ID!, $after: String) { node(id: $p) { ... on ProjectV2 {
      items(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id isArchived
          content { __typename ... on Issue { ${ISSUE_FIELDS} } }
          ${FIELD_VALUES}
        }
      } } } }`, { p: cfg.projectId, after });
    const page = d.node.items;
    items.push(...page.nodes);
    after = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (after);
  return items.filter((i) => i.content?.__typename === 'Issue' && i.content.repository.nameWithOwner === `${cfg.owner}/${cfg.repo}`);
}

// Open issues, plus closed issues updated in the last `closedLookbackDays`
// (so a close is seen even when the project listing misses the item).
async function repoIssues(states, since) {
  const out = [];
  let after = null;
  const filter = since ? `, filterBy: { since: ${lit(since)} }` : '';
  do {
    const d = await gql(`query($after: String) { repository(owner: ${lit(cfg.owner)}, name: ${lit(cfg.repo)}) {
      issues(states: ${states}, first: 50, after: $after${filter}) {
        pageInfo { hasNextPage endCursor }
        nodes { ${ISSUE_WITH_ITEM} }
      } } }`, { after });
    const page = d.repository.issues;
    out.push(...page.nodes);
    after = page.pageInfo.hasNextPage ? page.pageInfo.endCursor : null;
  } while (after);
  return out;
}

const setOption = (itemId, field, optionId) =>
  `updateProjectV2ItemFieldValue(input: { projectId: ${lit(cfg.projectId)}, itemId: ${lit(itemId)}, fieldId: ${lit(field)}, value: { singleSelectOptionId: ${lit(optionId)} } }) { clientMutationId }`;
const clearField = (itemId, field) =>
  `clearProjectV2ItemFieldValue(input: { projectId: ${lit(cfg.projectId)}, itemId: ${lit(itemId)}, fieldId: ${lit(field)} }) { clientMutationId }`;

async function main() {
  if (!token({ required: false })) {
    // In CI the secret is optional until the owner creates it (#1248).
    console.log('::notice::BOARD_TOKEN is not set; board sync skipped.');
    return;
  }
  const started = Date.now();
  fetchRefs();
  const since = new Date(Date.now() - (cfg.closedLookbackDays ?? 30) * 86400e3).toISOString();
  const [items, issues, recentlyClosed, refs] = await Promise.all([
    boardItems(),
    repoIssues('OPEN'),
    repoIssues('CLOSED', since),
    restAll(`/repos/${cfg.owner}/${cfg.repo}/git/matching-refs/heads/`),
  ]);
  branchNumbers = branchIssueNumbers(refs.map((r) => r.ref));
  const today = new Date().toISOString().slice(0, 10);
  const sprintNow = await currentSprint(today);
  let carried = 0;
  const byNumber = new Map(items.map((i) => [i.content.number, i]));
  for (const issue of [...issues, ...recentlyClosed]) {
    const it = itemOf(issue);
    if (it) byNumber.set(issue.number, it);
  }

  // 1. Every open issue is on the board.
  const missing = issues.filter((i) => !byNumber.has(i.number));
  if (missing.length) {
    console.log(`${DRY ? 'would add' : 'add'} ${missing.length} issue(s)`);
    if (!DRY) {
      const res = await mutateBatched(missing.map((i) =>
        `addProjectV2ItemById(input: { projectId: ${lit(cfg.projectId)}, contentId: ${lit(i.id)} }) { item { id } }`));
      missing.forEach((i, k) => byNumber.set(i.number, { id: res[k].item.id, isArchived: false, content: i, fieldValues: { nodes: [] } }));
    } else {
      missing.forEach((i) => byNumber.set(i.number, { id: `new-${i.number}`, isArchived: false, content: i, fieldValues: { nodes: [] } }));
    }
  }

  // 2. Recompute each item.
  const F = cfg.fields;
  const ops = [];
  const changes = [];
  const counts = {};
  const archive = [];
  const unarchive = [];
  const releasedId = F.Status.options.Released;
  for (const item of byNumber.values()) {
    // Released is final for a closed issue: skip the git work.
    if (item.content.state === 'CLOSED' && item.fieldValues.nodes.some((v) => v.optionId === releasedId)) {
      counts.Released = (counts.Released || 0) + 1;
      continue;
    }
    const f = facts(item.content);
    if (item.isArchived) {
      // A reopened issue comes back; anything else archived stays archived.
      if (f.state !== 'OPEN') continue;
      unarchive.push(item);
      changes.push(`#${f.number} unarchive (reopened)`);
    }
    const current = Object.fromEntries(item.fieldValues.nodes.filter((v) => v.field).map((v) => [v.field.id, v.optionId]));
    const status = deriveStatus(f, cfg);
    if (status === ARCHIVE) {
      archive.push(item);
      changes.push(`#${f.number} archive (${f.stateReason})`);
      continue;
    }
    const want = {
      [F.Status.id]: status ? F.Status.options[status] : undefined,
      [F.Priority.id]: derivePriority(f.labels) ? F.Priority.options[derivePriority(f.labels)] : null,
      [F.Tier.id]: deriveTier(f.labels) ? F.Tier.options[deriveTier(f.labels)] : null,
    };
    const statusName = Object.entries(F.Status.options).find(([, id]) => id === (want[F.Status.id] ?? current[F.Status.id]))?.[0] ?? '(none)';
    counts[statusName] = (counts[statusName] || 0) + 1;
    for (const [field, optionId] of Object.entries(want)) {
      if (optionId === undefined) continue; // leave as is
      if ((current[field] ?? null) === optionId) continue;
      ops.push(optionId === null ? clearField(item.id, field) : setOption(item.id, field, optionId));
      if (field === F.Status.id) changes.push(`#${f.number} → ${status}`);
    }
    if (F.Sprint && sprintNow) {
      const v = item.fieldValues.nodes.find((n) => n.field?.id === F.Sprint.id && n.iterationId);
      const sprint = v ? { id: v.iterationId, startDate: v.startDate, duration: v.duration } : null;
      const next = deriveSprint(statusName, { state: f.state, sprint }, sprintNow, today);
      if (next) {
        ops.push(`updateProjectV2ItemFieldValue(input: { projectId: ${lit(cfg.projectId)}, itemId: ${lit(item.id)}, fieldId: ${lit(F.Sprint.id)}, value: { iterationId: ${lit(next)} } }) { clientMutationId }`);
        if (sprint) carried += 1;
        changes.push(`#${f.number} sprint → current${sprint ? ' (carried over)' : ''}`);
      }
    }
  }

  console.log(`${DRY ? 'would write' : 'write'} ${ops.length} field value(s), archive ${archive.length}, carried over ${carried}`);
  for (const c of changes.slice(0, 200)) console.log(`  ${c}`);
  if (changes.length > 200) console.log(`  … ${changes.length - 200} more`);
  if (!DRY) {
    await mutateBatched(unarchive.map((i) => `unarchiveProjectV2Item(input: { projectId: ${lit(cfg.projectId)}, itemId: ${lit(i.id)} }) { clientMutationId }`));
    await mutateBatched(ops);
    await mutateBatched(archive.map((i) => `archiveProjectV2Item(input: { projectId: ${lit(cfg.projectId)}, itemId: ${lit(i.id)} }) { clientMutationId }`));
  }

  const order = Object.keys(F.Status.options).concat('(none)');
  console.log('status counts: ' + order.filter((s) => counts[s]).map((s) => `${s} ${counts[s]}`).join(' · '));
  console.log(`done in ${((Date.now() - started) / 1000).toFixed(1)} s`);
}

main().catch((e) => {
  console.error(`sync: ${e.message}`);
  process.exit(1);
});
