#!/usr/bin/env node
// The one way refinement closes an issue (#1270, rulebook 7.3): with a reason and a proof,
// recorded on the issue before it closes. `gh issue close` is refused by the guard.
// The logic is in scripts/refine/lib.mjs, which the weekly planning also uses (#1326).
//
//   node scripts/refine/close.mjs --issue <n> --reason duplicate --of <kept> --proof "<text>"
//   node scripts/refine/close.mjs --issue <n> --reason not_planned --proof "<path, PR or ADR>"

import { arg } from '../review/gh.mjs';
import { closeIssue, closeProblem } from './lib.mjs';

const n = Number(arg('issue'));
const reason = arg('reason');
const of = arg('of');
const proof = (arg('proof') || '').trim();
if (!n || closeProblem(reason, of, proof)) {
  console.error('usage: --issue <n> --reason duplicate --of <kept> --proof "<text>" | --reason not_planned --proof "<text, 10+ chars>"');
  process.exit(2);
}
try {
  await closeIssue(n, reason, of, proof);
} catch (e) {
  console.error(e.message);
  process.exit(2);
}
console.log(`#${n} closed (${reason})`);
