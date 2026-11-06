#!/usr/bin/env node
// Cuts the next release on the current checkout (D-099, #1364).
//
//   node scripts/release/cut.mjs rc|final|<X.Y.Z[-rc.N]> [--date YYYY-MM-DD] [--dry-run]
//
// It reads the Conventional Commits since the last v* tag reachable from HEAD,
// computes the next version, moves the curated `## [Unreleased]` prose of
// CHANGELOG.md into a dated section (`### Planned` stays where it is), updates
// the compare links and writes VERSION. It never tags, pushes or calls the
// network. `make sync-version` propagates VERSION afterwards.
//
// --dry-run prints the plan as JSON and writes nothing.

import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const SEMVER = /^(\d+)\.(\d+)\.(\d+)(?:-rc\.(\d+))?$/;
const REPO_URL = 'https://github.com/opendefender/OpenRisk';

export function parseVersion(v) {
  const m = SEMVER.exec(String(v).trim().replace(/^v/, ''));
  if (!m) throw new Error(`not a vMAJOR.MINOR.PATCH[-rc.N] version: ${v}`);
  return { major: +m[1], minor: +m[2], patch: +m[3], rc: m[4] === undefined ? null : +m[4] };
}

export function formatVersion({ major, minor, patch, rc }) {
  return `${major}.${minor}.${patch}${rc === null ? '' : `-rc.${rc}`}`;
}

// Negative when a < b. A final release sorts after its own release candidates.
export function compareVersions(a, b) {
  const x = parseVersion(a);
  const y = parseVersion(b);
  for (const k of ['major', 'minor', 'patch']) if (x[k] !== y[k]) return x[k] - y[k];
  if (x.rc === y.rc) return 0;
  if (x.rc === null) return 1;
  if (y.rc === null) return -1;
  return x.rc - y.rc;
}

const HEADER = /^(\w+)(?:\(([^)]*)\))?(!)?:\s*(.+)$/;

// One commit: { subject, body }. Merge commits carry no type and are skipped.
export function classify(commit) {
  const m = HEADER.exec(commit.subject.trim());
  if (!m) return null;
  const breaking = Boolean(m[3]) || /^BREAKING[ -]CHANGE:/m.test(commit.body || '');
  const refs = [...commit.subject.matchAll(/#(\d+)/g)].map((r) => +r[1]);
  return { type: m[1].toLowerCase(), scope: m[2] || null, breaking, summary: m[4], refs };
}

const PATCH_TYPES = new Set(['fix', 'perf', 'security', 'revert']);

export function bumpOf(classified) {
  if (classified.some((c) => c.breaking)) return 'major';
  if (classified.some((c) => c.type === 'feat')) return 'minor';
  if (classified.some((c) => PATCH_TYPES.has(c.type))) return 'patch';
  return null;
}

// mode: 'rc' | 'final' | an explicit version.
// While the last tag is a release candidate, its core version is kept: the
// features since then belong to the version being stabilised. A breaking
// change during that cycle is reported, not applied; the owner passes an
// explicit version if it must become a major.
export function nextVersion(last, mode, bump) {
  const l = parseVersion(last);
  if (mode !== 'rc' && mode !== 'final') {
    const target = formatVersion(parseVersion(mode));
    if (compareVersions(target, last) <= 0) throw new Error(`${target} is not after ${formatVersion(l)}`);
    return target;
  }
  if (l.rc !== null) {
    // A final ships what the last rc proved, nothing more (D-099).
    if (mode === 'final' && bump) {
      throw new Error(`feat, fix or security commits landed since v${formatVersion(l)}: cut an rc first, so they are tested before the final`);
    }
    return formatVersion({ ...l, rc: mode === 'rc' ? l.rc + 1 : null });
  }
  if (!bump) throw new Error(`no feat, fix, perf or breaking commit since v${formatVersion(l)}: nothing to release`);
  const core =
    bump === 'major' ? { major: l.major + 1, minor: 0, patch: 0 }
    : bump === 'minor' ? { major: l.major, minor: l.minor + 1, patch: 0 }
    : { major: l.major, minor: l.minor, patch: l.patch + 1 };
  return formatVersion({ ...core, rc: mode === 'rc' ? 1 : null });
}

// Moves the Unreleased prose into `## [version] - date`, keeps `### Planned`
// under Unreleased, and rewrites the compare links. Throws when the section
// already exists or when Unreleased holds nothing to release.
export function cutChangelog(text, { last, version, date }) {
  if (text.includes(`## [${version}]`)) throw new Error(`CHANGELOG.md already has a [${version}] section`);
  const eol = text.includes('\r\n') ? '\r\n' : '\n';
  const lines = text.split(/\r?\n/);
  const start = lines.findIndex((l) => /^## \[Unreleased\]/.test(l));
  if (start === -1) throw new Error('CHANGELOG.md has no "## [Unreleased]" heading');
  let end = lines.findIndex((l, i) => i > start && /^## \[/.test(l));
  if (end === -1) end = lines.findIndex((l, i) => i > start && /^\[[^\]]+\]: /.test(l));
  if (end === -1) end = lines.length;

  const keep = [];
  const move = [];
  let target = move;
  for (const line of lines.slice(start + 1, end)) {
    if (/^### /.test(line)) target = /^### Planned\b/i.test(line) ? keep : move;
    target.push(line);
  }
  const trim = (block) => {
    const b = [...block];
    while (b.length && b[0].trim() === '') b.shift();
    while (b.length && b[b.length - 1].trim() === '') b.pop();
    return b;
  };
  const moved = trim(move);
  if (moved.length === 0) throw new Error('the [Unreleased] section has nothing to release: write the notes first');
  const kept = trim(keep);

  const out = [
    ...lines.slice(0, start + 1),
    '',
    ...(kept.length ? [...kept, ''] : []),
    `## [${version}] - ${date}`,
    '',
    ...moved,
    '',
    ...lines.slice(end),
  ];

  const unreleasedLink = out.findIndex((l) => /^\[Unreleased\]: /.test(l));
  const compare = `[${version}]: ${REPO_URL}/compare/v${last}...v${version}`;
  if (unreleasedLink === -1) {
    while (out.length && out[out.length - 1] === '') out.pop();
    out.push(`[Unreleased]: ${REPO_URL}/compare/v${version}...HEAD`, compare, '');
  } else {
    out.splice(unreleasedLink, 1, `[Unreleased]: ${REPO_URL}/compare/v${version}...HEAD`, compare);
  }
  return out.join(eol);
}

function git(...args) {
  return execFileSync('git', args, { encoding: 'utf8' }).trim();
}

export function readCommits(range) {
  const raw = git('log', '--no-merges', '--format=%H%x1f%s%x1f%b%x1e', range);
  return raw
    .split('\x1e')
    .map((r) => r.trim())
    .filter(Boolean)
    .map((r) => {
      const [sha, subject, body] = r.split('\x1f');
      return { sha, subject, body };
    });
}

export function plan({ last, mode, commits }) {
  const classified = commits.map((c) => ({ ...c, ...classify(c) })).filter((c) => c.type);
  const bump = bumpOf(classified);
  const version = nextVersion(last, mode, bump);
  const group = (pred) => classified.filter(pred).map((c) => ({ sha: c.sha.slice(0, 8), subject: c.subject }));
  return {
    last,
    version,
    tag: `v${version}`,
    prerelease: version.includes('-'),
    bump,
    breaking: group((c) => c.breaking),
    feat: group((c) => c.type === 'feat'),
    fix: group((c) => PATCH_TYPES.has(c.type)),
    security: group((c) => c.type === 'security' || c.scope === 'security'),
    other: classified.filter((c) => c.type !== 'feat' && !PATCH_TYPES.has(c.type)).length,
    refs: [...new Set(classified.flatMap((c) => c.refs))].sort((a, b) => a - b),
  };
}

function main(argv) {
  process.chdir(git('rev-parse', '--show-toplevel'));
  const args = argv.slice(2);
  const dry = args.includes('--dry-run');
  const di = args.indexOf('--date');
  const date = di === -1 ? new Date().toISOString().slice(0, 10) : args[di + 1];
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date || '')) throw new Error(`--date must be YYYY-MM-DD, got ${date}`);
  const mode = args.find((a, i) => !a.startsWith('--') && args[i - 1] !== '--date');
  if (!mode) throw new Error('usage: cut.mjs rc|final|<X.Y.Z[-rc.N]> [--date YYYY-MM-DD] [--dry-run]');

  const lastTag = git('describe', '--tags', '--abbrev=0', '--match', 'v[0-9]*.[0-9]*.[0-9]*', 'HEAD');
  const last = formatVersion(parseVersion(lastTag));
  const current = readFileSync('VERSION', 'utf8').trim();
  if (current !== last) throw new Error(`VERSION reads ${current} but the last tag is ${lastTag}: settle that first`);

  const p = { ...plan({ last, mode, commits: readCommits(`${lastTag}..HEAD`) }), date };
  if (!dry) {
    writeFileSync('CHANGELOG.md', cutChangelog(readFileSync('CHANGELOG.md', 'utf8'), { last, version: p.version, date }));
    writeFileSync('VERSION', `${p.version}\n`);
  }
  process.stdout.write(`${JSON.stringify({ ...p, written: !dry }, null, 2)}\n`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    main(process.argv);
  } catch (e) {
    process.stderr.write(`cut.mjs: ${e.message}\n`);
    process.exit(1);
  }
}
