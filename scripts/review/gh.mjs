// Small GitHub REST helpers for the review gate. The token comes from scripts/board/lib.mjs:
// BOARD_TOKEN / GH_TOKEN / GITHUB_TOKEN, else `gh auth token`.
import { token, restAll } from '../board/lib.mjs';

export const [OWNER, REPO] = (process.env.GITHUB_REPOSITORY || 'opendefender/OpenRisk').split('/');
export const R = `/repos/${OWNER}/${REPO}`;
export { restAll };

export async function rest(method, path, body) {
  const res = await fetch(`https://api.github.com${path}`, {
    method,
    headers: { authorization: `bearer ${token()}`, accept: 'application/vnd.github+json', 'user-agent': 'openrisk-review-gate', 'content-type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok && res.status !== 404) throw new Error(`${method} ${path}: HTTP ${res.status} ${(await res.text()).slice(0, 200)}`);
  return res.status === 204 || res.status === 404 ? null : res.json();
}

export const setStatus = (sha, context, state, description, url) =>
  rest('POST', `${R}/statuses/${sha}`, { state, context, description: description.slice(0, 140), ...(url ? { target_url: url } : {}) });

// Latest status per context on a SHA: { context: state }.
export async function statuses(sha) {
  const all = await restAll(`${R}/commits/${sha}/statuses`);
  const out = {};
  for (const s of all) if (!(s.context in out)) out[s.context] = s.state; // newest first
  return out;
}

export const comments = (n) => restAll(`${R}/issues/${n}/comments`);

export async function swapLabels(issue, { add, remove }) {
  for (const l of add) await rest('POST', `${R}/issues/${issue}/labels`, { labels: [l] });
  for (const l of remove) await rest('DELETE', `${R}/issues/${issue}/labels/${encodeURIComponent(l)}`);
}

export const arg = (name, argv = process.argv) => (argv.includes(`--${name}`) ? argv[argv.indexOf(`--${name}`) + 1] : undefined);
