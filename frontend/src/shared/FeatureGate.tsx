// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// FeatureGate is the paywall UX primitive. Given a feature key it reads the real
// entitlements snapshot and, when the plan does not include the feature, renders
// the paid UI BLURRED behind an explaining upsell (reusing UpsellLock) — never
// hides it. A wall that explains converts; a wall that hides frustrates (task §2).
//
// This is presentation only. Authorization is enforced by the backend (402), so a
// gate the user bypasses in the DOM still cannot reach the paid endpoint.

import type { ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { UpsellLock } from './UpsellLock';
import { useFeature } from '../features/billing/useEntitlements';

const BENEFITS: Record<string, { title: string; description: string }> = {
  financial_quantification: {
    title: 'Quantification financière Monte-Carlo',
    description:
      "Chiffrez votre exposition en XAF et justifiez vos budgets sécurité auprès de la direction. Disponible à partir du plan Pro.",
  },
  ai_advisor: {
    title: 'Assistant IA GRC',
    description:
      "Suggestions de plans de traitement, détection des risques émergents et rédaction de rapports d'audit. Disponible à partir du plan Pro.",
  },
  smart_score: {
    title: 'Score de risque intelligent',
    description:
      'Un score multifactoriel (exposition, vulnérabilités, menaces, valeur financière) au lieu du seul P×I. Disponible à partir du plan Pro.',
  },
  executive_dashboard: {
    title: 'Tableau de bord exécutif',
    description:
      'Toute votre posture (cyber score, exposition financière, KRI) consolidée en un écran pour le COMEX. Disponible à partir du plan Pro.',
  },
  scanner: {
    title: "Scanner d'infrastructure",
    description: 'Découverte automatique de vos actifs et vulnérabilités. Disponible à partir du plan Pro.',
  },
  automation: {
    title: 'Automatisation (SOAR)',
    description: 'Enchaînez alertes, tickets et escalades SLA sans intervention manuelle. Disponible à partir du plan Pro.',
  },
  cti: {
    title: 'Renseignement sur les menaces (CTI)',
    description: 'Enrichissez vos vulnérabilités avec NVD, CISA-KEV et MITRE ATT&CK. Disponible à partir du plan Business.',
  },
  governance: {
    title: 'Gouvernance & approbations',
    description: 'Workflows Maker-Checker et piste d’audit infalsifiable. Disponible à partir du plan Business.',
  },
  sso: {
    title: 'SSO / SAML',
    description: 'Authentification centralisée via votre annuaire d’entreprise. Disponible à partir du plan Business.',
  },
};

const PLAN_LABEL: Record<string, string> = { free: 'Free', pro: 'Pro', business: 'Business', enterprise: 'Enterprise' };

interface FeatureGateProps {
  feature: string;
  children: ReactNode;
  /** Optional override copy. */
  title?: string;
  description?: string;
  moment?: string;
}

export function FeatureGate({ feature, children, title, description, moment }: FeatureGateProps) {
  const navigate = useNavigate();
  const { enabled, requiredPlan, loading } = useFeature(feature);

  // While loading, render the children optimistically (no flash of the wall).
  if (loading || enabled) return <>{children}</>;

  const meta = BENEFITS[feature];
  return (
    <UpsellLock
      title={title ?? meta?.title ?? 'Fonctionnalité premium'}
      description={description ?? meta?.description ?? `Disponible à partir du plan ${PLAN_LABEL[requiredPlan] ?? 'Pro'}.`}
      ctaLabel={`Passer au plan ${PLAN_LABEL[requiredPlan] ?? 'Pro'}`}
      moment={moment ?? `Plan ${PLAN_LABEL[requiredPlan] ?? 'Pro'}`}
      onUpgrade={() => navigate('/settings?tab=billing')}
    >
      {children}
    </UpsellLock>
  );
}
