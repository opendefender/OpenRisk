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
// after, a run on the next release) ends with `(after merge)` in the issue (#1304). It does
// not hold the merge, under three conditions that keep an agent from exempting itself:
// - the marker ends the line, so prose that mentions it does not count;
// - the same line, marker included, was already in the issue body when the issue was first
//   set `status:in-progress` (read from GitHub's edit history by enqueue.mjs), so a marker
//   added or a criterion reworded during the work still holds the merge;
// - at least one criterion stays unmarked, and the PR names an open follow-up issue
//   (`After merge: #<m>`) that stays open until the criterion is met, because `Closes #<n>`
//   closes the issue itself on merge.
const AFTER_MERGE = /\((?:after merge|après (?:le )?merge)\)\s*$/i;

// Numbered acceptance criteria of the issue body → { count, afterMerge: Map<number, line> }.
export function criteriaOf(issueBody) {
  const lines = String(issueBody || '').split('\n');
  let inSection = false;
  let max = 0;
  const afterMerge = new Map();
  for (const raw of lines) {
    const l = raw.trimEnd();
    if (/^#{2,3}\s/.test(l)) inSection = /^#{2,3}\s+(acceptance criteria|critères? d['’]acceptation)/i.test(l);
    else if (inSection) {
      const m = /^(\d+)[.)]\s+\S/.exec(l);
      if (m) {
        max = Math.max(max, Number(m[1]));
        if (AFTER_MERGE.test(l)) afterMerge.set(Number(m[1]), l);
      }
    }
  }
  return { count: max, afterMerge };
}

// Criteria that may skip the ✅ check: marked now, and marked with the very same line when
// the issue was claimed. No body at claim (unknown, or a revision deleted) → none.
export function exemptCriteria(issueBody, issueBodyAtClaim) {
  const now = criteriaOf(issueBody).afterMerge;
  if (issueBodyAtClaim == null) return new Set();
  const then = criteriaOf(issueBodyAtClaim).afterMerge;
  return new Set([...now].filter(([i, line]) => then.get(i) === line).map(([i]) => i));
}

// The issue body as it stood when `status:in-progress` was first added, from GitHub's own
// history: `labelEvents` [{ createdAt, label }] oldest first, `edits` [{ editedAt, diff }]
// newest first (the oldest one is the original body), `totalEdits` their real count.
// Anything incomplete → null, which exempts nothing.
export function bodyAtClaim({ body, edits, totalEdits, labelEvents }) {
  const events = labelEvents || [];
  // A label deleted since leaves events with no name: the anchor could have been moved by
  // deleting and recreating `status:in-progress`, so the history is not trusted.
  if (events.some((e) => !e?.label)) return null;
  const claim = events.find((e) => e.label === 'status:in-progress')?.createdAt;
  if (!claim) return null;
  if (!totalEdits) return body ?? null;
  // Newest first, so the first revision strictly before the claim is the one in force, even
  // when older pages were not read. An edit in the same second as the claim does not count
  // as before it. Not found in a partial read, or deleted → unknown.
  const at = (edits || []).find((e) => Date.parse(e.editedAt) < Date.parse(claim));
  if (!at || at.deletedAt) return null;
  return at.diff ?? null;
}

// `After merge: #<m>` lines of the PR body → [m, …].
export function followUpRefs(prBody) {
  return [...String(prBody || '').matchAll(/^\s*After merge:\s*#(\d+)\b/gim)].map((m) => Number(m[1]));
}

// Every issue the PR body closes (`Closes`, `Fixes`, `Resolves` #n) → [n, …].
export function closingRefs(prBody) {
  return [...String(prBody || '').matchAll(/\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)\b/gi)].map((m) => Number(m[1]));
}

// The highest numbered criterion (kept for callers that only need the count).
export function criteriaCount(issueBody) {
  return criteriaOf(issueBody).count;
}

// facts = {
//   pr: { draft, fork, body, baseRef }, contexts: { name: 'success'|... },
//   statuses: { context: state }, issueBody, issueComments: [bodies], enabled: boolean,
//   issueBodyAtClaim: string | null,      body when status:in-progress was first set
//   followUps: { [issueNumber]: { state: 'open' | 'closed' | 'missing', body } }   named by `After merge: #m`
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
  const exempt = exemptCriteria(f.issueBody, f.issueBodyAtClaim);
  const marks = latestCriteria(f.issueComments || []);
  if (!n) reasons.push('the issue has no numbered acceptance criteria');
  else if (!marks) reasons.push('no progress report with a **Criteria** line on the issue');
  else {
    const notDone = [];
    const deferred = [];
    for (let i = 1; i <= n; i++) {
      if (marks.get(i) === '✅') continue;
      if (exempt.has(i)) { deferred.push(i); continue; }
      const late = afterMerge.has(i) ? ' (marked after merge only after the issue was claimed: does not count)' : '';
      notDone.push(`${i} ${marks.get(i) || 'missing'}${late}`);
    }
    if (notDone.length) reasons.push(`acceptance criteria not all ✅ in the latest report: ${notDone.join(', ')}`);
    if (exempt.size >= n) reasons.push('every criterion is marked (after merge): at least one must be proven before the merge');
    else if (deferred.length) {
      // A follow-up is an open issue the merge does not close, and whose body names the issue
      // it follows (`#<n>`), so an unrelated open issue does not stand in for it.
      const closes = closingRefs(f.pr.body);
      const main = closes[0];
      const open = followUpRefs(f.pr.body).filter((m) => {
        const u = f.followUps?.[m];
        return !closes.includes(m) && u?.state === 'open' && new RegExp(`#${main}\\b`).test(u.body || '');
      });
      if (!open.length) reasons.push(`criteria ${deferred.join(', ')} are left for after the merge, but the PR names no open follow-up issue that refers to #${main} (\`After merge: #<m>\` in its body)`);
    }
  }
  return { ok: reasons.length === 0, reasons };
}
