// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The copy for a reached plan cap (D-078), shared by PlanCapNotice and by the
// toast a refused create shows. One sentence per cap, FR and EN, with the
// numbers: a cap that says what it is and how to lift it converts; a bare
// "limit reached" frustrates.

/** The caps a screen can meet, as the entitlements snapshot names them. */
export type PlanCapKey =
  | 'users'
  | 'risks'
  | 'assets'
  | 'frameworks'
  | 'automation_rules'
  | 'scan_sources'
  | 'scans_per_month'
  | 'approval_workflows';

const NOUN: Record<PlanCapKey, { fr: [string, string]; en: [string, string] }> = {
  users: { fr: ['utilisateur', 'utilisateurs'], en: ['user', 'users'] },
  risks: { fr: ['risque', 'risques'], en: ['risk', 'risks'] },
  assets: { fr: ['actif', 'actifs'], en: ['asset', 'assets'] },
  frameworks: {
    fr: ['référentiel actif', 'référentiels actifs'],
    en: ['active framework', 'active frameworks'],
  },
  automation_rules: {
    fr: ['règle d’automatisation active', 'règles d’automatisation actives'],
    en: ['active automation rule', 'active automation rules'],
  },
  scan_sources: { fr: ['source de scan', 'sources de scan'], en: ['scan source', 'scan sources'] },
  scans_per_month: {
    fr: ['scan par mois', 'scans par mois'],
    en: ['scan a month', 'scans a month'],
  },
  approval_workflows: {
    fr: ['circuit d’approbation', 'circuits d’approbation'],
    en: ['approval workflow', 'approval workflows'],
  },
};

/** What lifts the cap, besides upgrading. */
const WAY_OUT: Partial<Record<PlanCapKey, { fr: string; en: string }>> = {
  frameworks: { fr: 'Supprimez-en un', en: 'Delete one' },
  automation_rules: { fr: 'Suspendez-en une', en: 'Suspend one' },
  scan_sources: { fr: 'Supprimez-en une', en: 'Delete one' },
  approval_workflows: { fr: 'Supprimez-en un', en: 'Delete one' },
};

/** The quiet counter under the cap: "Sources de scan : 1/10 sur votre plan". */
export function planCapUsage(fr: boolean, key: PlanCapKey, limit: number, used: number): string {
  const many = NOUN[key][fr ? 'fr' : 'en'][1];
  const label = many.charAt(0).toUpperCase() + many.slice(1);
  return fr
    ? `${label} : ${used}/${limit} sur votre plan`
    : `${label}: ${used}/${limit} on your plan`;
}

export function isPlanCapKey(key: string): key is PlanCapKey {
  return key in NOUN;
}

export function planCapMessage(fr: boolean, key: PlanCapKey, limit: number, used: number): string {
  const [one, many] = NOUN[key][fr ? 'fr' : 'en'];
  const noun = limit === 1 ? one : many;
  if (key === 'scans_per_month') {
    return fr
      ? `Votre plan inclut ${limit} ${noun}, tous utilisés ce mois-ci (${used}/${limit}). Le compteur repart le 1er du mois ; passez à un plan supérieur pour scanner sans limite.`
      : `Your plan includes ${limit} ${noun}, all used this month (${used}/${limit}). The count resets on the 1st; upgrade to scan without a cap.`;
  }
  const way = WAY_OUT[key];
  if (fr) {
    return `Votre plan inclut ${limit} ${noun} (${used}/${limit}). ${way ? `${way.fr} ou passez` : 'Passez'} à un plan supérieur pour en ajouter.`;
  }
  return `Your plan includes ${limit} ${noun} (${used}/${limit}). ${way ? `${way.en} or upgrade` : 'Upgrade'} to add more.`;
}
