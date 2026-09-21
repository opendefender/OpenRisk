// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The plan line under the organisation name in the sidebar (#656).
//
// It used to be the fixed string "Enterprise", on every tenant and every plan:
// a Free organisation was told it was on Enterprise, and a Pro customer found
// Enterprise features paywalled after the UI said otherwise. The line now says
// only what GET /entitlements says — the same snapshot the paywall reads — and
// says nothing rather than guess: a skeleton while it loads, no line at all if
// it fails.

import { useEntitlements } from '../../features/billing/useEntitlements';
import type { PlanKey } from '../../services/entitlementService';
import { useI18n } from '../../hooks/useI18n';

// Plan names are product names and read the same in both languages; only the
// word around them is translated (common.planName).
const PLAN_NAME: Record<PlanKey, string> = {
  free: 'Free',
  pro: 'Pro',
  business: 'Business',
  enterprise: 'Enterprise',
};

export function OrgPlanLabel() {
  const { t } = useI18n();
  const { data, isLoading } = useEntitlements();

  if (isLoading) {
    return (
      <div
        className="h-[9px] w-12 mt-[3px] rounded or-skeleton"
        aria-hidden="true"
        data-testid="org-plan-loading"
      />
    );
  }

  const name = data ? PLAN_NAME[data.plan] : undefined;
  // Failed to load, or a plan key this build does not know: no line. A
  // defaulted plan is exactly the defect this component replaces.
  if (!name) return null;

  return (
    <div className="text-[10.5px] text-ink-soft" data-testid="org-plan">
      {t('common.planName', { plan: name })}
    </div>
  );
}
