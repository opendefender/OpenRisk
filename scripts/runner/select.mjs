// What the agent runner works on next (#1252, #1270, D-094). Pure functions, no I/O.
//
// Order of a batch: refinement first, then implementation. An issue is implemented only
// once it is Ready (#1249) AND refined (#1270): it carries a `<!-- refined: … -->` marker
// from the refinement step, later than its last material edit.

export const REFINED_MARKER = /<!--\s*refined:\s*(\d{4}-\d{2}-\d{2})/;
const TRUSTED = ['OWNER', 'MEMBER', 'COLLABORATOR'];
const PROTECTED = ['status:in-progress', 'status:in-review', 'status:blocked'];

const priorityRank = (labels) => {
  const m = labels.map((l) => /^priority:P([0-3])/.exec(l)?.[1]).filter(Boolean).sort();
  return m.length ? Number(m[0]) : 9;
};
const tierRank = (labels) => {
  const m = labels.map((l) => /^tier:([0-5])-/.exec(l)?.[1]).filter(Boolean).sort();
  return m.length ? Number(m[0]) : 9;
};
// P0 first, then tier 0 first, then the oldest.
export const byUrgency = (a, b) =>
  priorityRank(a.labels) - priorityRank(b.labels) || tierRank(a.labels) - tierRank(b.labels) || a.number - b.number;

// issue = { number, labels, authorAssociation, milestone, comments: [bodies], hasOpenPr, size }
export function isRefined(issue) {
  return (issue.comments || []).some((c) => REFINED_MARKER.test(c || ''));
}

const eligible = (i) =>
  TRUSTED.includes(i.authorAssociation) &&
  !i.labels.includes('status:blocked') &&
  !i.labels.some((l) => l.startsWith('tier:5')) && // deferred work waits for a human call
  !i.hasOpenPr;

// Issues the refinement step takes: Refinement ones, and Ready ones not refined yet.
export function pickRefinement(issues, cap) {
  return issues
    .filter(eligible)
    .filter((i) => !i.labels.some((l) => PROTECTED.includes(l)))
    .filter((i) => i.labels.includes('status:needs-refinement') || (i.labels.includes('status:ready') && !isRefined(i)))
    .sort(byUrgency)
    .slice(0, cap);
}

// Issues the implementation step takes, at most `cap`, never one already in flight.
export function pickImplementation(issues, cap, { milestone } = {}) {
  return issues
    .filter(eligible)
    .filter((i) => i.labels.includes('status:ready') && !i.labels.some((l) => PROTECTED.includes(l)))
    .filter(isRefined)
    .filter((i) => !milestone || i.milestone === milestone)
    .sort(byUrgency)
    .slice(0, cap);
}

// Turn budget by size (S/M/L, set by refinement); M when unknown.
export function turnBudget(size, budgets) {
  return budgets[size] ?? budgets.M;
}

// Did the agent CLI stop because the subscription's usage window ran out?
export function hitUsageLimit(output) {
  return /usage limit|rate limit|quota (exceeded|reached)|limit will reset|Claude AI usage limit/i.test(String(output || ''));
}

// Which interrupted step the next batch resumes first (#1294), or null.
// state: the runner's state.json; issues: the open issues just read from GitHub.
// A clean stop records `resume`; a run that never returned (power cut, crash) left
// `current` set. An issue that was closed or blocked meanwhile is not resumed.
export function resumeTarget(state, issues) {
  const r = state?.current ?? state?.resume;
  if (!r || !Number.isInteger(r.issue)) return null;
  const i = issues.find((x) => x.number === r.issue);
  if (!i || i.labels.includes('status:blocked')) return null;
  return { step: r.step, issue: r.issue, size: i.size };
}
