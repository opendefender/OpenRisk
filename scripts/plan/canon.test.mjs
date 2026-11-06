import { test } from 'node:test';
import assert from 'node:assert/strict';
import { canonicalise } from './canon.mjs';

const wrap = (plan) => `Thinking.\n\n\`\`\`json\n${plan}\n\`\`\`\n`;

test('B1: the agent job hands over the plan as canonical JSON, and nothing else', () => {
  const r = canonicalise(wrap('{ "refine": [ ], "monday": {"notes": "caf\\u00e9"} }'));
  assert.equal(r.ok, true);
  assert.equal(r.json, '{"refine":[],"monday":{"notes":"café"}}');
  assert.equal(canonicalise('no block').ok, false);
  assert.equal(canonicalise(wrap('{broken')).ok, false);
  assert.equal(canonicalise(wrap('[1]')).ok, false, 'a plan is an object');
});

test('B1: a secret-shaped string, hidden or JSON-escaped, empties the answer', () => {
  for (const s of ['sk-ant-oat01-abc', 'sk-\\u0061nt-zzz', 'sk-<!---->ant-x', 'gh\\u0001p_abc', 'ghs_abcdef', '-----BEGIN KEY']) {
    const r = canonicalise(wrap(`{"monday": {"notes": "${s}"}}`));
    assert.equal(r.ok, false, s);
    assert.ok(!JSON.stringify(r).includes('abc'), 'the error repeats nothing');
  }
  assert.equal(canonicalise(wrap('{"sk-ant-key": []}')).ok, false, 'keys are scanned too');
});

test('B1: the OAuth token and the canary, anywhere in the answer, empty it', () => {
  const token = 'oat-the-real-token-value-123';
  const canary = 'canary-0123456789abcdef';
  const forbid = [token, canary, '', 'short'];
  assert.equal(canonicalise(wrap(`{"monday": {"notes": "see ${token}"}}`), forbid).ok, false);
  assert.equal(canonicalise(wrap(`{"monday": {"notes": "x"}}`).replace('Thinking.', `I read ${canary}`), forbid).ok, false, 'in the reasoning too');
  assert.equal(canonicalise(wrap(`{"monday": {"notes": "oat-the-<!-- -->real-token-value-123"}}`), forbid).ok, false, 'split by a comment');
  assert.equal(canonicalise(wrap('{"monday": {"notes": "short word"}}'), forbid).ok, true, 'an empty or short forbidden value matches nothing');
});
