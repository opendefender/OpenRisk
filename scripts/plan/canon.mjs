#!/usr/bin/env node
// Last step of the agent job in .github/workflows/plan.yml (#1326): turns the model's raw
// answer into the one thing allowed to leave the job, the plan as canonical JSON, and only if
// nothing in it or around it looks like a leak. The raw answer is never uploaded: the
// repository is public and artifacts are downloadable.
//
//   node scripts/plan/canon.mjs --in <answer.raw> --out <plan.json> [--forbid-env NAME,NAME]
//
// Exit 1 and no output file when there is no plan, or a secret-shaped string, the agent's
// OAuth token or the job's canary appears anywhere in the answer. Error texts are fixed:
// they never repeat what the model wrote.

import { readFileSync, writeFileSync, rmSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { parsePlan, hasSecret, squash } from './plan.mjs';
import { arg } from '../review/gh.mjs';

function* strings(v) {
  if (typeof v === 'string') yield v;
  else if (Array.isArray(v)) for (const x of v) yield* strings(x);
  else if (v && typeof v === 'object') for (const [k, x] of Object.entries(v)) { yield k; yield* strings(x); }
}

// → { ok: true, json } | { ok: false, error }. `forbidden`: values that must not appear (too short ones are ignored).
export function canonicalise(answer, forbidden = []) {
  const p = parsePlan(answer);
  if (!p.ok) return { ok: false, error: 'the answer holds no valid plan' };
  if (!p.plan || typeof p.plan !== 'object' || Array.isArray(p.plan)) return { ok: false, error: 'the plan is not a JSON object' };
  const json = JSON.stringify(p.plan);
  const all = [...strings(p.plan)];
  if (all.some((s) => hasSecret(s)) || hasSecret(json)) return { ok: false, error: 'the plan holds secret-shaped text' };
  const needles = forbidden.filter((f) => typeof f === 'string' && f.length >= 8).map((f) => squash(f));
  const hay = [squash(String(answer)), squash(json), ...all.map(squash)];
  if (needles.some((n) => hay.some((h) => h.includes(n)))) return { ok: false, error: 'the answer holds a value the agent must never repeat' };
  return { ok: true, json };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const out = arg('out');
  const forbid = (arg('forbid-env') || '').split(',').filter(Boolean).map((n) => process.env[n] || '');
  let raw = '';
  try { raw = readFileSync(arg('in') || '', 'utf8'); } catch { raw = ''; }
  const r = canonicalise(raw, forbid);
  if (!r.ok) {
    rmSync(out, { force: true });
    console.error(`::error::the answer is discarded: ${r.error}`);
    process.exit(1);
  }
  writeFileSync(out, r.json);
  console.log(`plan written: ${r.json.length} bytes`);
}
