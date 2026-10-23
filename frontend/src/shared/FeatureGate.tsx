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
//
// The plan named in the copy is the one the snapshot reports for the feature, so
// the wall cannot promise a plan the matrix does not grant it on.

import type { ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { UpsellLock } from './UpsellLock';
import { useI18n } from '../hooks/useI18n';
import { useFeature, useFeatureAtLeast } from '../features/billing/useEntitlements';
import type { PlanKey } from '../services/entitlementService';

/** The features with benefit copy under `paywall.features` in the locales. */
const DESCRIBED = new Set([
  'financial_quantification',
  'ai_advisor',
  'smart_score',
  'executive_dashboard',
  'cti',
  'vendor_risk',
]);

const PLAN_LABEL: Record<string, string> = {
  free: 'Free',
  pro: 'Pro',
  business: 'Business',
  enterprise: 'Enterprise',
};

interface FeatureGateProps {
  feature: string;
  children: ReactNode;
  /** Optional override copy. */
  title?: string;
  description?: string;
  moment?: string;
  /** Gate on a depth rather than on the feature being on at all (e.g. the
   *  Monte-Carlo views need "on" while Free has "basic"), with the plan that
   *  reaches it. */
  minLevel?: string;
  minPlan?: PlanKey;
}

export function FeatureGate({
  feature,
  children,
  title,
  description,
  moment,
  minLevel,
  minPlan,
}: FeatureGateProps) {
  const navigate = useNavigate();
  const { t } = useI18n();
  const any = useFeature(feature);
  const deep = useFeatureAtLeast(feature, minLevel ?? 'limited', minPlan ?? any.requiredPlan);
  const { enabled, requiredPlan, loading } = minLevel ? deep : any;

  // While loading, render the children optimistically (no flash of the wall).
  if (loading || enabled) return <>{children}</>;

  const plan = PLAN_LABEL[requiredPlan] ?? PLAN_LABEL.pro;
  const described = DESCRIBED.has(feature);
  const availableFrom = t('paywall.availableFrom', { plan });
  return (
    <UpsellLock
      title={
        title ?? (described ? t(`paywall.features.${feature}.title`) : t('paywall.fallbackTitle'))
      }
      description={
        description ??
        (described
          ? `${t(`paywall.features.${feature}.description`)} ${availableFrom}`
          : availableFrom)
      }
      ctaLabel={t('paywall.cta', { plan })}
      moment={moment ?? t('paywall.moment', { plan })}
      onUpgrade={() => navigate('/settings?tab=billing')}
    >
      {children}
    </UpsellLock>
  );
}
