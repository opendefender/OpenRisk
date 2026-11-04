// Pure functions: what a board item should show, from the facts of its issue.
// No I/O here, so every rule is unit-tested (derive.test.mjs).

export const ARCHIVE = 'ARCHIVE';

const has = (labels, name) => labels.includes(name);

// facts: { state: 'OPEN' | 'CLOSED', stateReason, labels: string[] }
// cfg:   { ownerLabels: string[] }
// Returns a Status option name, ARCHIVE, or null (leave the item as it is).
export function deriveStatus(facts, cfg) {
  const labels = facts.labels || [];
  if (facts.state === 'CLOSED') {
    if (facts.stateReason === 'NOT_PLANNED' || facts.stateReason === 'DUPLICATE') return ARCHIVE;
    return null;
  }
  if ((cfg.ownerLabels || []).some((l) => has(labels, l))) return 'Needs owner';
  if (has(labels, 'status:blocked')) return 'Blocked';
  if (has(labels, 'status:in-review')) return 'In review';
  if (has(labels, 'status:in-progress')) return 'In progress';
  if (has(labels, 'status:ready')) return 'Ready';
  if (has(labels, 'status:needs-refinement')) return 'Refinement';
  return 'Inbox';
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
