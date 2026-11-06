// Pure functions: what a board item should show, from the facts of its issue.
// No I/O here, so every rule is unit-tested (derive.test.mjs).

export const ARCHIVE = 'ARCHIVE';

// How far the commit that delivers an issue has gone. Computed by sync.mjs from git.
export const DELIVERED = { dev: 'On dev', staging: 'On staging', released: 'Released' };

const has = (labels, name) => labels.includes(name);

// facts: {
//   state: 'OPEN' | 'CLOSED', stateReason, labels: string[],
//   delivered: null | 'dev' | 'staging' | 'released'   closing commit merged, and how far it went
//   openPr: boolean      an open PR says it closes the issue
//   hasBranch: boolean   a <type>/<n>-* branch exists
//   staleLabel: boolean  an in-progress / in-review label nothing in git backs (staleActiveLabel)
// }
// cfg: { ownerLabels: string[] }
// Returns a Status option name, or ARCHIVE.
//
// Precedence, first match wins:
//   closed : not planned / duplicate → archive; delivered → how far; otherwise archive
//            (closed by hand with no merged change: nothing left to track).
//   open   : Needs owner, Blocked, delivered (merged into the dev branch while the issue
//            stays open), In review, In progress, Ready, Refinement, Inbox.
export function deriveStatus(facts, cfg) {
  const labels = facts.labels || [];
  if (facts.state === 'CLOSED') {
    if (facts.stateReason === 'NOT_PLANNED' || facts.stateReason === 'DUPLICATE') return ARCHIVE;
    return facts.delivered ? DELIVERED[facts.delivered] : ARCHIVE;
  }
  if ((cfg.ownerLabels || []).some((l) => has(labels, l))) return 'Needs owner';
  if (has(labels, 'status:blocked')) return 'Blocked';
  if (facts.delivered) return DELIVERED[facts.delivered];
  const trustLabel = !facts.staleLabel;
  if (facts.openPr || (trustLabel && has(labels, 'status:in-review'))) return 'In review';
  if (facts.hasBranch || (trustLabel && has(labels, 'status:in-progress'))) return 'In progress';
  if (has(labels, 'status:ready')) return 'Ready';
  if (has(labels, 'status:needs-refinement')) return 'Refinement';
  return 'Inbox';
}

// An in-progress or in-review label with no branch and no open PR is believed for
// STALE_DAYS after the issue's last update (time to push the branch), then ignored (#1365):
// labels left by a stopped run kept issues "In progress" for weeks. updatedAt moves on any
// activity, a comment included, so a progress comment buys the label another STALE_DAYS.
// A missing or unreadable updatedAt trusts the label.
export const STALE_DAYS = 7;
const ACTIVE_LABELS = ['status:in-progress', 'status:in-review'];
export function staleActiveLabel(facts, now = Date.now()) {
  if (facts.state !== 'OPEN' || facts.hasBranch || facts.openPr || facts.delivered) return false;
  if (!ACTIVE_LABELS.some((l) => (facts.labels || []).includes(l))) return false;
  const updated = Date.parse(facts.updatedAt);
  return Number.isFinite(updated) && now - updated > STALE_DAYS * 86400e3;
}

// The furthest stage a commit has reached, from three predicates on the sha.
export function deliveryStage(sha, { onDev, onStaging, released }) {
  if (!sha) return null;
  if (released(sha)) return 'released';
  if (onStaging(sha)) return 'staging';
  if (onDev(sha)) return 'dev';
  return null;
}

// Issue numbers that have a working branch: `<type>/<n>-<slug>` (CLAUDE.md).
export function branchIssueNumbers(refs) {
  const out = new Set();
  for (const r of refs) {
    const m = /^refs\/heads\/[a-z]+\/(\d+)-/.exec(r);
    if (m) out.add(Number(m[1]));
  }
  return out;
}

// Both label families (D-003): `priority:P1-high` and the wave family's `priority:P1`.
// The Constitution family wins when both are present; otherwise the most urgent.
const PRIORITY = { P0: 'P0-critical', P1: 'P1-high', P2: 'P2-medium', P3: 'P3-low' };
export function derivePriority(labels) {
  const long = labels.map((l) => /^priority:(P[0-3])-[a-z]+$/.exec(l)?.[1]).filter(Boolean).sort();
  if (long.length) return PRIORITY[long[0]];
  const short = labels.map((l) => /^priority:(P[0-3])$/.exec(l)?.[1]).filter(Boolean).sort();
  return short.length ? PRIORITY[short[0]] : null;
}

const TIERS = ['0-trust', '1-product-engine', '2-killer-workflow', '3-ecosystem', '4-ux', '5-deferred'];
export function deriveTier(labels) {
  const t = labels.map((l) => /^tier:(.+)$/.exec(l)?.[1]).filter((x) => TIERS.includes(x)).sort();
  return t.length ? t[0] : null;
}

// Sprint rules (#1266). Returns the iteration id to set, or undefined to leave it.
//   status:  the Status just derived for the item
//   item:    { state, sprint: null | { id, startDate, duration } }
//   current: { id, startDate, duration } | null   the sprint containing today
//   today:   'YYYY-MM-DD'
// 1. Open work started (In progress, In review) with no sprint → the current sprint.
// 2. Open work not delivered whose sprint has ended → the current sprint (carry-over).
// Everything else is left: planning sets future sprints, delivered work keeps its sprint.
const STARTED = new Set(['In progress', 'In review']);
const DONE = new Set(['On dev', 'On staging', 'Released']);
export function sprintEnd(s) {
  return new Date(Date.parse(`${s.startDate}T00:00:00Z`) + s.duration * 86400e3).toISOString().slice(0, 10);
}
export function deriveSprint(status, item, current, today) {
  if (!current || item.state !== 'OPEN' || DONE.has(status)) return undefined;
  if (!item.sprint) return STARTED.has(status) ? current.id : undefined;
  if (item.sprint.id !== current.id && sprintEnd(item.sprint) <= today) return current.id;
  return undefined;
}
