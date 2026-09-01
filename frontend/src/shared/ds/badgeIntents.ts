// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: Apache-2.0

/**
 * Business enum -> visual intent.
 *
 * Deliberately NOT inside Badge.tsx. Two reasons: a file that exports both a
 * component and constants loses fast refresh, and — more importantly — the
 * badge must not know what a risk status is. It renders an intent; these tables
 * are where a domain says which intent its values deserve, and they can be read
 * and checked against the domain without reading any rendering code.
 *
 * Each table is exhaustive over its union, so adding a value to the domain is a
 * compile error here rather than a silent fall-through to neutral.
 */

import type { BadgeIntent } from './Badge';

export type RiskStatusValue = 'open' | 'in_progress' | 'mitigated' | 'accepted' | 'closed';

export const riskStatusIntent: Record<RiskStatusValue, BadgeIntent> = {
  open: 'danger',
  in_progress: 'warning',
  mitigated: 'success',
  accepted: 'neutral',
  closed: 'neutral',
};

export type SeverityValue = 'critical' | 'high' | 'medium' | 'low';

export const severityIntent: Record<SeverityValue, BadgeIntent> = {
  critical: 'danger',
  high: 'warning',
  /* Medium maps to info, not warning: if medium and high share an intent the
     badge stops distinguishing the two values it exists to distinguish. */
  medium: 'info',
  low: 'success',
};
