// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import axios from 'axios';
import { CSRF_HEADER, getAccessToken, getCsrfToken, setAccessToken } from './session';
import { markApiFailure, markApiSuccess } from './connection';

/**
 * API base URL.
 *
 * Relative by default so requests are same-origin: the Vite dev server proxies
 * /api to the backend (see vite.config.ts) and a production deployment serves
 * both behind one host. Same-origin is what lets the HttpOnly session cookies
 * ride along on every call.
 *
 * VITE_API_URL stays available for deployments that genuinely split the origins.
 * Those need the backend CORS allowlist to name the SPA origin, and cookies then
 * require SameSite=None over HTTPS.
 */
const baseURL = import.meta.env.VITE_API_URL ?? '/api/v1';

export const api = axios.create({
  baseURL,
  headers: { 'Content-Type': 'application/json' },
  // Send the HttpOnly session cookies. Without this axios omits credentials and
  // the session silently fails to authenticate.
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  // The cookie is the primary credential; this header is a fallback for a
  // split-origin deployment where the cookie cannot be attached. The token is
  // held in memory only — never in localStorage, which is the exposure this
  // migration removes.
  const token = getAccessToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }

  // CSRF double-submit, required by the backend on cookie-authenticated state
  // changes. Safe methods are exempt.
  const method = (config.method ?? 'get').toUpperCase();
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const csrf = getCsrfToken();
    if (csrf) {
      config.headers[CSRF_HEADER] = csrf;
    }
  }

  return config;
});

// Gestion automatique de l'expiration (401)
// Only the RS256 auth middleware itself (missing/expired/revoked/invalid token, or a token
// missing tenant_id) sets a `code` field on its 401 response. Several other server-side checks
// (broken or not) also return 401 without that field — e.g. a missing permission, or a route
// whose guard is misconfigured. Redirecting to /login on *any* 401 logs the user out for those
// too, even though their session is perfectly valid. Only force logout on genuine token failures.
const TOKEN_ERROR_CODES = new Set(['TOKEN_EXPIRED', 'TOKEN_REVOKED', 'TOKEN_INVALID', 'UNAUTHORIZED']);

// A single in-flight refresh shared by every request that 401s at once, so a
// dashboard full of widgets that all expire together triggers ONE /auth/refresh,
// not one per widget. Cleared when it settles so a later expiry can refresh
// again. Returns whether a usable session was re-established.
let refreshInFlight: Promise<boolean> | null = null;

function refreshSession(): Promise<boolean> {
  if (!refreshInFlight) {
    // Bare axios (not `api`) so the refresh call never recurses through this
    // interceptor. Cookie-based: the HttpOnly refresh cookie authenticates it;
    // the CSRF header satisfies the double-submit check on this state change.
    const csrf = getCsrfToken();
    refreshInFlight = axios
      .post(
        `${baseURL}/auth/refresh`,
        {},
        { withCredentials: true, headers: csrf ? { [CSRF_HEADER]: csrf } : {} },
      )
      .then((res) => {
        const token = res.data?.token_pair?.access_token as string | undefined;
        if (token) setAccessToken(token);
        return res.status === 200;
      })
      .catch(() => false);
    // Null it out once settled — regardless of outcome — so the NEXT expiry
    // starts a fresh refresh instead of reusing this resolved promise.
    void refreshInFlight.finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}

api.interceptors.response.use(
  (response) => {
    // Feeds the header's connection indicator with a real observation.
    markApiSuccess();
    return response;
  },
  async (error) => {
    if (error.response) {
      // The backend answered — the connection is fine even if the answer is 4xx.
      markApiSuccess();
    } else {
      // No response at all: refused, timed out, DNS, CORS. That is a real
      // connection fault, and the only thing the status dot may go amber for.
      markApiFailure();
    }

    const status = error.response?.status;
    const code = error.response?.data?.code;
    const original = error.config as (typeof error.config & { _retried?: boolean }) | undefined;

    // Transparent refresh-and-retry: an access token that merely EXPIRED is
    // recoverable from the long-lived refresh cookie. Without this, every
    // fetch-on-mount widget (dashboard KPIs, score gauge, …) silently rendered
    // zeros — or bounced the user to /login — the moment the 15-minute access
    // token lapsed, even though a valid session cookie was one refresh away.
    // Retry at most once per request, and never for the refresh call itself.
    if (
      status === 401 &&
      code === 'TOKEN_EXPIRED' &&
      original &&
      !original._retried &&
      !String(original.url ?? '').includes('/auth/refresh')
    ) {
      const ok = await refreshSession();
      if (ok) {
        original._retried = true;
        return api(original); // replays with the refreshed token + cookies
      }
      // Refresh genuinely failed: the session is over, send them to login.
      window.location.href = '/login';
      return Promise.reject(error);
    }

    // A revoked/invalid token (or a bare UNAUTHORIZED) is not refreshable.
    if (status === 401 && TOKEN_ERROR_CODES.has(code)) {
      window.location.href = '/login'; // Redirection forcée
    }

    // OR26-03 — the session is valid but the account's MFA grace period is over,
    // so the server is refusing everything except the enrolment endpoints.
    //
    // Deliberately NOT a redirect to /login: signing the user out would drop
    // them at a password form that would hand back the very same enrolment
    // demand, with nothing on screen explaining why. Instead, invalidate the
    // cached MFA state so the banner re-reads it and escalates to its blocking
    // copy — the enrolment dialog is one click away from there, and the user
    // stays on the screen they were on.
    if (status === 403 && code === 'MFA_ENROLLMENT_REQUIRED') {
      notifyMFARequired();
    }
    return Promise.reject(error);
  },
);
// ---------------------------------------------------------------------------
// OR26-03 — MFA enforcement signal
// ---------------------------------------------------------------------------

/**
 * Subscribers notified when the server refuses a request because the account's
 * MFA grace period has expired.
 *
 * A callback registry rather than a direct import of the query client, because
 * lib/api is imported by the query layer itself and the reverse dependency
 * would be a cycle. The app registers one subscriber at start-up.
 *
 * This is a UI signal only. It cannot grant access, and ignoring it changes
 * nothing: the server has already refused, and will refuse the next request too.
 */
const mfaRequiredListeners = new Set<() => void>();

/** Registers a listener. Returns the unsubscribe function. */
export function onMFARequired(fn: () => void): () => void {
  mfaRequiredListeners.add(fn);
  return () => mfaRequiredListeners.delete(fn);
}

function notifyMFARequired(): void {
  mfaRequiredListeners.forEach((fn) => {
    try {
      fn();
    } catch {
      // A misbehaving listener must not swallow the original API error.
    }
  });
}
