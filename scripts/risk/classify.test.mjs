import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseDiff, classify, missingContexts } from './classify.mjs';

const rules = JSON.parse(readFileSync(new URL('./rules.json', import.meta.url), 'utf8'));
const file = (path, added = ['x']) => ({ path, added });
const red = (files, extra = {}) => classify({ files, fork: false, ...extra }, rules);

test('green: product code outside every red path', () => {
  const r = red([file('backend/internal/handler/risk_handler.go'), file('frontend/src/features/risks/List.tsx'), file('docs/USER_GUIDE.md')]);
  assert.equal(r.cls, 'green');
  assert.deepEqual(r.reasons, []);
});

test('red: one fixture per path rule', () => {
  for (const r of rules.red_paths) {
    const path = r.match ? (r.match.endsWith('/') ? r.match + 'x.go' : r.match)
      : r.suffix ? 'backend/internal/handler/foo_' + r.suffix
      : 'backend/internal/handler/authz_matrix_test.go';
    assert.equal(red([file(path)]).cls, 'red', `${path} should be red (${r.why})`);
  }
});

test('red path exceptions: agent memory stays green, prefixes do not over-match', () => {
  assert.equal(red([file('.claude/agent-memory/tech-lead/x.md')]).cls, 'green');
  assert.equal(red([file('.claude/hooks/guard-bash.sh')]).cls, 'red');
  assert.equal(red([file('backend/pkg/authz_helpers/x.go')]).cls, 'green'); // not backend/pkg/auth/
  assert.equal(red([file('docs/LICENSE_FAQ.md')]).cls, 'green');
});

test('migrations: an added DROP or RENAME is red, a down migration and a CREATE are not', () => {
  assert.equal(red([file('migrations/0090_x.up.sql', ['ALTER TABLE risks DROP COLUMN legacy;'])]).cls, 'red');
  assert.equal(red([file('migrations/0091_x.up.sql', ['ALTER TABLE risks RENAME COLUMN a TO b;'])]).cls, 'red');
  assert.equal(red([file('migrations/0092_x.up.sql', ['DROP TABLE old_things;'])]).cls, 'red');
  assert.equal(red([file('migrations/0090_x.down.sql', ['DROP TABLE new_things;'])]).cls, 'green');
  assert.equal(red([file('migrations/0093_x.up.sql', ['CREATE TABLE t (id uuid);'])]).cls, 'green');
  // A DROP outside a migration (Go code building SQL) is left to review, not this rule.
  assert.equal(red([file('backend/internal/handler/x.go', ['// DROP TABLE is never sent'])]).cls, 'green');
});

test('fail closed: fork, unreadable change, missing rules', () => {
  assert.match(red([file('docs/x.md')], { fork: true }).reasons.join(), /fork/);
  assert.equal(red([], { error: 'no merge base' }).cls, 'red');
  assert.equal(classify({ files: [file('docs/x.md')], fork: false }, null).cls, 'red');
});

test('parseDiff: paths and added lines only', () => {
  const d = [
    'diff --git a/migrations/1.up.sql b/migrations/1.up.sql',
    '--- a/migrations/1.up.sql', '+++ b/migrations/1.up.sql', '@@ -0,0 +1,2 @@',
    '+DROP TABLE x;', '-old line', '+ok',
    'diff --git a/docs/a.md b/docs/a.md', '+++ b/docs/a.md', '+hello',
  ].join('\n');
  assert.deepEqual(parseDiff(d), [{ path: 'migrations/1.up.sql', added: ['DROP TABLE x;', 'ok'] }, { path: 'docs/a.md', added: ['hello'] }]);
});

test('gate integrity: a required context removed from the workflows is reported', () => {
  const wf = 'jobs:\n  a:\n    name: CI Status\n  b:\n    name: "Secret Detection"\n';
  assert.deepEqual(missingContexts(wf, ['CI Status', 'Secret Detection']), []);
  assert.deepEqual(missingContexts(wf, ['CI Status', 'Licence gate']), ['Licence gate']);
  assert.deepEqual(missingContexts('name: CI Status (old)\n', ['CI Status']), ['CI Status']);
});
