// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import axios from 'axios';
import { CSRF_HEADER, getAccessToken, getCsrfToken } from './session';

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

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && TOKEN_ERROR_CODES.has(error.response?.data?.code)) {
      window.location.href = '/login'; // Redirection forcée
    }
    return Promise.reject(error);
  }
);