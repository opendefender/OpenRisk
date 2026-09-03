// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Typed client for the unauthenticated auth endpoints (password reset, strength
// checking) and for session management. No `any`.

import { api } from '../../lib/api';
import type { Lang } from '../../store/uiStore';

// ---------------------------------------------------------------------------
// Password policy
// ---------------------------------------------------------------------------

/** One policy finding. Mirrors pkg/pwpolicy.Reason. */
export interface PolicyReason {
  code: string;
  fr: string;
  en: string;
}

/** The server's verdict on a candidate password. Mirrors pkg/pwpolicy.Assessment. */
export interface PasswordAssessment {
  ok: boolean;
  score: number;
  min_score: number;
  length: number;
  breached: boolean;
  breach_count: number;
  breach_check_skipped: boolean;
  blocking: PolicyReason[] | null;
  advisory: PolicyReason[] | null;
}

/** Picks the rendering for the current language. */
export function reasonText(reason: PolicyReason, lang: Lang): string {
  return lang === 'en' ? reason.en : reason.fr;
}

/**
 * Asks the server to score a password.
 *
 * The browser also scores locally for instant feedback, but this is the
 * authority: it runs the same pwpolicy.Policy the write paths run, including the
 * HaveIBeenPwned lookup the client cannot do. Without it the meter could say
 * "strong" and the submit could still be refused.
 */
export async function checkPassword(
  password: string,
  identity: { email?: string; name?: string },
  signal?: AbortSignal,
): Promise<PasswordAssessment> {
  const { data } = await api.post<PasswordAssessment>(
    '/auth/password/check',
    { password, email: identity.email ?? '', name: identity.name ?? '' },
    { signal },
  );
  return data;
}

// ---------------------------------------------------------------------------
// Password reset
// ---------------------------------------------------------------------------

export interface ForgotPasswordResult {
  message: string;
}

/**
 * Requests a reset link.
 *
 * Resolves identically whether or not the address has an account — that is the
 * server's contract, and the UI must not try to infer anything more from it.
 */
export async function requestPasswordReset(
  email: string,
  locale: Lang,
): Promise<ForgotPasswordResult> {
  const { data } = await api.post<ForgotPasswordResult>('/auth/password/forgot', { email, locale });
  return data;
}

export interface ResetPasswordResult {
  message: string;
  sessions_revoked: boolean;
}

/** Sets a new password from a reset link. */
export async function resetPassword(
  token: string,
  newPassword: string,
  locale: Lang,
): Promise<ResetPasswordResult> {
  const { data } = await api.post<ResetPasswordResult>('/auth/password/reset', {
    token,
    new_password: newPassword,
    locale,
  });
  return data;
}

/** Shape of the 422 body when the server refuses a password. */
export interface ResetPasswordErrorBody {
  error?: string;
  code?: 'invalid_token' | 'weak_password';
  assessment?: PasswordAssessment;
  retry_after?: number;
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

/** One signed-in device. Mirrors domain.SessionRecord. */
export interface SessionRecord {
  id: string;
  device: string;
  user_agent?: string;
  ip_address?: string;
  created_at: string;
  last_used_at?: string;
  expires_at: string;
  /** True for the device making the request. */
  current: boolean;
}

export async function listSessions(): Promise<SessionRecord[]> {
  const { data } = await api.get<{ sessions: SessionRecord[]; total: number }>('/auth/sessions');
  return data.sessions ?? [];
}

export async function revokeSession(id: string): Promise<void> {
  await api.delete(`/auth/sessions/${id}`);
}

/** Signs out every device except this one. */
export async function revokeOtherSessions(): Promise<number> {
  const { data } = await api.delete<{ revoked: number }>('/auth/sessions/others');
  return data.revoked ?? 0;
}

// ---------------------------------------------------------------------------
// MFA
// ---------------------------------------------------------------------------

export interface MFASetupResult {
  secret: string;
  qr_code: string;
  backup_codes: string[];
}

/**
 * Begins MFA enrolment.
 *
 * `enrollmentToken` is present only on the mandated path: an admin who has just
 * proved their password but holds no session yet. Voluntary enrolment from
 * Settings passes nothing and rides the ordinary session cookie.
 */
export async function setupMFA(enrollmentToken?: string): Promise<MFASetupResult> {
  const { data } = await api.post<MFASetupResult>(
    '/auth/mfa/setup',
    {},
    authHeaders(enrollmentToken),
  );
  return data;
}

/**
 * Confirms a TOTP code and activates MFA.
 *
 * Returns a token pair ONLY on the mandated-enrolment path, where the server
 * completes the half-finished login in the same response. Voluntary enrolment
 * from Settings keeps the session it already has and gets no pair.
 */
export async function verifyMFA(
  code: string,
  enrollmentToken?: string,
): Promise<MFAChallengeResult> {
  const { data } = await api.post<MFAChallengeResult>(
    '/auth/mfa/verify',
    { code },
    authHeaders(enrollmentToken),
  );
  return data;
}

export interface MFAChallengeResult {
  token_pair?: { access_token: string; refresh_token: string };
  csrf_token?: string;
}

export async function challengeMFA(code: string, mfaToken: string): Promise<MFAChallengeResult> {
  const { data } = await api.post<MFAChallengeResult>(
    '/auth/mfa/challenge',
    { code },
    authHeaders(mfaToken),
  );
  return data;
}

/**
 * Bearer header for the short-lived MFA tokens.
 *
 * These are the one place the SPA still carries a token in memory: they exist
 * precisely because no session cookie has been issued yet. They are
 * permission-less, live for minutes, and are never persisted.
 */
function authHeaders(token?: string) {
  return token ? { headers: { Authorization: `Bearer ${token}` } } : undefined;
}
