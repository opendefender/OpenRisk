// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Regression tests for the api client's transparent refresh-and-retry on an
// expired access token (audit-2026 #242): without it, every fetch-on-mount
// widget rendered zeros the moment the 15-minute access token lapsed.

import axios from 'axios';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { api } from '../api';
import { getAccessToken, setAccessToken } from '../session';

type MockCall = (config: unknown) => Promise<unknown>;

const origAdapter = api.defaults.adapter;

beforeEach(() => {
  setAccessToken(null);
  // Stub window.location so the failure path's redirect does not blow up jsdom.
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { href: '' },
    writable: true,
  });
});

afterEach(() => {
  api.defaults.adapter = origAdapter;
  vi.restoreAllMocks();
});

function expired(config: unknown, code = 'TOKEN_EXPIRED') {
  const err = new Error('401') as Error & { response?: unknown; config?: unknown };
  err.response = {
    status: 401,
    data: { code },
    headers: {},
    config,
    statusText: '',
  };
  err.config = config;
  return err;
}

function ok(config: unknown, data: unknown) {
  return { status: 200, statusText: 'OK', headers: {}, data, config };
}

describe('api interceptor — refresh & retry on TOKEN_EXPIRED', () => {
  it('refreshes once and replays the request with the new token', async () => {
    const post = vi.spyOn(axios, 'post').mockResolvedValue({
      status: 200,
      data: { token_pair: { access_token: 'fresh-token' } },
    } as never);

    let calls = 0;
    const adapter: MockCall = async (config) => {
      calls += 1;
      if (calls === 1) throw expired(config);
      return ok(config, { total_risks: 11 });
    };
    api.defaults.adapter = adapter as never;

    const res = await api.get('/stats');

    expect(calls).toBe(2); // original 401 + one retry
    expect(post).toHaveBeenCalledTimes(1); // exactly one refresh
    expect((res.data as { total_risks: number }).total_risks).toBe(11);
    expect(getAccessToken()).toBe('fresh-token'); // in-memory token updated
  });

  it('shares a single refresh across concurrent expired requests', async () => {
    const post = vi
      .spyOn(axios, 'post')
      .mockResolvedValue({ status: 200, data: { token_pair: { access_token: 't' } } } as never);

    const seen: Record<string, number> = {};
    const adapter: MockCall = async (config) => {
      const url = (config as { url: string }).url;
      seen[url] = (seen[url] ?? 0) + 1;
      if (seen[url] === 1) throw expired(config);
      return ok(config, { url });
    };
    api.defaults.adapter = adapter as never;

    await Promise.all([api.get('/a'), api.get('/b'), api.get('/c')]);

    // Three requests expired together but only ONE refresh was issued.
    expect(post).toHaveBeenCalledTimes(1);
  });

  it('redirects to /login when the refresh itself fails', async () => {
    vi.spyOn(axios, 'post').mockRejectedValue(new Error('refresh 401'));

    const adapter: MockCall = async (config) => {
      throw expired(config);
    };
    api.defaults.adapter = adapter as never;

    await expect(api.get('/stats')).rejects.toBeTruthy();
    expect(window.location.href).toBe('/login');
  });
});

// #691: the access cookie expires with its token, so after 15 minutes the API
// sees no credential and answers UNAUTHORIZED — never TOKEN_EXPIRED.
describe('api interceptor — an expired access cookie (issue 691)', () => {
  it('refreshes and replays when the API reports no credential (UNAUTHORIZED)', async () => {
    const post = vi.spyOn(axios, 'post').mockResolvedValue({
      status: 200,
      data: { token_pair: { access_token: 'fresh-token' } },
    } as never);

    let calls = 0;
    const adapter: MockCall = async (config) => {
      calls += 1;
      if (calls === 1) throw expired(config, 'UNAUTHORIZED');
      return ok(config, { unread: 3 });
    };
    api.defaults.adapter = adapter as never;

    const res = await api.get('/notifications/unread-count');

    expect(post).toHaveBeenCalledTimes(1);
    expect(calls).toBe(2);
    expect((res.data as { unread: number }).unread).toBe(3);
    expect(window.location.href).toBe('');
  });

  it('does not try to refresh a revoked token', async () => {
    const post = vi.spyOn(axios, 'post');
    const adapter: MockCall = async (config) => {
      throw expired(config, 'TOKEN_REVOKED');
    };
    api.defaults.adapter = adapter as never;

    await expect(api.get('/stats')).rejects.toBeTruthy();
    expect(post).not.toHaveBeenCalled();
    expect(window.location.href).toBe('/login');
  });

  it('sends the user to /login once, without looping, when the replay is still refused', async () => {
    const post = vi
      .spyOn(axios, 'post')
      .mockResolvedValue({ status: 200, data: { token_pair: { access_token: 't' } } } as never);
    let calls = 0;
    const adapter: MockCall = async (config) => {
      calls += 1;
      throw expired(config, 'UNAUTHORIZED');
    };
    api.defaults.adapter = adapter as never;

    await expect(api.get('/stats')).rejects.toBeTruthy();
    expect(post).toHaveBeenCalledTimes(1);
    expect(calls).toBe(2); // the original and one replay, nothing more
    expect(window.location.href).toBe('/login');
  });
});

// #872 — the MFA challenge is authenticated by its own short-lived token, not by
// the session. Its 401s say "this sign-in attempt is over", which the challenge
// screen explains. Refreshing an unrelated session or reloading /login here
// wiped that explanation and left the user on a blank password form.
describe('api interceptor — the MFA challenge owns its 401s (issue 872)', () => {
  for (const code of ['TOKEN_REVOKED', 'TOKEN_EXPIRED', 'TOKEN_INVALID', 'UNAUTHORIZED']) {
    it(`neither refreshes nor redirects on ${code}`, async () => {
      const post = vi.spyOn(axios, 'post');
      let calls = 0;
      const adapter: MockCall = async (config) => {
        calls += 1;
        throw expired(config, code);
      };
      api.defaults.adapter = adapter as never;

      const err = await api.post('/auth/mfa/challenge', { code: '123456' }).catch((e: unknown) => e);
      expect((err as { response?: { data?: { code?: string } } }).response?.data?.code).toBe(code);
      expect(post).not.toHaveBeenCalled();
      expect(calls).toBe(1);
      expect(window.location.href).toBe('');
    });
  }

  it('still sends an ordinary request with a revoked token to /login', async () => {
    const adapter: MockCall = async (config) => {
      throw expired(config, 'TOKEN_REVOKED');
    };
    api.defaults.adapter = adapter as never;

    await expect(api.get('/auth/mfa/status')).rejects.toBeTruthy();
    expect(window.location.href).toBe('/login');
  });
});
