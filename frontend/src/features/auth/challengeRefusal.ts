// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// What a refused MFA challenge means for the person at the screen (#872).
//
// Since #689 the server refuses a code in more ways than "wrong code", and each
// one calls for a different move:
//
//   401 MFA_CHALLENGE_EXHAUSTED   this sign-in used its five codes
//   401 TOKEN_REVOKED             the same, on any later attempt
//   401 TOKEN_EXPIRED / _INVALID  the challenge token outlived its five minutes
//   401 UNAUTHORIZED              no usable challenge token at all
//   429 MFA_LOCKED                the account refuses codes for a while
//   429 (anything else)           the per-address limit on the route
//
// Showing "incorrect code" for all of them kept people typing codes into a
// sign-in attempt the server had already closed.

import axios from 'axios';

export type ChallengeRefusal =
  /** A wrong code. The same token can take another one. */
  | { kind: 'invalid' }
  /** The sign-in attempt is over. Only the password can start a new one. */
  | { kind: 'restart'; reason: 'exhausted' | 'expired' }
  /** Codes are refused for a while. `retryAfterSeconds` is null when unknown. */
  | { kind: 'locked'; retryAfterSeconds: number | null };

interface RefusalBody {
  code?: unknown;
  retry_after?: unknown;
}

function positiveSeconds(v: unknown): number | null {
  const n = typeof v === 'string' ? Number.parseInt(v, 10) : v;
  return typeof n === 'number' && Number.isFinite(n) && n > 0 ? Math.ceil(n) : null;
}

export function classifyChallengeRefusal(err: unknown): ChallengeRefusal {
  if (!axios.isAxiosError(err) || !err.response) return { kind: 'invalid' };

  const { status, data, headers } = err.response;
  const body = (data ?? {}) as RefusalBody;
  const code = typeof body.code === 'string' ? body.code.toUpperCase() : '';

  if (status === 429) {
    const header = (headers as Record<string, unknown> | undefined)?.['retry-after'];
    return {
      kind: 'locked',
      retryAfterSeconds: positiveSeconds(body.retry_after) ?? positiveSeconds(header),
    };
  }

  if (status !== 401) return { kind: 'invalid' };

  switch (code) {
    case 'MFA_CHALLENGE_EXHAUSTED':
    case 'TOKEN_REVOKED':
      return { kind: 'restart', reason: 'exhausted' };
    default:
      // TOKEN_EXPIRED, TOKEN_INVALID, UNAUTHORIZED, and any 401 not known yet:
      // this token will not be accepted again, so asking for another code
      // would only fail again.
      return { kind: 'restart', reason: 'expired' };
  }
}

/** Whole minutes to wait, never zero: "0 minutes" reads as "now". */
export function minutesToWait(seconds: number): number {
  return Math.max(1, Math.ceil(seconds / 60));
}
