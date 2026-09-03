// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// OR26-03 — the MFA state the client is allowed to know, and the organization
// policy behind it.
//
// THE RULE THIS FILE ENFORCES: the server decides. Nothing here caches a
// verdict, writes a "skipped" flag, or derives "am I required?" from a role. The
// client asks /auth/me and renders what comes back; if it lies to itself, the
// request-time guard refuses the very next call anyway.

import { api } from '../../lib/api';
import type { components } from '../../types/openapi.generated';

/** Generated from openapi.yaml — see components.schemas.MFAStatus. */
export type MFAStatus = components['schemas']['MFAStatus'];
/** The five states the banner renders. Derived, never re-declared. */
export type MFAState = NonNullable<MFAStatus['state']>;
export type MFAPolicy = components['schemas']['MFAPolicy'];

/**
 * Reads the caller's MFA state from the session endpoint.
 *
 * Deliberately NOT a dedicated endpoint: /auth/me already exists, already runs
 * on every session restore, and already carries the identity this depends on.
 * A second call per page would be a request per navigation for a value that
 * changes at most a handful of times in an account's life.
 *
 * Returns null when the server omits the block — it does that when it cannot
 * resolve the state rather than guessing, and "unknown" must not render as
 * "you're fine".
 */
export async function fetchMFAStatus(): Promise<MFAStatus | null> {
  const { data } = await api.get<{ mfa?: MFAStatus }>('/auth/me');
  return data?.mfa ?? null;
}

/** Reads the organization's grace policy. Open to any authenticated member. */
export async function fetchMFAPolicy(): Promise<MFAPolicy> {
  const { data } = await api.get<MFAPolicy>('/security/mfa-policy');
  return data;
}

/** Saves the organization's grace policy. Admin only — the server enforces it. */
export async function saveMFAPolicy(graceDays: number): Promise<MFAPolicy> {
  const { data } = await api.put<MFAPolicy>('/security/mfa-policy', { grace_days: graceDays });
  return data;
}

/** Days remaining before enrolment becomes mandatory, or null when it never does. */
export function daysUntilDeadline(
  status: MFAStatus | null | undefined,
  now = new Date(),
): number | null {
  if (!status?.deadline) return null;
  const deadline = new Date(status.deadline);
  if (Number.isNaN(deadline.getTime())) return null;
  // Ceiling, so "18 hours left" reads as 1 day rather than 0 — a countdown that
  // says zero while access still works teaches people to ignore it.
  return Math.max(0, Math.ceil((deadline.getTime() - now.getTime()) / 86_400_000));
}

/** Whether the banner has anything to say at all. */
export function shouldPromptEnrollment(status: MFAStatus | null | undefined): boolean {
  return !!status && status.state !== 'configured';
}
