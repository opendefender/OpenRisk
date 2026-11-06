#!/usr/bin/env node
// Risk class of a PR or a merge group (#1250). Reads the diff with git, classifies it
// with scripts/risk/rules.json, checks gate integrity, and prints the verdict.
//
//   node scripts/risk/check.mjs <base-sha> <head-sha> [--fork]
//
// Exit 0 when classified (green or red; the class is printed and written to
// $GITHUB_OUTPUT as `class`). Exit 1 only when a change to the gates drops a required
// context (criterion 6). Any git failure is classified red, never green.

import { execFileSync } from 'node:child_process';
import { readFileSync, appendFileSync } from 'node:fs';
import { classify, parseDiff, missingContexts } from './classify.mjs';

const [base, head] = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const fork = process.argv.includes('--fork');
const git = (args) => execFileSync('git', args, { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 });

let rules = null;
try {
  rules = JSON.parse(readFileSync(new URL('./rules.json', import.meta.url), 'utf8'));
} catch {
  rules = null; // classify() fails closed on a missing rule file
}

let files = [];
let error = '';
try {
  if (!base || !head) throw new Error('base and head SHAs are required');
  const mb = git(['merge-base', base, head]).trim();
  files = parseDiff(git(['diff', '--no-ext-diff', '--no-textconv', '--no-renames', '-U0', `${mb}..${head}`]));
} catch (e) {
  error = e.message.split('\n')[0];
}

const verdict = classify({ files, fork, error }, rules);

// Gate integrity: if the change touches the workflows, every required context must
// still be declared by the head's workflows.
let missing = [];
const touchesGates = files.some((f) => f.path.startsWith('.github/workflows/') || f.path.startsWith('scripts/risk/'));
if (touchesGates && rules?.required_contexts && head) {
  try {
    const names = git(['ls-tree', '--name-only', `${head}:.github/workflows`]).split('\n').filter((n) => /\.ya?ml$/.test(n));
    const text = names.map((n) => git(['show', `${head}:.github/workflows/${n}`])).join('\n');
    missing = missingContexts(text, rules.required_contexts);
  } catch (e) {
    missing = [`(could not read the head's workflows: ${e.message.split('\n')[0]})`];
  }
}

const lines = [
  `### Risk class: ${verdict.cls === 'red' ? '🔴 red' : '🟢 green'}`,
  '',
  verdict.cls === 'red'
    ? 'This change goes through the stricter Red gate (ADR 0009 D3): devsecops and tech-lead approve the head commit, and the full suites pass.'
    : 'No red-class rule matches this change.',
  ...verdict.reasons.map((r) => `- ${r}`),
];
if (missing.length) lines.push('', `**Gate integrity failed:** these required contexts are no longer declared: ${missing.join(', ')}`);
console.log(lines.join('\n'));
if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, lines.join('\n') + '\n');
if (process.env.GITHUB_OUTPUT) appendFileSync(process.env.GITHUB_OUTPUT, `class=${verdict.cls}\n`);
process.exit(missing.length ? 1 : 0);
