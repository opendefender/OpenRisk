// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

import type { ResolvedPlanKey } from '../../services/entitlementService';

// Plan names are product names and read the same in both languages; only the
// word around them is translated (common.planName).
const PLAN_NAME: Record<ResolvedPlanKey, string> = {
  free: 'Free',
  pro: 'Pro',
  business: 'Business',
  enterprise: 'Enterprise',
  self_hosted: 'Self-hosted',
};

/** The display name of a resolved plan, or undefined for a key this build does
 *  not know. Shared by the sidebar and the organisation profile, which must show
 *  the plan the tenant actually gets, not the raw column (#612). */
export function planDisplayName(plan: ResolvedPlanKey): string | undefined {
  return PLAN_NAME[plan];
}
