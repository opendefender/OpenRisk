#!/usr/bin/env node
// The one way refinement closes an issue (#1270, rulebook 7.3): with a reason and a proof,
// recorded on the issue before it closes. `gh issue close` is refused by the guard.
//
//   node scripts/refine/close.mjs --issue <n> --reason duplicate --of <kept> --proof "<text>"
//   node scripts/refine/close.mjs --issue <n> --reason not_planned --proof "<path, PR or ADR>"

import { R, rest, arg } from '../review/gh.mjs';

const n = Number(arg('issue'));
const reason = arg('reason');
const of = arg('of');
const proof = (arg('proof') || '').trim();
if (!n || !['duplicate', 'not_planned'].includes(reason) || proof.length < 10 || (reason === 'duplicate' && !Number(of))) {
  console.error('usage: --issue <n> --reason duplicate --of <kept> --proof "<text>" | --reason not_planned --proof "<text, 10+ chars>"');
  process.exit(2);
}
const i = await rest('GET', `${R}/issues/${n}`);
if (!i || i.state !== 'open' || i.pull_request) { console.error(`#${n}: not an open issue`); process.exit(2); }
const body = reason === 'duplicate'
  ? `Closing as a duplicate of #${of} (refinement, #1270). The facts unique to this issue were moved there.\n\n**Proof** — ${proof}`
  : `Closing as not planned (refinement, #1270).\n\n**Proof** — ${proof}`;
await rest('POST', `${R}/issues/${n}/comments`, { body });
await rest('PATCH', `${R}/issues/${n}`, { state: 'closed', state_reason: reason === 'duplicate' ? 'duplicate' : 'not_planned' });
console.log(`#${n} closed (${reason})`);
