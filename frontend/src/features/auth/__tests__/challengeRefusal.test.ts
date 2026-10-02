// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only

import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios';
import { describe, expect, it } from 'vitest';

import { classifyChallengeRefusal, minutesToWait } from '../challengeRefusal';

function refused(status: number, data: unknown, headers: Record<string, string> = {}): AxiosError {
  const response = {
    status,
    statusText: '',
    data,
    headers,
    config: { headers: new AxiosHeaders() },
  } as AxiosResponse;
  return new AxiosError('refused', String(status), response.config, null, response);
}

describe('classifyChallengeRefusal (#872)', () => {
  it('reads a wrong code as a wrong code', () => {
    expect(classifyChallengeRefusal(refused(400, { error: 'invalid MFA code' }))).toEqual({
      kind: 'invalid',
    });
  });

  it('sends a spent or revoked token back to the password', () => {
    for (const code of ['MFA_CHALLENGE_EXHAUSTED', 'TOKEN_REVOKED']) {
      expect(classifyChallengeRefusal(refused(401, { code }))).toEqual({
        kind: 'restart',
        reason: 'exhausted',
      });
    }
  });

  it('sends an expired or missing token back to the password', () => {
    for (const code of ['TOKEN_EXPIRED', 'TOKEN_INVALID', 'UNAUTHORIZED', 'SOMETHING_NEW']) {
      expect(classifyChallengeRefusal(refused(401, { code }))).toEqual({
        kind: 'restart',
        reason: 'expired',
      });
    }
  });

  it('reads the wait from the body first, then from Retry-After', () => {
    expect(classifyChallengeRefusal(refused(429, { code: 'MFA_LOCKED', retry_after: 900 }))).toEqual({
      kind: 'locked',
      retryAfterSeconds: 900,
    });
    expect(
      classifyChallengeRefusal(refused(429, { code: 'MFA_LOCKED' }, { 'retry-after': '120' })),
    ).toEqual({ kind: 'locked', retryAfterSeconds: 120 });
  });

  it('treats the per-address limit as a wait of unknown length', () => {
    expect(classifyChallengeRefusal(refused(429, { error: true, msg: 'Rate limit exceeded' }))).toEqual({
      kind: 'locked',
      retryAfterSeconds: null,
    });
  });

  it('falls back to "invalid" when no response came back', () => {
    expect(classifyChallengeRefusal(new Error('network'))).toEqual({ kind: 'invalid' });
  });

  it('rounds the wait up to whole minutes and never says zero', () => {
    expect(minutesToWait(900)).toBe(15);
    expect(minutesToWait(61)).toBe(2);
    expect(minutesToWait(1)).toBe(1);
  });
});
