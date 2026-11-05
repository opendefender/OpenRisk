// Retries of the REST helpers on transient GitHub errors (#1330).
// fetch is replaced by a queue of canned responses; nothing reaches the network.
import { test, beforeEach } from 'node:test';
import assert from 'node:assert/strict';

process.env.GH_TOKEN = 'test-token';
const { restAll, retryPolicy } = await import('./lib.mjs');
const { rest } = await import('../review/gh.mjs');

let calls;
let slept;
function serve(...responses) {
  calls = [];
  globalThis.fetch = async (url, init = {}) => {
    calls.push({ url, method: init.method || 'GET' });
    const next = responses.shift();
    if (!next) throw new Error('no more canned responses');
    if (next instanceof Error) throw next;
    return next;
  };
}
const json = (status, body, headers = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json', ...headers } });

beforeEach(() => {
  slept = [];
  retryPolicy.waitedMs = 0;
  retryPolicy.sleep = async (ms) => { slept.push(ms); };
});

test('rest: once the wait budget is spent, the next failure is returned without waiting', async () => {
  retryPolicy.waitedMs = retryPolicy.budgetMs - 500;
  serve(json(429, {}, { 'retry-after': '60' }), json(500, {}));
  await assert.rejects(rest('GET', '/x'), /HTTP 500/);
  assert.deepEqual(slept, [500]);
  assert.equal(calls.length, 2);
  serve(json(500, {}));
  await assert.rejects(rest('GET', '/x'), /HTTP 500/);
  assert.equal(calls.length, 1);
});

test('rest: a 500 then a 200 returns the body', async () => {
  serve(json(500, { message: 'boom' }), json(200, { ok: 1 }));
  assert.deepEqual(await rest('GET', '/repos/o/r/commits/abc/check-runs'), { ok: 1 });
  assert.equal(calls.length, 2);
});

test('rest: five 500s in a row throw, after exactly retryPolicy.attempts calls', async () => {
  serve(...Array.from({ length: 5 }, () => json(500, {})));
  await assert.rejects(rest('GET', '/x'), /HTTP 500/);
  assert.equal(calls.length, retryPolicy.attempts);
});

test('rest: a 422 is not retried', async () => {
  serve(json(422, { message: 'invalid' }));
  await assert.rejects(rest('GET', '/x'), /HTTP 422/);
  assert.equal(calls.length, 1);
});

test('rest: a 404 still returns null without retry', async () => {
  serve(json(404, {}));
  assert.equal(await rest('GET', '/x'), null);
  assert.equal(calls.length, 1);
});

test('rest: a POST that gets a 500 is not retried, so nothing is posted twice', async () => {
  serve(json(500, {}), json(201, { id: 1 }));
  await assert.rejects(rest('POST', '/x', { body: 'hi' }), /HTTP 500/);
  assert.equal(calls.length, 1);
});

test('rest: a POST refused by a 429 is retried and honours retry-after', async () => {
  serve(json(429, {}, { 'retry-after': '3' }), json(201, { id: 1 }));
  assert.deepEqual(await rest('POST', '/x', { body: 'hi' }), { id: 1 });
  assert.equal(calls.length, 2);
  assert.deepEqual(slept, [3000]);
});

test('rest: a secondary rate-limit 403 is retried; a plain 403 is not', async () => {
  serve(json(403, {}, { 'x-ratelimit-remaining': '0' }), json(200, { ok: 1 }));
  assert.deepEqual(await rest('GET', '/x'), { ok: 1 });
  serve(json(403, { message: 'Resource not accessible' }));
  await assert.rejects(rest('GET', '/x'), /HTTP 403/);
  assert.equal(calls.length, 1);
});

test('rest: a network error on a GET is retried', async () => {
  serve(new TypeError('fetch failed'), json(200, { ok: 1 }));
  assert.deepEqual(await rest('GET', '/x'), { ok: 1 });
});

test('restAll: a 502 on the second page is retried and both pages are returned', async () => {
  const next = { link: '<https://api.github.com/x?per_page=100&page=2>; rel="next"' };
  serve(json(200, [1, 2], next), json(502, {}), json(200, [3]));
  assert.deepEqual(await restAll('/x'), [1, 2, 3]);
  assert.equal(calls.length, 3);
});

test('restAll: backoff grows between attempts', async () => {
  serve(json(503, {}), json(503, {}), json(200, []));
  await restAll('/x');
  assert.equal(slept.length, 2);
  assert.ok(slept[1] > slept[0]);
});
