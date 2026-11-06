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

// One-week sprints starting on Monday (D-090). The first is the week of 2026-11-02.
// setup keeps at least SPRINTS_AHEAD future sprints; re-running it extends the list.
const SPRINT = { name: 'Sprint', start: '2026-11-02', days: 7 };
const SPRINTS_AHEAD = 6;

// The views, in the order of the delivery cycle (#1365). Each one answers one question.
// [name, layout, filter, visible fields (by name), former names]
// Running setup again brings existing views back to this (name, layout, filter, fields);
// a former name is renamed in place, so the view keeps its URL and its hand-set grouping.
// Group-by and sort are not in the API: set them by hand (see the project README).
// Risk class, Agent and Tokens are shown nowhere: the pipeline that wrote them is stopped.
const CARD = ['Title', 'Assignees', 'Priority', 'Size', 'Sprint', 'Linked pull requests'];
const ROW = ['Title', 'Status', 'Priority', 'Tier', 'Size', 'Milestone', 'Sprint', 'Labels'];
export const VIEWS = [
  ['Sprint', 'BOARD_LAYOUT', 'sprint:@current', CARD, ['Current sprint']],
  ['Flow', 'BOARD_LAYOUT', '-status:Inbox,Refinement,Ready,Released', CARD, ['Board']],
  ['Backlog', 'TABLE_LAYOUT', 'status:Ready', ROW, ['Table']],
  ['Refinement', 'TABLE_LAYOUT', 'status:Inbox,Refinement', ROW, ['By milestone']],
  ['Release', 'TABLE_LAYOUT', 'status:"On dev","On staging"', ['Title', 'Status', 'Priority', 'Milestone', 'Linked pull requests', 'Labels'], []],
  ['Needs attention', 'TABLE_LAYOUT', 'status:Blocked,"Needs owner"', ['Title', 'Status', 'Priority', 'Assignees', 'Milestone', 'Labels'], ['Needs owner']],
  ['Roadmap', 'ROADMAP_LAYOUT', 'is:open', [], []], // the API refuses visible fields on a roadmap
];
// Views from before #1365 that no longer have a place.
const RETIRED_VIEWS = ['Blocked'];

const isoDay = (d) => d.toISOString().slice(0, 10);
const addDays = (iso, n) => isoDay(new Date(Date.parse(`${iso}T00:00:00Z`) + n * 86400e3));
// Sprints from `from` (inclusive) until SPRINTS_AHEAD after the one containing today.
export function sprintPlan(from, today = isoDay(new Date())) {
  const out = [];
  let start = from;
  let ahead = -1;
  while (ahead < SPRINTS_AHEAD) {
    out.push({ startDate: start, duration: SPRINT.days });
    if (addDays(start, SPRINT.days) > today) ahead += 1;
    start = addDays(start, SPRINT.days);
  }
  return out;
}
const README = `Delivery board for opendefender/OpenRisk (ADR 0009, #1247, #1248).

**Status is written by automation.** \`scripts/board/sync.mjs\` derives it from repository facts — labels, closing PRs, branches, merge commits, tags — on every issue, PR, push, branch and tag event (\`.github/workflows/board.yml\`) and on a schedule. A hand edit of Status, Priority or Tier is overwritten on the next run: change the label instead.

| Field | Written by | Source |
|---|---|---|
| Status | sync | labels and repository facts |
| Priority | sync | \`priority:\` labels |
| Tier | sync | \`tier:\` labels |
| Size | refinement | set by hand or by the refining agent |
| Risk class | #1250 | the PR's risk check |
| Sprint | planning, sync | sprint planning sets it on Ready items; the sync gives the current sprint to work started without one, and carries unfinished work over |
| Agent, Tokens | #1257 | not written since the local runner was removed (D-102); #1257 decides what fills them |

Statuses: Inbox → Refinement → Ready → In progress → In review → On dev → On staging → Released, plus Blocked and Needs owner. Issues closed as not planned or duplicate are archived.

## The cycle and its views

| Step | How an issue gets there | View |
|---|---|---|
| Inbox → Refinement | opened; \`status:needs-refinement\` | **Refinement** |
| Ready | \`status:ready\` (Ready definition met) | **Backlog** — pick the sprint from here |
| Planned | Sprint set on a Ready item | **Sprint** |
| In progress | a \`<type>/<n>-<slug>\` branch, or \`status:in-progress\` | **Sprint**, **Flow** |
| In review | an open PR with \`Closes #n\`, or \`status:in-review\` | **Sprint**, **Flow** |
| On dev | the PR is merged into \`develop\` | **Flow**, **Release** |
| On staging | the commit is on \`staging\` (sprint-end promotion) | **Flow**, **Release** |
| Released | the commit is in a \`vX.Y.Z\` tag | archived from views |
| Blocked / Needs owner | \`status:blocked\` / \`needs-owner\` | **Needs attention** |

## Automation

All of it lives in \`scripts/board/sync.mjs\`, run by the **Board sync** workflow on every issue, PR, push, branch and tag event and every 30 minutes. GitHub's built-in project workflows are off on purpose: "Item closed → Done" would fight the derived Status.

- Every open issue is added to the board; issues closed as not planned or duplicate are archived; a reopened issue comes back.
- Status, Priority and Tier follow the labels and git facts above.
- Work that starts with no sprint joins the current sprint; unfinished work of an ended sprint is carried over.
- A \`status:in-progress\` or \`status:in-review\` label with no branch and no open PR is believed for 7 days after the issue's last update, then ignored; the run lists those issues as "stale labels" so the label can be fixed.

## Set by hand (not in the API)

Group-by and sort: **Backlog** and **Refinement** group by Priority, sort by Tier; **Sprint** and **Flow** sort by Priority; **Release** group by Status; **Roadmap** dates from Sprint, group by Milestone.`;

// What differs between a live view and its declaration: name, layout, filter, visible fields.
export function viewDiff(view, want) {
  const out = [];
  if (view.name !== want.name) out.push('name');
  if (view.layout !== want.layout) out.push('layout');
  if ((view.filter ?? '') !== want.filter) out.push('filter');
  // GitHub returns visible fields in its own order, so compare them as a set. A roadmap
  // has no visible fields in the API: an empty list means "leave them".
  const have = new Set((view.fields?.nodes ?? []).map((f) => f.id).filter(Boolean));
  if (want.ids.length && (have.size !== want.ids.length || want.ids.some((id) => !have.has(id)))) out.push('fields');
  return out;
}

// True when the field lacks one of the wanted option names (order is ignored).
export function needsOptions(field, names) {
  const have = new Set((field.options ?? []).map((o) => o.name));
  return names.some((n) => !have.has(n));
}
// An option input that keeps the existing option (and its values) when the name exists.
export function optionInput(field, name, color, description) {
  const id = field.options?.find((o) => o.name === name)?.id;
  return `{ ${id ? `id: ${lit(id)}, ` : ''}name: ${lit(name)}, color: ${color}, description: ${lit(description)} }`;
}

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
      ... on ProjectV2IterationField { id name dataType configuration {
        iterations { id startDate duration title } completedIterations { id startDate duration title } } }
    } }
    views(first: 50) { nodes { id name layout filter fields(first: 30) { nodes { ... on ProjectV2FieldCommon { id } } } } }
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

  // Status is the built-in field. Options are compared by name, ignoring order (a column
  // dragged on the board reorders them), and an existing option is always sent back with
  // its id: an option sent without its id is a new option, and every value of the old one
  // is lost. That is what wiped every Status on 2026-11-04 (#1266).
  const status = p.fields.nodes.find((f) => f.name === 'Status');
  if (needsOptions(status, STATUSES.map((s) => s[0]))) {
    log(`Status options → ${STATUSES.map((s) => s[0]).join(', ')}`);
    if (!DRY) {
      const opts = STATUSES.map(([name, color, description]) => optionInput(status, name, color, description)).join(', ');
      await gql(`mutation { updateProjectV2Field(input: { fieldId: ${lit(status.id)}, singleSelectOptions: [${opts}] }) { clientMutationId } }`);
    }
  }

  for (const [name, options] of Object.entries(SELECTS)) {
    const f = p.fields.nodes.find((x) => x.name === name);
    if (f && !needsOptions(f, options)) continue;
    if (f) {
      log(`${name} options → ${options.join(', ')}`);
      if (!DRY) {
        const opts = options.map((o) => optionInput(f, o, 'GRAY', '')).join(', ');
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

  // Sprint: create it, or append future sprints so SPRINTS_AHEAD always exist.
  const sprint = p.fields.nodes.find((x) => x.name === SPRINT.name);
  const sprintTitle = (start) => `Sprint ${start}`;
  if (!sprint) {
    const plan = sprintPlan(SPRINT.start);
    log(`create field ${SPRINT.name} (${plan.length} one-week sprints from ${SPRINT.start})`);
    if (!DRY) {
      const its = plan.map((s) => `{ startDate: ${lit(s.startDate)}, duration: ${s.duration}, title: ${lit(sprintTitle(s.startDate))} }`).join(', ');
      await gql(`mutation { createProjectV2Field(input: { projectId: ${lit(p.id)}, dataType: ITERATION, name: ${lit(SPRINT.name)},
        iterationConfiguration: { startDate: ${lit(SPRINT.start)}, duration: ${SPRINT.days}, iterations: [${its}] } }) { clientMutationId } }`);
    }
  } else {
    const existing = sprint.configuration.iterations; // current and future
    const last = existing.map((i) => i.startDate).sort().at(-1) ?? SPRINT.start;
    const wanted = sprintPlan(SPRINT.start).filter((s) => s.startDate > last);
    if (wanted.length) {
      log(`add ${wanted.length} future sprint(s) after ${last}`);
      if (!DRY) {
        const keep = [...sprint.configuration.completedIterations, ...existing]
          .map((i) => `{ id: ${lit(i.id)}, startDate: ${lit(i.startDate)}, duration: ${i.duration}, title: ${lit(i.title)} }`);
        const add = wanted.map((s) => `{ startDate: ${lit(s.startDate)}, duration: ${s.duration}, title: ${lit(sprintTitle(s.startDate))} }`);
        await gql(`mutation { updateProjectV2Field(input: { fieldId: ${lit(sprint.id)},
          iterationConfiguration: { startDate: ${lit(SPRINT.start)}, duration: ${SPRINT.days}, iterations: [${[...keep, ...add].join(', ')}] } }) { clientMutationId } }`);
      }
    }
  }
  if (!DRY) p = await readProject(p.id);

  // Built-in workflows ("Item closed", "Pull request merged" → Done) would fight the sync.
  for (const w of p.workflows.nodes.filter((w) => w.enabled)) {
    log(`delete built-in workflow "${w.name}"`);
    if (!DRY) await gql(`mutation { deleteProjectV2Workflow(input: { workflowId: ${lit(w.id)} }) { clientMutationId } }`);
  }

  for (const [name, layout, filter, visible, former] of VIEWS) {
    const ids = visible.map((n) => p.fields.nodes.find((f) => f.name === n)?.id).filter(Boolean);
    const conf = ids.length ? `, configuration: { visibleFieldIds: [${ids.map(lit).join(', ')}] }` : '';
    const view = p.views.nodes.find((v) => v.name === name) ?? p.views.nodes.find((v) => former.includes(v.name));
    if (!view) {
      log(`create view ${name}`);
      if (DRY) continue;
      const d = await gql(`mutation { createProjectV2View(input: { projectId: ${lit(p.id)}, name: ${lit(name)}, layout: ${layout}${conf} }) { projectV2View { id } } }`);
      if (filter) await gql(`mutation { updateProjectV2View(input: { viewId: ${lit(d.createProjectV2View.projectV2View.id)}, filter: ${lit(filter)} }) { clientMutationId } }`);
      continue;
    }
    const diff = viewDiff(view, { name, layout, filter, ids });
    if (!diff.length) continue;
    log(`update view ${view.name}${view.name === name ? '' : ` → ${name}`} (${diff.join(', ')})`);
    if (!DRY) await gql(`mutation { updateProjectV2View(input: { viewId: ${lit(view.id)}, name: ${lit(name)}, layout: ${layout}, filter: ${lit(filter)}${conf} }) { clientMutationId } }`);
  }
  for (const v of p.views.nodes.filter((v) => RETIRED_VIEWS.includes(v.name))) {
    log(`delete retired view ${v.name}`);
    if (!DRY) await gql(`mutation { deleteProjectV2View(input: { viewId: ${lit(v.id)} }) { clientMutationId } }`);
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
    if (!['Status', SPRINT.name, ...Object.keys(SELECTS), ...Object.keys(OTHERS)].includes(f.name)) continue;
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

if (process.argv[1]?.endsWith('setup.mjs')) main().catch((e) => {
  console.error(`setup: ${e.message}`);
  process.exit(1);
});
