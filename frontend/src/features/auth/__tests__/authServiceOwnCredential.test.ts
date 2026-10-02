// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #872 — the MFA calls that carry their own token must say so, or the response
// interceptor treats their 401s as a lost session and reloads /login.

import { beforeEach, describe, expect, it, vi } from 'vitest';

const post = vi.fn();
vi.mock('../../../lib/api', () => ({
  api: { post: (...a: unknown[]) => post(...a) },
}));

import { challengeMFA, setupMFA, verifyMFA } from '../authService';

function configOfLastCall(): { ownCredential?: boolean; headers?: Record<string, string> } | undefined {
  return post.mock.calls.at(-1)?.[2] as ReturnType<typeof configOfLastCall>;
}

beforeEach(() => {
  post.mockReset();
  post.mockResolvedValue({ data: {} });
});

describe('MFA requests and the session (#872)', () => {
  it('marks the challenge as carrying its own credential', async () => {
    await challengeMFA('123456', 'challenge-token');
    expect(configOfLastCall()).toEqual({
      headers: { Authorization: 'Bearer challenge-token' },
      ownCredential: true,
    });
  });

  it('marks mandated enrolment the same way', async () => {
    await setupMFA('enrol-token');
    expect(configOfLastCall()?.ownCredential).toBe(true);
    await verifyMFA('123456', 'enrol-token');
    expect(configOfLastCall()?.ownCredential).toBe(true);
  });

  it('leaves enrolment from Settings on the session', async () => {
    await setupMFA();
    expect(configOfLastCall()).toBeUndefined();
    await verifyMFA('123456');
    expect(configOfLastCall()).toBeUndefined();
  });
});
