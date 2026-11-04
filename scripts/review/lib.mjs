// The PR review gate (#1271), as pure functions. No I/O here.
//
// Who reviews is computed, never chosen by an agent: the reviewers route-reviewers.sh
// prints for the diff (#1005), plus tech-lead on every PR, devsecops on every red-class
// PR (#1250), and qa-automation when tests change. Each reviewer sets its own status
// `agent-review/<role>` on the head SHA; the aggregate `agent-review` passes only when
// every required role passed on that SHA. A new push has new SHA, so it starts over.

export const MAX_ROUNDS = 3;
export const REQUIRED_MARKER = 'agent-review:required';
export const VERDICT_MARKER = 'agent-review:verdict';

const isTest = (p) => /(_test\.go|\.test\.[jt]sx?|\.spec\.[jt]sx?)$/.test(p) || /(^|\/)(e2e|__tests__|tests?)\//.test(p);

// requiredRoles({ routed: string[], cls: 'green'|'red', files: string[] }) → sorted roles.
export function requiredRoles({ routed = [], cls = 'red', files = [] }) {
  const roles = new Set(routed.filter(Boolean));
  roles.add('tech-lead');
  if (cls !== 'green') roles.add('devsecops'); // unknown class counts as red: fail closed
  if (files.some(isTest)) roles.add('qa-automation');
  return [...roles].sort();
}

// aggregate(required, statuses) where statuses maps role → 'success'|'failure'|'pending'.
// → { state: 'success'|'failure'|'pending', description }
export function aggregate(required, statuses) {
  if (!required.length) return { state: 'failure', description: 'no required reviewers recorded (fail closed)' };
  const errored = required.filter((r) => statuses[r] === 'error');
  const failed = required.filter((r) => statuses[r] === 'failure');
  const passed = required.filter((r) => statuses[r] === 'success');
  const waiting = required.filter((r) => !failed.includes(r) && !passed.includes(r) && !errored.includes(r));
  if (failed.length) return { state: 'failure', description: `changes requested by ${failed.join(', ')}` };
  // A reviewer that crashed or answered nonsense is not a rejection: it is rerun, and it
  // does not count as a review round.
  if (errored.length) return { state: 'pending', description: `reviewer error (${errored.join(', ')}): rerun the review` };
  if (waiting.length) return { state: 'pending', description: `waiting for ${waiting.join(', ')}` };
  return { state: 'success', description: `approved by ${passed.join(', ')}` };
}

// The machine-readable comment that records the required roles for one head SHA.
export function requiredComment(sha, roles, reasons = []) {
  return [
    `<!-- ${REQUIRED_MARKER} sha=${sha} roles=${roles.join(',')} -->`,
    `**Review gate (#1271)** — reviewers required on \`${sha.slice(0, 8)}\`: ${roles.map((r) => `\`${r}\``).join(', ')}.`,
    ...reasons.map((r) => `- ${r}`),
    '',
    'Each reviewer sets `agent-review/<role>`; `agent-review` passes when all of them pass on this commit. A new push starts a new round.',
  ].join('\n');
}

// parseRequired(commentBodies) → { sha, roles } of the most recent required-roles comment.
export function parseRequired(bodies) {
  let found = null;
  for (const b of bodies) {
    const m = new RegExp(`<!-- ${REQUIRED_MARKER} sha=([0-9a-f]{7,40}) roles=([a-z0-9,-]*) -->`).exec(b || '');
    if (m) found = { sha: m[1], roles: m[2] ? m[2].split(',') : [] };
  }
  return found;
}

// A verdict comment, posted by a reviewer. `findings` is the reviewer's Markdown.
export function verdictComment({ role, sha, verdict, blocking, findings }) {
  return [
    `<!-- ${VERDICT_MARKER} role=${role} sha=${sha} verdict=${verdict} blocking=${blocking} -->`,
    `### ${role} — ${verdict === 'pass' ? '✅ approve' : `❌ changes requested (${blocking} blocking)`} on \`${sha.slice(0, 8)}\``,
    '',
    findings.trim(),
  ].join('\n');
}

// rounds(commentBodies) → number of distinct head SHAs that got at least one failing verdict.
export function rounds(bodies) {
  const shas = new Set();
  const re = new RegExp(`<!-- ${VERDICT_MARKER} role=[a-z0-9-]+ sha=([0-9a-f]{7,40}) verdict=fail `);
  for (const b of bodies) {
    const m = re.exec(b || '');
    if (m) shas.add(m[1]);
  }
  return shas.size;
}

// What happens to the issue after an aggregate result.
// → { action: 'none'|'send-back'|'block', labels: { add: [], remove: [] } }
export function afterAggregate(state, failedRounds) {
  if (state !== 'failure') return { action: 'none', labels: { add: [], remove: [] } };
  if (failedRounds >= MAX_ROUNDS) {
    return { action: 'block', labels: { add: ['status:blocked'], remove: ['status:in-review', 'status:in-progress'] } };
  }
  return { action: 'send-back', labels: { add: ['status:in-progress'], remove: ['status:in-review'] } };
}

// The issue a PR closes, from its body (`Closes #123`), or null.
export function closingIssue(body) {
  const m = /\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+#(\d+)\b/i.exec(body || '');
  return m ? Number(m[1]) : null;
}

// The reviewer agent ends its answer with one fenced JSON block:
//   {"verdict": "pass"|"fail", "blocking": <int>, "findings": "<markdown>"}
// parseAgentVerdict(text) → { ok: true, verdict, blocking, findings } | { ok: false, error }
export function parseAgentVerdict(text) {
  const blocks = [...String(text || '').matchAll(/```json\s*([\s\S]*?)```/g)];
  if (!blocks.length) return { ok: false, error: 'no fenced json block in the reviewer answer' };
  let v;
  try { v = JSON.parse(blocks[blocks.length - 1][1]); } catch (e) { return { ok: false, error: `invalid json: ${e.message}` }; }
  if (!['pass', 'fail'].includes(v.verdict)) return { ok: false, error: 'verdict must be "pass" or "fail"' };
  if (!Number.isInteger(v.blocking) || v.blocking < 0) return { ok: false, error: 'blocking must be a non-negative integer' };
  if (typeof v.findings !== 'string' || !v.findings.trim()) return { ok: false, error: 'findings must be a non-empty string, even on a pass' };
  if (v.verdict === 'pass' && v.blocking > 0) return { ok: false, error: 'a pass cannot have blocking findings' };
  if (v.verdict === 'fail' && v.blocking === 0) return { ok: false, error: 'a fail needs at least one blocking finding; nits never fail' };
  return { ok: true, verdict: v.verdict, blocking: v.blocking, findings: v.findings };
}

// D-096 (owner): after a fix, only the roles that refused review again; a role that
// approved the previous reviewed commit keeps its approval, and the carry-over is said.
// allRequiredBodies: every bot comment body; current: the head SHA being routed.
// → the previous routing ({ sha, roles }) before `current`, or null.
export function previousRouting(bodies, current) {
  const seen = [];
  const re = new RegExp(`<!-- ${REQUIRED_MARKER} sha=([0-9a-f]{7,40}) roles=([a-z0-9,-]*) -->`);
  for (const b of bodies) {
    const m = re.exec(b || '');
    if (m && m[1] !== current) seen.push({ sha: m[1], roles: m[2] ? m[2].split(',') : [] });
  }
  return seen.length ? seen[seen.length - 1] : null;
}

// carryOver(roles, prevStatuses) → { carried: [], review: [] }
// prevStatuses: the bot's `agent-review/<role>` states on the previous routed commit.
// Carried: the role was required before and approved that commit. Everything else is
// reviewed: roles that refused, errored, never answered, or are newly required.
export function carryOver(roles, prevStatuses, prevRoles = []) {
  const carried = roles.filter((r) => prevRoles.includes(r) && prevStatuses[r] === 'success');
  return { carried, review: roles.filter((r) => !carried.includes(r)) };
}
