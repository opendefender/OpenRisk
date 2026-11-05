// The Ready definition of CLAUDE.md, as pure functions (#1249). No I/O here.
//
// An issue is workable only with: a problem statement in user terms, numbered testable
// acceptance criteria, a Definition of Done, a milestone, and the three labels type:,
// area:, priority: (either label family, D-003).
//
// Section titles are matched by meaning, in English or French, because the backlog was
// written over months by different people: "Problem", "Problème", "Goal", "Outcome",
// "Context" all state the problem; "Critères d'acceptation" is "Acceptance criteria".

const PROBLEM = /^(problem|problème|probleme|goal|objectif|outcome|context|contexte|why|pourquoi|user story)\b/i;
const CRITERIA = /^(acceptance criteria|critères? d['’]acceptation|criteres? d['’]acceptation|acceptance)\b/i;
const TRUSTED = ['OWNER', 'MEMBER', 'COLLABORATOR'];
// An epic tracks child issues; each child carries its own criteria (e.g. #899, #1002).
const CHILDREN = /^(child issues|sub-issues|issues filles|sous-issues)\b/i;
const DOD = /^(definition of done|dod|définition de (fini|terminé)|definition de (fini|termine))\b/i;

// Splits a Markdown body into { title, text } sections at headings of level 2 or 3.
export function sections(body) {
  const out = [];
  let cur = null;
  for (const line of String(body || '').split(/\r?\n/)) {
    const h = /^#{2,3}\s+(.+?)\s*#*\s*$/.exec(line);
    if (h) {
      cur = { title: h[1].replace(/[*_`]/g, '').trim(), text: '' };
      out.push(cur);
    } else if (cur) {
      cur.text += line + '\n';
    }
  }
  return out;
}

const nonEmpty = (s) => s.replace(/<!--[\s\S]*?-->/g, '').trim().length > 0;
const numbered = (s) => /^\s*\d+[.)]\s+\S/m.test(s);

// Returns { ready: boolean, missing: string[] } for one issue.
// issue: { body, milestone: string|null, labels: string[] }
export function checkReady(issue) {
  const missing = [];
  const secs = sections(issue.body);
  const find = (re) => secs.find((s) => re.test(s.title));

  const problem = find(PROBLEM);
  if (!problem || !nonEmpty(problem.text)) missing.push('a problem statement (`## Problem`)');

  const criteria = find(CRITERIA);
  const children = find(CHILDREN);
  if (!criteria && children && /#\d+/.test(children.text)) {
    // An epic: its acceptance criteria are its child issues.
  } else if (!criteria) missing.push('an `## Acceptance criteria` section');
  else if (!numbered(criteria.text)) missing.push('numbered acceptance criteria (`1.`, `2.`, …)');

  const dod = find(DOD);
  if (!dod || !nonEmpty(dod.text)) missing.push('a `## Definition of Done`');

  if (!issue.milestone) missing.push('a milestone');
  // The repository is public: an issue written by an outsider is never made Ready by a
  // script, or the daily brief would send the owner to a stranger's ticket (#1252 criterion 7).
  if (issue.authorAssociation && !TRUSTED.includes(issue.authorAssociation)) {
    missing.push('a maintainer to take it over (it was opened from outside the team)');
  }
  const labels = issue.labels || [];
  for (const p of ['type:', 'area:', 'priority:']) {
    if (!labels.some((l) => l.startsWith(p))) missing.push(`a \`${p}\` label`);
  }
  return { ready: missing.length === 0, missing };
}

// The only status labels the gate may set, and the ones it never touches.
export const GATE_STATUSES = ['status:ready', 'status:needs-refinement'];
export const PROTECTED_STATUSES = ['status:in-progress', 'status:in-review', 'status:blocked'];

// What the gate should do with an issue's status labels.
// Returns { add: string[], remove: string[] } — both empty when nothing changes.
export function statusChange(labels, ready) {
  if (labels.some((l) => PROTECTED_STATUSES.includes(l))) return { add: [], remove: [] };
  const want = ready ? 'status:ready' : 'status:needs-refinement';
  const other = ready ? 'status:needs-refinement' : 'status:ready';
  return {
    add: labels.includes(want) ? [] : [want],
    remove: labels.includes(other) ? [other] : [],
  };
}

export const MARKER = '<!-- ready-gate -->';

// The one comment the gate keeps on an issue (updated, never duplicated).
export function gateComment(result) {
  if (result.ready) {
    return `${MARKER}\n**Ready gate: passes.** This issue meets the Ready definition of CLAUDE.md.`;
  }
  return [
    MARKER,
    '**Ready gate: not ready yet.** The Ready definition of CLAUDE.md also needs:',
    ...result.missing.map((m) => `- ${m}`),
    '',
    'Fix the body, labels or milestone and the gate runs again on its own.',
  ].join('\n');
}
