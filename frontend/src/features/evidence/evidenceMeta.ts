// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import type { EvidenceStatus, EvidenceType, MissingKind } from '../../types/evidence';

/**
 * Status presentation, in one place.
 *
 * Colour is reserved for status here — the project's dataviz rule — so expired
 * and rejected read as the problems they are, while "valid" stays quiet. A
 * register where everything is coloured is a register where nothing stands out.
 */
export const EVIDENCE_STATUS_META: Record<
  EvidenceStatus,
  { color: string; fr: string; en: string; hint: { fr: string; en: string } }
> = {
  valid: {
    color: 'var(--low)',
    fr: 'Valide',
    en: 'Valid',
    hint: { fr: 'Justifie les contrôles rattachés.', en: 'Substantiates the linked controls.' },
  },
  expiring_soon: {
    color: 'var(--medium)',
    fr: 'Expire bientôt',
    en: 'Expiring soon',
    hint: {
      fr: 'Encore valable, à renouveler avant la date.',
      en: 'Still valid, renew before the date.',
    },
  },
  expired: {
    color: 'var(--critical)',
    fr: 'Expirée',
    en: 'Expired',
    hint: {
      fr: 'Ne justifie plus rien : les contrôles rattachés sont découverts.',
      en: 'Substantiates nothing any more: the linked controls are uncovered.',
    },
  },
  rejected: {
    color: 'var(--critical)',
    fr: 'Rejetée',
    en: 'Rejected',
    hint: {
      fr: 'Écartée par un relecteur, quelle que soit sa fraîcheur.',
      en: 'Thrown out by a reviewer, however fresh it is.',
    },
  },
  pending: {
    color: 'var(--medium)',
    fr: 'À relire',
    en: 'Pending review',
    hint: {
      fr: 'Pas encore acceptée : ne compte pas comme preuve.',
      en: 'Not accepted yet: does not count as proof.',
    },
  },
};

export const EVIDENCE_TYPE_META: Record<EvidenceType, { fr: string; en: string }> = {
  document: { fr: 'Document', en: 'Document' },
  capture: { fr: "Capture d'écran", en: 'Screen capture' },
  configuration: { fr: 'Configuration', en: 'Configuration' },
  attestation: { fr: 'Attestation', en: 'Attestation' },
  log: { fr: 'Journal', en: 'Log' },
};

/**
 * The worklist's two states, which are two different jobs.
 *
 * "Never evidenced" means someone has to go and collect proof. "Stale" means the
 * document exists and needs refreshing — usually far less work, and the state
 * that tools counting attachments report as covered.
 */
export const MISSING_KIND_META: Record<
  MissingKind,
  { color: string; fr: string; en: string; action: { fr: string; en: string } }
> = {
  covered: { color: 'var(--low)', fr: 'Couvert', en: 'Covered', action: { fr: '', en: '' } },
  no_evidence: {
    color: 'var(--critical)',
    fr: 'Aucune preuve',
    en: 'No evidence',
    action: { fr: 'Collecter une preuve', en: 'Collect evidence' },
  },
  stale_evidence: {
    color: 'var(--high)',
    fr: 'Preuve périmée',
    en: 'Stale evidence',
    action: { fr: 'Renouveler la preuve', en: 'Refresh the evidence' },
  },
  expiring_soon: {
    color: 'var(--medium)',
    fr: 'Expire bientôt',
    en: 'Expiring soon',
    action: { fr: 'Renouveler avant échéance', en: 'Renew before it lapses' },
  },
};

/** Days until expiry, phrased so a negative number reads as what it is. */
export function expiryLabel(lang: string, days: number | undefined): string | null {
  if (days === undefined || days === null) return null;
  const fr = lang === 'fr';
  if (days < 0) {
    const n = Math.abs(days);
    return fr ? `expirée depuis ${n} j` : `expired ${n}d ago`;
  }
  if (days === 0) return fr ? "expire aujourd'hui" : 'expires today';
  return fr ? `expire dans ${days} j` : `expires in ${days}d`;
}
