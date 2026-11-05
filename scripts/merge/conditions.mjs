// When may a PR enter the develop merge queue (#1280, #1251, ADR 0009, D-088)?
// Pure functions: the decision is a script's, never a model's. No I/O here.

export const REQUIRED_CONTEXTS = [
  'Licence gate', 'CI Status', 'Tenant Isolation Coverage Gate', 'Dependency gate',
  'E2E (chromium + Mobile Chrome)', 'Signed-off-by trailer', 'Frontend Design Tokens', 'Secret Detection',
];

// Criteria marks in the mandatory report: "**Criteria** — 1 ✅ · 2 ✅ · 3 ❌ (reason)".
// → { marks: Map<number, string> } from the most recent report comment, or null.
export function latestCriteria(commentBodies) {
  let found = null;
  for (const body of commentBodies) {
    const line = String(body || '').split('\n').find((l) => /^\s*\*\*Criteria\*\*/.test(l));
    if (!line) continue;
    const marks = new Map();
    for (const m of line.matchAll(/(\d+)[a-z]?\s*(✅|❌|🟡|⏳|🔧|⚠️)/gu)) marks.set(Number(m[1]), m[2]);
    if (marks.size) found = marks;
  }
  return found;
}

// A criterion that can only be checked once the change is on develop (a measurement a day
// after, a run on the next release) is marked `(after merge)` in the issue. It does not hold
// the merge; the report still lists it, and the issue stays open until it is met.
const AFTER_MERGE = /\((after merge|après (le )?merge)\)/i;

// Numbered acceptance criteria of the issue body → { count, afterMerge: Set<number> }.
export function criteriaOf(issueBody) {
  const lines = String(issueBody || '').split('\n');
  let inSection = false;
  let max = 0;
  const afterMerge = new Set();
  for (const l of lines) {
    if (/^#{2,3}\s/.test(l)) inSection = /^#{2,3}\s+(acceptance criteria|critères? d['’]acceptation)/i.test(l);
    else if (inSection) {
      const m = /^(\d+)[.)]\s+\S/.exec(l);
      if (m) {
        max = Math.max(max, Number(m[1]));
        if (AFTER_MERGE.test(l)) afterMerge.add(Number(m[1]));
      }
    }
  }
  return { count: max, afterMerge };
}

// The highest numbered criterion (kept for callers that only need the count).
export function criteriaCount(issueBody) {
  return criteriaOf(issueBody).count;
}

// facts = {
//   pr: { draft, fork, body, baseRef }, contexts: { name: 'success'|... },
//   statuses: { context: state }, issueBody, issueComments: [bodies], enabled: boolean
// }
// → { ok: boolean, reasons: string[] } — reasons lists every unmet condition.
export function mayEnqueue(f) {
  const reasons = [];
  if (!f.enabled) reasons.push('automatic merging is switched off (AUTOMERGE_ENABLED=false)');
  if (f.pr.fork) reasons.push('the PR comes from a fork: never merged automatically');
  if (f.pr.draft) reasons.push('the PR is a draft');
  if (f.pr.baseRef !== 'develop') reasons.push(`the PR targets ${f.pr.baseRef}, not develop`);
  if (!/\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#\d+\b/i.test(f.pr.body || '')) reasons.push('the PR body has no `Closes #<n>`');

  for (const c of REQUIRED_CONTEXTS) {
    const s = f.contexts[c];
    if (s !== 'success' && s !== 'skipped') reasons.push(`required check "${c}" is ${s || 'missing'}`);
  }
  const review = f.statuses['agent-review'];
  if (review !== 'success') reasons.push(`agent-review is ${review || 'missing'} (#1271)`);
  const risk = f.contexts['Risk class'];
  const red = f.statuses['Red gate'];
  if (risk !== 'success') reasons.push(`Risk class is ${risk || 'missing'} (#1250)`);
  if (red !== 'success') reasons.push(`Red gate is ${red || 'missing'}: a red-class PR needs the stricter review`);

  const { count: n, afterMerge } = criteriaOf(f.issueBody);
  const marks = latestCriteria(f.issueComments || []);
  if (!n) reasons.push('the issue has no numbered acceptance criteria');
  else if (!marks) reasons.push('no progress report with a **Criteria** line on the issue');
  else {
    const notDone = [];
    for (let i = 1; i <= n; i++) {
      if (afterMerge.has(i)) continue; // checked after merge, by definition
      if (marks.get(i) !== '✅') notDone.push(`${i} ${marks.get(i) || 'missing'}`);
    }
    if (notDone.length) reasons.push(`acceptance criteria not all ✅ in the latest report: ${notDone.join(', ')}`);
  }
  return { ok: reasons.length === 0, reasons };
}
