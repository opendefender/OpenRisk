// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The public vendor questionnaire's API client (#674, ADR 0004 D4).
//
// DELIBERATELY NOT lib/api.ts. That client sends the session cookies, a bearer
// header and the CSRF token on every call. A vendor contact holds no account,
// and this page must not send an OpenRisk session even when the browser happens
// to have one: a staff member opening a vendor link on their own laptop would
// otherwise attach their session to a public route. So this is a bare axios
// instance — no credentials, no interceptors — and its ONLY credential is the
// questionnaire token, in the X-Vendor-Assessment-Token header. Never in the
// URL: paths and query strings are recorded by every access log and proxy.

import axios, { type AxiosInstance } from 'axios';
import type { components } from '../../types/openapi.generated';

export type VendorQuestionnaireView = components['schemas']['VendorAssessmentPublicView'];
export type VendorQuestionnaireItem = components['schemas']['VendorPublicItem'];
export type VendorAnswerInput = components['schemas']['VendorAnswerInput'];

export const VENDOR_TOKEN_HEADER = 'X-Vendor-Assessment-Token';

const baseURL = import.meta.env.VITE_API_URL ?? '/api/v1';

export const publicQuestionnaireApi: AxiosInstance = axios.create({
  baseURL,
  withCredentials: false,
  headers: { 'Content-Type': 'application/json' },
});

function withToken(token: string) {
  return { headers: { [VENDOR_TOKEN_HEADER]: token } };
}

export const vendorQuestionnaireService = {
  async get(token: string): Promise<VendorQuestionnaireView> {
    const { data } = await publicQuestionnaireApi.get<VendorQuestionnaireView>(
      '/public/vendor-assessment',
      withToken(token),
    );
    return data;
  },

  async saveAnswers(token: string, answers: VendorAnswerInput[]): Promise<VendorQuestionnaireView> {
    const { data } = await publicQuestionnaireApi.put<VendorQuestionnaireView>(
      '/public/vendor-assessment/answers',
      { answers },
      withToken(token),
    );
    return data;
  },

  async submit(token: string): Promise<VendorQuestionnaireView> {
    const { data } = await publicQuestionnaireApi.post<VendorQuestionnaireView>(
      '/public/vendor-assessment/submit',
      null,
      withToken(token),
    );
    return data;
  },
};

/** What went wrong, in the terms the page can say something useful about. */
export type QuestionnaireFailure =
  | 'invalid' // 404 — the link names nothing
  | 'gone' // 410 — superseded, withdrawn, expired or unavailable
  | 'locked' // 409 — already submitted
  | 'rate_limited' // 429
  | 'rejected' // 400 — an answer was refused
  | 'unavailable'; // network or server trouble

export function classifyQuestionnaireError(err: unknown): QuestionnaireFailure {
  if (!axios.isAxiosError(err) || !err.response) return 'unavailable';
  switch (err.response.status) {
    case 404:
      return 'invalid';
    case 410:
      return 'gone';
    case 409:
      return 'locked';
    case 429:
      return 'rate_limited';
    case 400:
      return 'rejected';
    default:
      return 'unavailable';
  }
}
