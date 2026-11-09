import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { checkRegister, parseProofs, CHAPTERS } from './check-asvs-register.mjs';

// A throwaway repository with one source file and one test file.
const root = mkdtempSync(join(tmpdir(), 'asvs-'));
mkdirSync(join(root, 'backend/auth'), { recursive: true });
writeFileSync(join(root, 'backend/auth/hasher.go'), 'package auth\n');
writeFileSync(join(root, 'backend/auth/hasher_test.go'), 'func TestHasher_Argon2id(t *testing.T) {}\nfunc (h *hasher) helper() {}\n');

const HEAD = '| Chapter | Title | Status | Proof | Issue | Notes |\n|---|---|---|---|---|---|\n';
const row = (ch, status, proof = '—', issue = '—') => `| ${ch} | Title | ${status} | ${proof} | ${issue} | note |`;
const MET = '`backend/auth/hasher.go`<br>`backend/auth/hasher_test.go::TestHasher_Argon2id`';
// Every chapter met with a valid proof, except the overrides.
const register = (overrides = {}) =>
  HEAD + CHAPTERS.map((ch) => overrides[ch] ?? row(ch, 'met', MET)).join('\n') + '\n';

test('a register whose every proof exists passes', () => {
  assert.deepEqual(checkRegister(register(), root), []);
});

test('partial and gap rows with an issue pass', () => {
  const md = register({ V5: row('V5', 'partial', '`backend/auth/hasher.go`', '#1316'), V12: row('V12', 'gap', '—', '#1316') });
  assert.deepEqual(checkRegister(md, root), []);
});

test('a met row whose proof path is missing fails', () => {
  const errors = checkRegister(register({ V2: row('V2', 'met', '`backend/auth/gone_test.go`') }), root);
  assert.equal(errors.length, 1);
  assert.match(errors[0], /V2.*proof path does not exist: backend\/auth\/gone_test\.go/);
});

test('a met row with no proof at all fails', () => {
  assert.match(checkRegister(register({ V3: row('V3', 'met') }), root).join('\n'), /V3.*met with no proof path/);
});

test('a met row whose proofs are not tests fails', () => {
  const errors = checkRegister(register({ V4: row('V4', 'met', '`backend/auth/hasher.go`') }), root);
  assert.match(errors.join('\n'), /V4.*no proof is a test/);
});

test('an anchor on a non-test file does not make the row a test', () => {
  const errors = checkRegister(register({ V5: row('V5', 'met', '`backend/auth/hasher.go::package`') }), root);
  assert.match(errors.join('\n'), /V5.*no proof is a test/);
});

test('an anchor must name the whole test, not a prefix of it', () => {
  const errors = checkRegister(register({ V6: row('V6', 'met', '`backend/auth/hasher_test.go::TestHasher`') }), root);
  assert.match(errors.join('\n'), /V6.*does not contain TestHasher/);
});

test('an empty anchor fails', () => {
  const errors = checkRegister(register({ V6: row('V6', 'met', '`backend/auth/hasher_test.go::`') }), root);
  assert.match(errors.join('\n'), /V6.*does not contain/);
});

test('a test anchor absent from the file fails', () => {
  const errors = checkRegister(register({ V6: row('V6', 'met', '`backend/auth/hasher_test.go::TestRenamed`') }), root);
  assert.match(errors.join('\n'), /V6.*does not contain TestRenamed/);
});

test('a proof outside the repository fails', () => {
  const errors = checkRegister(register({ V7: row('V7', 'met', '`../../etc/passwd_test.go`') }), root);
  assert.match(errors.join('\n'), /V7.*proof path does not exist/);
});

test('a symlink pointing outside the repository fails', () => {
  const outside = mkdtempSync(join(tmpdir(), 'asvs-outside-'));
  writeFileSync(join(outside, 'secret_test.go'), 'TestSecret\n');
  symlinkSync(join(outside, 'secret_test.go'), join(root, 'link_test.go'));
  const errors = checkRegister(register({ V11: row('V11', 'met', '`link_test.go::TestSecret`') }), root);
  assert.match(errors.join('\n'), /V11.*proof path does not exist: link_test\.go/);
});

test('a symlink named like a test does not make a non-test file a test', () => {
  symlinkSync(join(root, 'backend/auth/hasher.go'), join(root, 'backend/auth/alias_test.go'));
  const errors = checkRegister(register({ V12: row('V12', 'met', '`backend/auth/alias_test.go`') }), root);
  assert.match(errors.join('\n'), /V12.*no proof is a test/);
});

test('partial or gap without an issue fails', () => {
  const errors = checkRegister(register({ V8: row('V8', 'gap'), V9: row('V9', 'partial', '—', '#NEW-1') }), root);
  assert.match(errors.join('\n'), /V8.*gap must link an issue/);
  assert.match(errors.join('\n'), /V9.*partial must link an issue/);
});

test('an unknown status fails', () => {
  assert.match(checkRegister(register({ V10: row('V10', 'done', MET) }), root).join('\n'), /V10.*status "done"/);
});

test('a missing or duplicated chapter fails', () => {
  const missing = HEAD + CHAPTERS.filter((c) => c !== 'V14').map((c) => row(c, 'met', MET)).join('\n');
  assert.match(checkRegister(missing, root).join('\n'), /missing row for V14/);
  const dup = register({ V14: row('V13', 'met', MET) });
  assert.match(checkRegister(dup, root).join('\n'), /V13 already listed/);
});

test('a header without its separator row is not the register', () => {
  assert.match(checkRegister(register().replace('|---|---|---|---|---|---|\n', ''), root)[0], /register table not found/);
});

test('a document without the register table fails', () => {
  assert.match(checkRegister('# nothing here\n', root)[0], /register table not found/);
});

test('proofs are read from backticks only, with an optional ::anchor', () => {
  assert.deepEqual(parseProofs('`a/b.go`, `c_test.go::TestX` and prose'), [
    { raw: 'a/b.go', path: 'a/b.go', anchor: null },
    { raw: 'c_test.go::TestX', path: 'c_test.go', anchor: 'TestX' },
  ]);
});
