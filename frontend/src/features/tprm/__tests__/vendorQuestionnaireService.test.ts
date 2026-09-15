// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The public questionnaire client must carry the token in a header, and nothing
// else: no session cookie, no bearer token, no CSRF header (#674, ADR 0004 D4).

import { afterEach, describe, expect, it } from 'vitest';
import { AxiosError, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios';

import {
  VENDOR_TOKEN_HEADER,
  classifyQuestionnaireError,
  publicQuestionnaireApi,
  vendorQuestionnaireService,
} from '../vendorQuestionnaireService';
import { setAccessToken } from '../../../lib/session';

function captureRequests(): InternalAxiosRequestConfig[] {
  const seen: InternalAxiosRequestConfig[] = [];
  publicQuestionnaireApi.defaults.adapter = async (config) => {
    seen.push(config);
    return {
      data: { items: [] },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    } as AxiosResponse;
  };
  return seen;
}

describe('vendorQuestionnaireService', () => {
  afterEach(() => {
    setAccessToken(null);
  });

  it('sends the token in the header, never in the URL, and no session even when one exists', async () => {
    setAccessToken('staff-session-token');
    const seen = captureRequests();
    const token = 'tok_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq';

    await vendorQuestionnaireService.get(token);
    await vendorQuestionnaireService.saveAnswers(token, [{ item_id: 'i-1', answer_value: 'yes' }]);
    await vendorQuestionnaireService.submit(token);

    expect(seen).toHaveLength(3);
    for (const config of seen) {
      expect(config.headers.get(VENDOR_TOKEN_HEADER)).toBe(token);
      expect(config.headers.get('Authorization')).toBeFalsy();
      expect(config.withCredentials).toBe(false);
      expect(`${config.baseURL ?? ''}${config.url ?? ''}`).not.toContain(token);
      expect(JSON.stringify(config.params ?? {})).not.toContain(token);
    }
    expect(seen.map((c) => [c.method, c.url])).toEqual([
      ['get', '/public/vendor-assessment'],
      ['put', '/public/vendor-assessment/answers'],
      ['post', '/public/vendor-assessment/submit'],
    ]);
  });

  it('has no request interceptor that could attach a credential', () => {
    const interceptors = publicQuestionnaireApi.interceptors.request as unknown as {
      handlers: ReadonlyArray<unknown>;
    };
    expect(interceptors.handlers.filter(Boolean)).toHaveLength(0);
  });

  it.each([
    [404, 'invalid'],
    [410, 'gone'],
    [409, 'locked'],
    [429, 'rate_limited'],
    [400, 'rejected'],
    [500, 'unavailable'],
  ] as const)('classifies %i as %s', (status, expected) => {
    const response = { status, data: {}, statusText: '', headers: {} } as AxiosResponse;
    const err = new AxiosError('failed', 'ERR_BAD_RESPONSE', undefined, undefined, response);
    expect(classifyQuestionnaireError(err)).toBe(expected);
  });

  it('treats a network failure or a non-HTTP error as unavailable', () => {
    expect(classifyQuestionnaireError(new AxiosError('Network Error', 'ERR_NETWORK'))).toBe(
      'unavailable',
    );
    expect(classifyQuestionnaireError(new Error('boom'))).toBe('unavailable');
  });
});
