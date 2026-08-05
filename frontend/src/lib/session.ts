// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1

/**
 * Session credential access.
 *
 * The durable credential now lives in HttpOnly cookies the backend sets at
 * login. Script cannot read them, which is the point: an access token in
 * localStorage survives reloads and is readable by anything running on the page,
 * so a single XSS — or one compromised dependency in a large tree — yields a
 * session that outlives the visit.
 *
 * The access token is still held here in memory, for two reasons: call sites
 * that build their own `fetch` need something to send when a deployment serves
 * the API from a different origin than the SPA, and keeping it out of any
 * storage means a reload starts from the cookie rather than from a value an
 * attacker could have planted. Nothing writes it to localStorage or
 * sessionStorage.
 */

let accessToken: string | null = null;

/** Records the token for the lifetime of the tab. Never persisted. */
export function setAccessToken(token: string | null): void {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

/** Name of the readable CSRF cookie the backend issues alongside the session. */
const CSRF_COOKIE = 'or_csrf';

/** Header the backend compares the CSRF cookie against. */
export const CSRF_HEADER = 'X-CSRF-Token';

/**
 * Reads the CSRF token from its cookie.
 *
 * This cookie is deliberately not HttpOnly: the double-submit check requires the
 * page to read it and echo it in a header. Its security comes from the
 * same-origin policy — another origin can cause the cookie to be *sent* but
 * cannot *read* it, so it cannot build the matching header.
 */
export function getCsrfToken(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)or_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}

export { CSRF_COOKIE };

/**
 * Headers for a hand-rolled `fetch` that must carry the session.
 *
 * Prefer the shared axios client, which applies these automatically. This exists
 * for the call sites that predate it.
 */
export function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
  const headers: Record<string, string> = { ...extra };

  const token = getAccessToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  const csrf = getCsrfToken();
  if (csrf) headers[CSRF_HEADER] = csrf;

  return headers;
}
