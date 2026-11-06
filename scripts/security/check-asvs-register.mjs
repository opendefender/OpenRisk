#!/usr/bin/env node
// Fails when docs/security/ASVS_GAP_REGISTER.md claims more than the repository proves (#1313).
//
// Rule 12 of CLAUDE.md: a row marked "met" names a file and a test that exist. This script
// reads the one table whose header is `| Chapter | Title | Status | Proof | Issue | Notes |`
// and checks, for every row:
//   - the chapter is V1..V14, each exactly once;
//   - the status is met, partial or gap;
//   - every backticked proof path exists inside the repository (symlinks resolved), and
//     `path::Name` also means the file defines Name (`func Name(` in Go, a whole word elsewhere);
//   - met has at least one proof, and at least one of them is a test file (an anchor alone is not);
//   - partial and gap link an issue (#<n>).
//
// Usage: node scripts/security/check-asvs-register.mjs [register.md] [repo-root]
import { existsSync, readFileSync, realpathSync, statSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

export const HEADER = ['Chapter', 'Title', 'Status', 'Proof', 'Issue', 'Notes'];
export const CHAPTERS = Array.from({ length: 14 }, (_, i) => `V${i + 1}`);
const STATUSES = new Set(['met', 'partial', 'gap']);
const TEST_FILE = /(_test\.go|\.test\.[cm]?[jt]sx?|\.spec\.[cm]?[jt]sx?)$/;

const cells = (line) => line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((c) => c.trim());

// Returns the data rows of the register table, or null when the table is absent.
export function parseRegister(markdown) {
  const lines = markdown.split('\n');
  const start = lines.findIndex((l) => l.trim().startsWith('|') && cells(l).join('|') === HEADER.join('|'));
  if (start < 0 || !/^\|?[\s|:-]+\|?$/.test(lines[start + 1]?.trim() ?? '')) return null;
  const rows = [];
  for (let i = start + 2; i < lines.length && lines[i].trim().startsWith('|'); i++) {
    const [chapter, title, status, proof, issue, notes] = cells(lines[i]);
    rows.push({ line: i + 1, chapter, title, status, proof: proof ?? '', issue: issue ?? '', notes: notes ?? '' });
  }
  return rows;
}

// `path` or `path::Name` entries written in backticks in the Proof cell.
export function parseProofs(cell) {
  return [...cell.matchAll(/`([^`]+)`/g)].map(([, raw]) => {
    const [path, anchor] = raw.split('::');
    return { raw, path: path.trim(), anchor: anchor === undefined ? null : anchor.trim() };
  });
}

const isTest = (p) => TEST_FILE.test(p.real ?? p.path);
const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
// A renamed test must not pass because a longer name or a comment still contains the old one.
const defines = (file, name) =>
  (file.endsWith('.go') ? new RegExp(`\\bfunc ${escape(name)}\\(`) : new RegExp(`(^|[^\\w$])${escape(name)}([^\\w$]|$)`, 'm'))
    .test(readFileSync(file, 'utf8'));

export function checkRegister(markdown, root) {
  const rows = parseRegister(markdown);
  if (!rows) return [`register table not found (header must be: | ${HEADER.join(' | ')} |)`];
  const errors = [];
  const seen = new Map();
  const base = realpathSync(root) + sep;
  for (const row of rows) {
    const at = `line ${row.line} (${row.chapter || '?'})`;
    if (!CHAPTERS.includes(row.chapter)) errors.push(`${at}: chapter must be one of V1..V14`);
    else if (seen.has(row.chapter)) errors.push(`${at}: ${row.chapter} already listed on line ${seen.get(row.chapter)}`);
    else seen.set(row.chapter, row.line);

    if (!STATUSES.has(row.status)) {
      errors.push(`${at}: status "${row.status}" must be met, partial or gap`);
      continue;
    }
    const proofs = parseProofs(row.proof);
    for (const p of proofs) {
      // realpath, not resolve: a symlink inside the repository must not reach a file outside it.
      const file = existsSync(resolve(root, p.path)) ? realpathSync(resolve(root, p.path)) : '';
      if (file) p.real = file; // the test rule judges the file a symlink resolves to
      if (p.path === '' || !file.startsWith(base)) {
        errors.push(`${at}: proof path does not exist: ${p.path}`);
      } else if (p.anchor !== null) {
        if (p.anchor === '' || !statSync(file).isFile() || !defines(file, p.anchor)) {
          errors.push(`${at}: ${p.path} does not contain ${p.anchor}`);
        }
      }
    }
    if (row.status === 'met') {
      if (proofs.length === 0) errors.push(`${at}: marked met with no proof path`);
      else if (!proofs.some(isTest)) errors.push(`${at}: marked met but no proof is a test`);
    } else if (!/#\d+/.test(row.issue)) {
      errors.push(`${at}: ${row.status} must link an issue (#<n>)`);
    }
  }
  for (const ch of CHAPTERS) if (!seen.has(ch)) errors.push(`missing row for ${ch}`);
  return errors;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');
  const register = process.argv[2] ?? join(repo, 'docs/security/ASVS_GAP_REGISTER.md');
  const root = process.argv[3] ?? repo;
  const errors = checkRegister(readFileSync(register, 'utf8'), root);
  if (errors.length) {
    for (const e of errors) console.error(`::error file=${register}::${e}`);
    console.error(`${errors.length} problem(s) in ${register}`);
    process.exit(1);
  }
  console.log(`${register}: ${CHAPTERS.length} chapters, every proof path exists.`);
}
