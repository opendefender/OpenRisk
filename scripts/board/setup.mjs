#!/usr/bin/env node
// Creates or repairs the "OpenRisk — Delivery" project (#1247) and writes the ids
// of its fields and options to .github/board.json, so the sync never looks them
// up by name. Safe to run again: it only creates what is missing.
//
//   node scripts/board/setup.mjs            create or repair, then write board.json
//   node scripts/board/setup.mjs --dry-run  print what would change

import { gql, lit, loadConfig, saveConfig, configExists } from './lib.mjs';

const DRY = process.argv.includes('--dry-run');
const OWNER = 'opendefender';
const REPO = 'OpenRisk';
const TITLE = 'OpenRisk — Delivery';

// Order matters: it is the column order on the board.
export const STATUSES = [
  ['Inbox', 'GRAY', 'Opened, not refined yet'],
  ['Refinement', 'PURPLE', 'status:needs-refinement — missing criteria, DoD, milestone or labels'],
  ['Ready', 'BLUE', 'status:ready — passes the Ready definition'],
  ['In progress', 'YELLOW', 'status:in-progress, or a <type>/<n>-* branch exists'],
  ['In review', 'ORANGE', 'An open PR closes it, or status:in-review'],
  ['On dev', 'GREEN', 'Closing PR merged into the dev branch'],
  ['On staging', 'GREEN', 'Closing commit is on the staging branch'],
  ['Released', 'PINK', 'Closing commit is in a release tag'],
  ['Blocked', 'RED', 'status:blocked'],
  ['Needs owner', 'RED', 'Waits for an owner decision'],
];
const SELECTS = {
  Priority: ['P0-critical', 'P1-high', 'P2-medium', 'P3-low'],
  Tier: ['0-trust', '1-product-engine', '2-killer-workflow', '3-ecosystem', '4-ux', '5-deferred'],
  Size: ['S', 'M', 'L'],
  'Risk class': ['green', 'red'],
};
const OTHERS = { Agent: 'TEXT', Tokens: 'NUMBER' };
const VIEWS = [
  ['Board', 'BOARD_LAYOUT', ''],
  ['Table', 'TABLE_LAYOUT', ''],
  ['Needs owner', 'TABLE_LAYOUT', 'status:"Needs owner"'],
  ['Blocked', 'TABLE_LAYOUT', 'status:Blocked'],
];
const README = `Delivery board for opendefender/OpenRisk (ADR 0009, #1247, #1248).

**Status is written by automation.** \`scripts/board/sync.mjs\` derives it from repository facts — labels, closing PRs, branches, merge commits, tags — on every issue, PR, push, branch and tag event (\`.github/workflows/board.yml\`) and on a schedule. A hand edit of Status, Priority or Tier is overwritten on the next run: change the label instead.

| Field | Written by | Source |
|---|---|---|
| Status | sync | labels and repository facts |
| Priority | sync | \`priority:\` labels |
| Tier | sync | \`tier:\` labels |
| Size | refinement | set by hand or by the refining agent |
| Risk class | #1250 | the PR's risk check |
| Agent, Tokens | #1252 | the agent runner |

Statuses: Inbox → Refinement → Ready → In progress → In review → On dev → On staging → Released, plus Blocked and Needs owner. Issues closed as not planned or duplicate are archived.`;

const log = (...a) => console.log(DRY ? '[dry-run]' : '', ...a);

async function findProject() {
  const d = await gql(`query { organization(login: ${lit(OWNER)}) { id projectsV2(first: 50) { nodes { id number title closed } } } }`);
  const org = d.organization;
  return { orgId: org.id, project: org.projectsV2.nodes.find((p) => p.title === TITLE && !p.closed) };
}

async function readProject(id) {
  const d = await gql(`query($id: ID!) { node(id: $id) { ... on ProjectV2 {
    id number url
    fields(first: 50) { nodes {
      ... on ProjectV2Field { id name dataType }
      ... on ProjectV2SingleSelectField { id name dataType options { id name } }
    } }
    views(first: 50) { nodes { id name layout } }
    workflows(first: 50) { nodes { id name enabled } }
    repositories(first: 10) { nodes { nameWithOwner } }
  } } }`, { id });
  return d.node;
}

async function main() {
  let { orgId, project } = await findProject();
  if (!project) {
    log(`create project "${TITLE}"`);
    if (DRY) return;
    const d = await gql(`mutation($o: ID!, $t: String!) { createProjectV2(input: { ownerId: $o, title: $t }) { projectV2 { id number title } } }`, { o: orgId, t: TITLE });
    project = d.createProjectV2.projectV2;
  }
  let p = await readProject(project.id);

  if (!p.repositories.nodes.some((r) => r.nameWithOwner === `${OWNER}/${REPO}`)) {
    log('link repository');
    if (!DRY) {
      const r = await gql(`query { repository(owner: ${lit(OWNER)}, name: ${lit(REPO)}) { id } }`);
      await gql(`mutation($p: ID!, $r: ID!) { linkProjectV2ToRepository(input: { projectId: $p, repositoryId: $r }) { clientMutationId } }`, { p: p.id, r: r.repository.id });
    }
  }

  log('description and readme');
  if (!DRY) {
    await gql(`mutation($p: ID!, $s: String!, $r: String!) { updateProjectV2(input: { projectId: $p, shortDescription: $s, readme: $r }) { clientMutationId } }`, {
      p: p.id,
      s: 'Where every OpenRisk issue stands, derived from labels, PRs, branches and tags. Do not edit Status by hand.',
      r: README,
    });
  }

  // Status is the built-in field: replace its options only when the names differ,
  // because replacing options drops the values already set.
  const status = p.fields.nodes.find((f) => f.name === 'Status');
  const want = STATUSES.map((s) => s[0]);
  if (status.options.map((o) => o.name).join('|') !== want.join('|')) {
    log(`Status options → ${want.join(', ')}`);
    if (!DRY) {
      const opts = STATUSES.map(([name, color, description]) => `{ name: ${lit(name)}, color: ${color}, description: ${lit(description)} }`).join(', ');
      await gql(`mutation { updateProjectV2Field(input: { fieldId: ${lit(status.id)}, singleSelectOptions: [${opts}] }) { clientMutationId } }`);
    }
  }

  for (const [name, options] of Object.entries(SELECTS)) {
    const f = p.fields.nodes.find((x) => x.name === name);
    if (f && f.options?.map((o) => o.name).join('|') === options.join('|')) continue;
    if (f) {
      log(`${name} options → ${options.join(', ')}`);
      if (!DRY) {
        const opts = options.map((o) => `{ name: ${lit(o)}, color: GRAY, description: "" }`).join(', ');
        await gql(`mutation { updateProjectV2Field(input: { fieldId: ${lit(f.id)}, singleSelectOptions: [${opts}] }) { clientMutationId } }`);
      }
      continue;
    }
    log(`create field ${name}`);
    if (!DRY) {
      const opts = options.map((o) => `{ name: ${lit(o)}, color: GRAY, description: "" }`).join(', ');
      await gql(`mutation { createProjectV2Field(input: { projectId: ${lit(p.id)}, dataType: SINGLE_SELECT, name: ${lit(name)}, singleSelectOptions: [${opts}] }) { clientMutationId } }`);
    }
  }
  for (const [name, type] of Object.entries(OTHERS)) {
    if (p.fields.nodes.some((x) => x.name === name)) continue;
    log(`create field ${name} (${type})`);
    if (!DRY) await gql(`mutation { createProjectV2Field(input: { projectId: ${lit(p.id)}, dataType: ${type}, name: ${lit(name)} }) { clientMutationId } }`);
  }

  // Built-in workflows ("Item closed", "Pull request merged" → Done) would fight the sync.
  for (const w of p.workflows.nodes.filter((w) => w.enabled)) {
    log(`delete built-in workflow "${w.name}"`);
    if (!DRY) await gql(`mutation { deleteProjectV2Workflow(input: { workflowId: ${lit(w.id)} }) { clientMutationId } }`);
  }

  for (const [name, layout, filter] of VIEWS) {
    if (p.views.nodes.some((v) => v.name === name)) continue;
    log(`create view ${name}`);
    if (DRY) continue;
    const d = await gql(`mutation { createProjectV2View(input: { projectId: ${lit(p.id)}, name: ${lit(name)}, layout: ${layout} }) { projectV2View { id } } }`);
    if (filter) await gql(`mutation { updateProjectV2View(input: { viewId: ${lit(d.createProjectV2View.projectV2View.id)}, filter: ${lit(filter)} }) { clientMutationId } }`);
  }
  // The default "View 1" is empty once ours exist.
  for (const v of p.views.nodes.filter((v) => /^View \d+$/.test(v.name))) {
    log(`delete default view ${v.name}`);
    if (!DRY) await gql(`mutation { deleteProjectV2View(input: { viewId: ${lit(v.id)} }) { clientMutationId } }`);
  }

  if (DRY) return;
  p = await readProject(p.id);
  const prev = configExists() ? loadConfig() : {};
  const fields = {};
  for (const f of p.fields.nodes) {
    if (!['Status', ...Object.keys(SELECTS), ...Object.keys(OTHERS)].includes(f.name)) continue;
    fields[f.name] = { id: f.id, ...(f.options ? { options: Object.fromEntries(f.options.map((o) => [o.name, o.id])) } : {}) };
  }
  saveConfig({
    owner: OWNER,
    repo: REPO,
    projectNumber: p.number,
    projectId: p.id,
    url: p.url,
    // On dev = closing PR merged into `dev`. On staging = closing commit on `staging`
    // (null until #1244 creates it). Released = closing commit in a tag matching `releaseTags`.
    branches: prev.branches ?? { dev: 'master', staging: null },
    releaseTags: prev.releaseTags ?? '^v\\d+\\.\\d+\\.\\d+$',
    ownerLabels: prev.ownerLabels ?? ['status:needs-owner', 'needs-owner'],
    fields,
  });
  console.log(`board: ${p.url}`);
}

main().catch((e) => {
  console.error(`setup: ${e.message}`);
  process.exit(1);
});
