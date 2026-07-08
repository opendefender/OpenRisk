// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
// This Source Code Form is subject to the terms of the Business Source License, Version 1.1.
// If a copy of the BUSL was not distributed with this file, You can obtain one at https://mariadb.com/bsl11/

import axios from 'axios';

export const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1',
  headers: { 'Content-Type': 'application/json' },
});

// Injection automatique du Token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('auth_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
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
      localStorage.removeItem('auth_token');
      window.location.href = '/login'; // Redirection forcée
    }
    return Promise.reject(error);
  }
);