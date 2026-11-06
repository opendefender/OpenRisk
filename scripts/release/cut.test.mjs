import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseVersion, compareVersions, classify, bumpOf, nextVersion, cutChangelog, plan } from './cut.mjs';

const C = (subject, body = '') => ({ sha: 'a'.repeat(40), subject, body });

test('versions parse with or without the v prefix and refuse anything else', () => {
  assert.deepEqual(parseVersion('v1.1.0-rc.8'), { major: 1, minor: 1, patch: 0, rc: 8 });
  assert.deepEqual(parseVersion('2.0.3'), { major: 2, minor: 0, patch: 3, rc: null });
  for (const bad of ['1.1', '1.1.0-beta.1', 'v1.1.0-rc', '1.1.0+meta', '']) assert.throws(() => parseVersion(bad));
});

test('a final release sorts after its release candidates', () => {
  assert.ok(compareVersions('1.1.0', '1.1.0-rc.9') > 0);
  assert.ok(compareVersions('1.1.0-rc.2', '1.1.0-rc.10') < 0);
  assert.ok(compareVersions('1.2.0-rc.1', '1.1.9') > 0);
  assert.equal(compareVersions('1.1.0', 'v1.1.0'), 0);
});

test('Conventional Commit headers are classified, breaking from ! or the footer', () => {
  assert.deepEqual(classify(C('feat(risk): add a matrix (#12)')), { type: 'feat', scope: 'risk', breaking: false, summary: 'add a matrix (#12)', refs: [12] });
  assert.equal(classify(C('fix!: drop the v0 route')).breaking, true);
  assert.equal(classify(C('refactor(api): rename', 'x\n\nBREAKING CHANGE: the field moved')).breaking, true);
  assert.equal(classify(C('Merge pull request #1 from x/y')), null);
});

test('the bump is the strongest change: breaking > feat > fix/perf > nothing', () => {
  const k = (...s) => s.map((x) => classify(C(x)));
  assert.equal(bumpOf(k('fix: a', 'feat: b', 'chore!: c')), 'major');
  assert.equal(bumpOf(k('fix: a', 'feat: b')), 'minor');
  assert.equal(bumpOf(k('docs: a', 'perf: b')), 'patch');
  assert.equal(bumpOf(k('security(auth): c')), 'patch');
  assert.equal(bumpOf(k('docs: a', 'chore: b')), null);
});

test('after a final release, rc and final follow the bump', () => {
  assert.equal(nextVersion('1.1.0', 'rc', 'minor'), '1.2.0-rc.1');
  assert.equal(nextVersion('1.1.0', 'final', 'patch'), '1.1.1');
  assert.equal(nextVersion('1.1.0', 'rc', 'major'), '2.0.0-rc.1');
  assert.throws(() => nextVersion('1.1.0', 'rc', null), /nothing to release/);
});

test('during an rc cycle the core version is kept', () => {
  assert.equal(nextVersion('1.1.0-rc.8', 'rc', 'minor'), '1.1.0-rc.9');
  assert.equal(nextVersion('1.1.0-rc.8', 'final', null), '1.1.0');
  assert.equal(nextVersion('1.1.0-rc.8', 'rc', null), '1.1.0-rc.9');
});

test('a final ships only what the last rc proved', () => {
  assert.throws(() => nextVersion('1.1.0-rc.8', 'final', 'patch'), /cut an rc first/);
  assert.throws(() => nextVersion('1.1.0-rc.8', 'final', 'minor'), /cut an rc first/);
});

test('an explicit version must come after the last one', () => {
  assert.equal(nextVersion('1.1.0-rc.8', 'v2.0.0-rc.1', 'minor'), '2.0.0-rc.1');
  assert.throws(() => nextVersion('1.1.0-rc.8', '1.1.0-rc.8', null), /not after/);
  assert.throws(() => nextVersion('1.1.0-rc.8', '1.0.9', null), /not after/);
  assert.throws(() => nextVersion('1.1.0-rc.8', 'latest', null), /not a v/);
});

const LOG = `# Changelog

Intro.

## [Unreleased]

### Breaking
- \`X\` is required (#1).

### Planned
- Mobile app

### Fixed
- A crash (#2).

## [1.1.0-rc.8] - 2026-10-29

- old

[Unreleased]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.8...HEAD
[1.1.0-rc.8]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.7...v1.1.0-rc.8
`;

test('the Unreleased prose moves verbatim into the dated section, Planned stays', () => {
  const out = cutChangelog(LOG, { last: '1.1.0-rc.8', version: '1.1.0-rc.9', date: '2026-11-06' });
  assert.equal(
    out,
    `# Changelog

Intro.

## [Unreleased]

### Planned
- Mobile app

## [1.1.0-rc.9] - 2026-11-06

### Breaking
- \`X\` is required (#1).

### Fixed
- A crash (#2).

## [1.1.0-rc.8] - 2026-10-29

- old

[Unreleased]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.9...HEAD
[1.1.0-rc.9]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.8...v1.1.0-rc.9
[1.1.0-rc.8]: https://github.com/opendefender/OpenRisk/compare/v1.1.0-rc.7...v1.1.0-rc.8
`,
  );
});

test('CRLF files keep CRLF, and a missing link list gets a final newline', () => {
  const out = cutChangelog(LOG.replace(/\n/g, '\r\n'), { last: '1.1.0-rc.8', version: '1.1.0-rc.9', date: '2026-11-06' });
  assert.equal(out.replace(/\r\n/g, '').includes('\n'), false);
  const bare = cutChangelog('# C\n\n## [Unreleased]\n\n### Fixed\n- x\n', { last: '1.0.0', version: '1.0.1', date: 'd' });
  assert.ok(bare.endsWith('[1.0.1]: https://github.com/opendefender/OpenRisk/compare/v1.0.0...v1.0.1\n'));
});

test('the changelog refuses an empty Unreleased, a missing heading and a duplicate section', () => {
  const empty = LOG.replace(/### Breaking[\s\S]*?(### Planned)/, '$1').replace(/### Fixed\n- A crash \(#2\)\.\n/, '');
  assert.throws(() => cutChangelog(empty, { last: '1.1.0-rc.8', version: '1.1.0-rc.9', date: '2026-11-06' }), /nothing to release/);
  assert.throws(() => cutChangelog('# Changelog\n', { last: '1', version: '2', date: 'd' }), /no "## \[Unreleased\]"/);
  assert.throws(() => cutChangelog(LOG, { last: '1.1.0-rc.7', version: '1.1.0-rc.8', date: 'd' }), /already has/);
});

test('the plan groups commits and collects the referenced issues', () => {
  const p = plan({
    last: '1.1.0',
    mode: 'rc',
    commits: [C('feat(a): x (#5)'), C('fix(b): y (#3)'), C('docs: z (#9)'), C('Merge pull request #7 from a/b')],
  });
  assert.equal(p.version, '1.2.0-rc.1');
  assert.equal(p.tag, 'v1.2.0-rc.1');
  assert.equal(p.prerelease, true);
  assert.equal(p.feat.length, 1);
  assert.equal(p.fix.length, 1);
  assert.equal(p.other, 1);
  assert.deepEqual(p.refs, [3, 5, 9]);
});

test('the CLI cuts a throwaway repository end to end, and --dry-run writes nothing', () => {
  const cli = fileURLToPath(new URL('./cut.mjs', import.meta.url));
  const dir = mkdtempSync(join(tmpdir(), 'cut-'));
  const sh = (...a) => execFileSync('git', a, { cwd: dir, encoding: 'utf8' });
  try {
    sh('init', '-q', '-b', 'main');
    sh('config', 'user.email', 't@example.invalid');
    sh('config', 'user.name', 't');
    sh('config', 'commit.gpgsign', 'false');
    sh('config', 'tag.gpgsign', 'false');
    writeFileSync(join(dir, 'VERSION'), '1.1.0-rc.8\n');
    writeFileSync(join(dir, 'CHANGELOG.md'), LOG);
    sh('add', '.');
    sh('commit', '-qm', 'chore(release): 1.1.0-rc.8');
    sh('tag', '-a', 'v1.1.0-rc.8', '-m', 'rc.8');
    sh('commit', '-q', '--allow-empty', '-m', 'feat(x): something (#42)');

    const run = (...a) => JSON.parse(execFileSync('node', [cli, ...a], { cwd: dir, encoding: 'utf8' }));
    const dry = run('rc', '--date', '2026-11-06', '--dry-run');
    assert.equal(dry.version, '1.1.0-rc.9');
    assert.equal(dry.written, false);
    assert.equal(readFileSync(join(dir, 'VERSION'), 'utf8'), '1.1.0-rc.8\n');

    const real = run('rc', '--date', '2026-11-06');
    assert.equal(real.written, true);
    assert.deepEqual(real.refs, [42]);
    assert.equal(readFileSync(join(dir, 'VERSION'), 'utf8'), '1.1.0-rc.9\n');
    assert.match(readFileSync(join(dir, 'CHANGELOG.md'), 'utf8'), /## \[1\.1\.0-rc\.9\] - 2026-11-06/);

    writeFileSync(join(dir, 'VERSION'), '9.9.9\n');
    assert.throws(() => execFileSync('node', [cli, 'rc', '--dry-run'], { cwd: dir, stdio: 'pipe' }), /VERSION reads 9\.9\.9/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
