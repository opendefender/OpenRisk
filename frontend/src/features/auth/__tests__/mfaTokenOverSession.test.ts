// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// GHSA-45mr-j2w6-m4jv — an organisation switch reaches the MFA step with the
// current organisation's session token in memory. The MFA calls go through the
// real authService and api client here, so the header the server would see is
// the one asserted: the MFA token for the target, never the session token.

import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { api } from '../../../lib/api';
import { setAccessToken } from '../../../lib/session';
import { challengeMFA, setupMFA, verifyMFA } from '../authService';

const origAdapter = api.defaults.adapter;
let sent: string[] = [];

beforeEach(() => {
  sent = [];
  setAccessToken('session-token-for-a');
  api.defaults.adapter = (async (config: { headers: { Authorization?: string } }) => {
    sent.push(String(config.headers.Authorization));
    return { status: 200, statusText: 'OK', headers: {}, data: {}, config };
  }) as never;
});

afterEach(() => {
  api.defaults.adapter = origAdapter;
  setAccessToken(null);
});

describe('MFA calls after an organisation switch, with a session in memory', () => {
  it('enrols with the target organisation’s enrolment token', async () => {
    await setupMFA('enrol-token-for-b');
    await verifyMFA('123456', 'enrol-token-for-b');
    expect(sent).toEqual(['Bearer enrol-token-for-b', 'Bearer enrol-token-for-b']);
  });

  it('answers the challenge with the target organisation’s MFA token', async () => {
    await challengeMFA('123456', 'mfa-token-for-b');
    expect(sent).toEqual(['Bearer mfa-token-for-b']);
  });

  it('enrols from Settings on the session, as before', async () => {
    await setupMFA();
    expect(sent).toEqual(['Bearer session-token-for-a']);
  });
});
