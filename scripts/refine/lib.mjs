// The one way refinement closes an issue (#1270, rulebook 7.3), shared by the CLI
// (scripts/refine/close.mjs) and the weekly planning (scripts/plan/apply.mjs, #1326): with a
// reason and a proof, recorded on the issue before it closes. No side effect on import.

import { R, rest } from '../review/gh.mjs';

export const CLOSE_REASONS = ['duplicate', 'not_planned'];

// Returns null when the arguments are acceptable, else the problem in one line.
export function closeProblem(reason, of, proof) {
  if (!CLOSE_REASONS.includes(reason)) return 'reason must be duplicate or not_planned';
  if ((proof || '').trim().length < 10) return 'the proof needs 10+ characters';
  if (reason === 'duplicate' && !Number(of)) return 'a duplicate needs the kept issue (--of)';
  return null;
}

// Posts the reason and the proof, then closes. Throws on a bad argument or a closed issue.
export async function closeIssue(n, reason, of, proof) {
  const problem = closeProblem(reason, of, proof);
  if (problem) throw new Error(`#${n}: ${problem}`);
  const i = await rest('GET', `${R}/issues/${n}`);
  if (!i || i.state !== 'open' || i.pull_request) throw new Error(`#${n}: not an open issue`);
  const text = proof.trim();
  const body = reason === 'duplicate'
    ? `Closing as a duplicate of #${of} (refinement, #1270). The facts unique to this issue were moved there.\n\n**Proof** — ${text}`
    : `Closing as not planned (refinement, #1270).\n\n**Proof** — ${text}`;
  await rest('POST', `${R}/issues/${n}/comments`, { body });
  await rest('PATCH', `${R}/issues/${n}`, { state: 'closed', state_reason: reason === 'duplicate' ? 'duplicate' : 'not_planned' });
}
