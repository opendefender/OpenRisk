// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Pure helpers of the threat list (#911). The relevance itself is computed by
// the backend (domain.ThreatRelevance); nothing here re-derives it.

import type { Threat } from './ctiService';
import { SOURCE_LABEL } from '../vulnerabilities/vulnMeta';
import type { VulnSource } from '../vulnerabilities/vulnerabilityService';

/** The "Pertinence ≥ 70" view (backend domain.ThreatRelevanceHigh). */
export const RELEVANCE_HIGH = 70;

export type ThreatMode = 'all' | 'high';

export function visibleThreats(threats: readonly Threat[], mode: ThreatMode): Threat[] {
  return mode === 'all' ? [...threats] : threats.filter((t) => t.relevance >= RELEVANCE_HIGH);
}

/** The design's three tones: ≥ 85 danger, ≥ 60 warning, else quiet. */
export function relevanceTone(relevance: number): string {
  if (relevance >= 85) return 'text-danger-text';
  if (relevance >= 60) return 'text-warning-text';
  return 'text-ink-soft';
}

/**
 * A card's heading: the scanner's title when one found the flaw, else the
 * first sentence of the catalogue description, else the CVE id. Never made up.
 */
export function threatTitle(t: Pick<Threat, 'title' | 'description' | 'cve_id'>): string {
  if (t.title.trim()) return t.title.trim();
  const d = t.description.trim();
  if (!d) return t.cve_id;
  const first = d.split(/(?<=[.!?])\s/)[0] ?? d;
  return first.length > 140 ? `${first.slice(0, 139).trimEnd()}…` : first;
}

const FEED_LABEL: Record<string, string> = {
  nvd: 'NVD',
  cisa_kev: 'CISA KEV',
  mitre_attack: 'MITRE ATT&CK',
};

/** Source codes as names: the feeds, then the scanners that reported it. */
export function sourceLabels(sources: readonly string[]): string[] {
  return sources.map((s) => FEED_LABEL[s] ?? SOURCE_LABEL[s as VulnSource] ?? s);
}
