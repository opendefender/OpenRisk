// Shared helpers for the delivery board scripts (#1247, #1248).
// No dependencies: Node's fetch and child_process only.
//
// Token: BOARD_TOKEN, then GH_TOKEN / GITHUB_TOKEN, then `gh auth token`.
// It needs the `project` scope (or fine-grained "Projects: read and write" on the
// organisation) and read access to the repository. It is never printed.

import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
export const CONFIG_PATH = join(ROOT, '.github', 'board.json');

export function loadConfig() {
  return JSON.parse(readFileSync(CONFIG_PATH, 'utf8'));
}

export function saveConfig(cfg) {
  writeFileSync(CONFIG_PATH, JSON.stringify(cfg, null, 2) + '\n');
}

export function configExists() {
  return existsSync(CONFIG_PATH);
}

let cachedToken;
export function token({ required = true } = {}) {
  if (cachedToken !== undefined) return cachedToken;
  cachedToken = process.env.BOARD_TOKEN || process.env.GH_TOKEN || process.env.GITHUB_TOKEN || '';
  if (!cachedToken && !process.env.CI) {
    try {
      cachedToken = execFileSync('gh', ['auth', 'token'], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();
    } catch {
      cachedToken = '';
    }
  }
  if (!cachedToken && required) throw new Error('no token: set BOARD_TOKEN or log in with gh');
  return cachedToken;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// One GraphQL call, with retries on transient and secondary rate-limit errors.
export async function gql(query, variables = {}, attempt = 0) {
  const res = await fetch('https://api.github.com/graphql', {
    method: 'POST',
    headers: {
      authorization: `bearer ${token()}`,
      'content-type': 'application/json',
      'user-agent': 'openrisk-board-sync',
    },
    body: JSON.stringify({ query, variables }),
  });
  const limited = res.headers.get('retry-after') || res.headers.get('x-ratelimit-remaining') === '0';
  const retryable = res.status >= 500 || res.status === 429 || (res.status === 403 && limited);
  if (retryable && attempt < 6) {
    const after = Number(res.headers.get('retry-after')) || 2 ** attempt * 2;
    await sleep(after * 1000);
    return gql(query, variables, attempt + 1);
  }
  const raw = await res.text();
  let body = {};
  try { body = JSON.parse(raw); } catch { /* not JSON: handled below */ }
  if (res.ok && !body.errors && !body.data) {
    // A 200 with no data and no errors was seen under load on 2026-11-04: retry it like a 502.
    if (attempt < 4) {
      await sleep(2 ** attempt * 2000);
      return gql(query, variables, attempt + 1);
    }
    throw new Error(`GraphQL: empty response (HTTP ${res.status}): ${raw.slice(0, 200)}`);
  }
  if (!res.ok || body.errors) {
    const msg = (body.errors || []).map((e) => e.message).join('; ') || `HTTP ${res.status}`;
    if (/rate limit|abuse|secondary/i.test(msg) && attempt < 6) {
      await sleep(2 ** attempt * 5000);
      return gql(query, variables, attempt + 1);
    }
    throw new Error(`GraphQL: ${msg}`);
  }
  return body.data;
}

// How REST calls ride out transient GitHub errors (#1330). Tests replace `sleep`.
// `budgetMs` caps the total time one process spends waiting on retries, so a long outage
// fails the run cleanly before the job's timeout instead of being cancelled mid-way.
export const retryPolicy = { attempts: 5, baseMs: 1000, maxWaitMs: 60_000, budgetMs: 120_000, waitedMs: 0, sleep };

async function backoff(ms) {
  const wait = Math.min(ms, retryPolicy.maxWaitMs, retryPolicy.budgetMs - retryPolicy.waitedMs);
  if (wait <= 0) return false;
  retryPolicy.waitedMs += wait;
  await retryPolicy.sleep(wait);
  return true;
}

const WRITES = new Set(['POST', 'PATCH', 'PUT', 'DELETE']);

// fetch with retries. A 429 or a rate-limit 403 is retried for every method: GitHub did not
// apply the call. A 5xx or a network error is retried for reads only, since a write may
// have landed and a retry could post a comment or a status twice. Returns the last
// response; the caller decides what a non-OK status means.
export async function ghFetch(url, init = {}) {
  const write = WRITES.has((init.method || 'GET').toUpperCase());
  for (let attempt = 1; ; attempt++) {
    const last = attempt >= retryPolicy.attempts;
    let res;
    try {
      res = await fetch(url, init);
    } catch (e) {
      if (write || last || !(await backoff(retryPolicy.baseMs * 2 ** (attempt - 1)))) throw e;
      continue;
    }
    const retryAfter = res.headers.get('retry-after');
    const rateLimited = res.status === 429 ||
      (res.status === 403 && (retryAfter !== null || res.headers.get('x-ratelimit-remaining') === '0'));
    if (last || !(rateLimited || (!write && res.status >= 500))) return res;
    const wait = Number(retryAfter) > 0 ? Number(retryAfter) * 1000 : retryPolicy.baseMs * 2 ** (attempt - 1);
    if (!(await backoff(wait))) return res;
  }
}

// REST GET with Link-header pagination. Returns the concatenated array.
export async function restAll(path) {
  const out = [];
  let url = `https://api.github.com${path}${path.includes('?') ? '&' : '?'}per_page=100`;
  while (url) {
    const res = await ghFetch(url, {
      headers: { authorization: `bearer ${token()}`, accept: 'application/vnd.github+json', 'user-agent': 'openrisk-board-sync' },
    });
    if (res.status === 404) return out;
    if (!res.ok) throw new Error(`REST ${path}: HTTP ${res.status}`);
    out.push(...(await res.json()));
    const next = /<([^>]+)>;\s*rel="next"/.exec(res.headers.get('link') || '');
    url = next ? next[1] : null;
  }
  return out;
}

// Runs a list of mutation fragments as aliased batches of `size` per request.
// Each fragment is a string like `updateProjectV2ItemFieldValue(input: {...}) { clientMutationId }`.
// Returns one result per fragment, in order.
export async function mutateBatched(fragments, size = 20) {
  const out = [];
  for (let i = 0; i < fragments.length; i += size) {
    const chunk = fragments.slice(i, i + size);
    const q = `mutation { ${chunk.map((f, j) => `m${j}: ${f}`).join('\n')} }`;
    const d = await gql(q);
    chunk.forEach((_, j) => out.push(d[`m${j}`]));
  }
  return out;
}

// GraphQL string literal.
export const lit = (s) => JSON.stringify(String(s));

export function git(args) {
  return execFileSync('git', args, { cwd: ROOT, encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }).trim();
}

export function gitOk(args) {
  try {
    execFileSync('git', args, { cwd: ROOT, stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}
