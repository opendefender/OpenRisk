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
  const failed = required.filter((r) => statuses[r] === 'failure');
  const passed = required.filter((r) => statuses[r] === 'success');
  const waiting = required.filter((r) => !failed.includes(r) && !passed.includes(r));
  if (failed.length) return { state: 'failure', description: `changes requested by ${failed.join(', ')}` };
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
