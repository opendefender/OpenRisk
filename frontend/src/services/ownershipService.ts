// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import { api } from '../lib/api';

/**
 * The three accountability slots every actionable entity carries.
 *
 *  owner    — responsable, answers for the outcome
 *  assignee — exécutant, does the work
 *  reviewer — validateur, signs it off
 *
 * They are deliberately distinct: a single "assigned_to" cannot say both who
 * answers for something and who is doing it, which is why assigning a risk
 * never produced anything a filter could later find.
 */
export type OwnershipRole = 'owner' | 'assignee' | 'reviewer';

/** The block as it arrives on every entity (flat, not nested). */
export interface Ownership {
  owner_id?: string | null;
  assignee_id?: string | null;
  reviewer_id?: string | null;
  // Resolved server-side in one batched lookup; empty when unavailable.
  owner_email?: string;
  assignee_email?: string;
  reviewer_email?: string;
}

/** One pickable person. */
export interface Assignee {
  user_id: string;
  email: string;
  full_name: string;
  initials: string;
  org_role: string;
  business_role?: string;
  business_role_label?: string;
  is_active: boolean;
  /**
   * Whether this member actually holds the permission the caller asked about.
   * The picker shows the answer rather than hiding the person, so "why can't I
   * assign Amina?" has a visible answer.
   */
  can_act: boolean;
}

/** A role-shaped bucket ("the RSSIs", "the admins"). */
export interface AssignableGroup {
  key: string;
  label: string;
  members: string[];
  count: number;
}

export interface AssignableResult {
  users: Assignee[];
  groups: AssignableGroup[];
  permission?: string;
}

export interface AssignableQuery {
  q?: string;
  /** e.g. "risks:update" — flags (or filters to) members who can do the work. */
  permission?: string;
  only_capable?: boolean;
}

export const ownershipService = {
  /**
   * The source of truth for <UserPicker>. Deliberately the SAME list the API
   * validates against, so what the picker offers and what the server accepts
   * cannot drift apart.
   */
  async listAssignable(
    params: AssignableQuery = {},
    signal?: AbortSignal,
  ): Promise<AssignableResult> {
    const { data } = await api.get<AssignableResult>('/ownership/assignable', { params, signal });
    return data;
  },

  /** The caller's own row, so the picker can offer "Moi" without a second call. */
  async me(): Promise<Assignee> {
    const { data } = await api.get<Assignee>('/ownership/me');
    return data;
  },
};

/**
 * Build the tri-state patch the API expects.
 *
 * `undefined` means "leave this slot alone" and is omitted from the payload;
 * `null` means "unassign". Without that distinction, saving a form that does
 * not include a reviewer would silently unassign the reviewer.
 */
export function ownershipPatch(
  changes: Partial<Record<OwnershipRole, string | null>>,
): Record<string, string | null> {
  const patch: Record<string, string | null> = {};
  if ('owner' in changes) patch.owner_id = changes.owner ?? null;
  if ('assignee' in changes) patch.assignee_id = changes.assignee ?? null;
  if ('reviewer' in changes) patch.reviewer_id = changes.reviewer ?? null;
  return patch;
}

/** Read a slot off an entity carrying the flat block. */
export function slotOf(entity: Ownership | undefined, role: OwnershipRole): string | null {
  if (!entity) return null;
  if (role === 'owner') return entity.owner_id ?? null;
  if (role === 'assignee') return entity.assignee_id ?? null;
  return entity.reviewer_id ?? null;
}

/** The email the server resolved for a slot, when it could. */
export function slotEmail(entity: Ownership | undefined, role: OwnershipRole): string {
  if (!entity) return '';
  if (role === 'owner') return entity.owner_email ?? '';
  if (role === 'assignee') return entity.assignee_email ?? '';
  return entity.reviewer_email ?? '';
}

export const ROLE_LABELS: Record<
  OwnershipRole,
  { fr: string; en: string; hint_fr: string; hint_en: string }
> = {
  owner: {
    fr: 'Responsable',
    en: 'Owner',
    hint_fr: 'Répond du résultat.',
    hint_en: 'Answers for the outcome.',
  },
  assignee: {
    fr: 'Exécutant',
    en: 'Assignee',
    hint_fr: 'Fait le travail.',
    hint_en: 'Does the work.',
  },
  reviewer: {
    fr: 'Validateur',
    en: 'Reviewer',
    hint_fr: 'Valide une fois terminé.',
    hint_en: 'Signs it off when done.',
  },
};
