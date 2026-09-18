// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Where a notification takes you when it is clicked in the bell panel.
//
// The backend already records WHAT a notification is about — `resource_type`
// and `resource_id` on every row (domain.Notification). This module turns that
// pair into a route, using the same destinations the Action Center uses
// (backend/internal/application/actioncenter/deeplinks.go), and proves the path
// resolves before anyone navigates to it (`safeDeepLink`).
//
//   risk               /risks?drawer=risk&entity={id}
//   mitigation         /risks/mitigations/{id}
//   incident           /incidents/{id}/war-room      (no id → /incidents)
//   evidence           /compliance/evidence?drawer=evidence&entity={id}
//   remediation(_plan) /compliance/remediation/{id}
//   approval_request   /governance
//   vendor_assessment  /vendors/{vendor}/assessments/{id}  (vendor looked up)
//   organization       /settings
//
// A notification whose target is unknown resolves to null: the row still marks
// itself read on click, it just does not pretend to go somewhere.

import { safeDeepLink } from '../action-center/actionLinks';
import { vendorService } from '../tprm/vendorService';
import type { Notification } from './notificationService';

type Target = Pick<Notification, 'resource_type' | 'resource_id'>;

/** The route for a notification that needs no lookup, or null. */
export function notificationHref(n: Target): string | null {
  const id = n.resource_id ? encodeURIComponent(n.resource_id) : '';
  switch (n.resource_type) {
    case 'risk':
      return id ? safeDeepLink(`/risks?drawer=risk&entity=${id}`) : safeDeepLink('/risks');
    case 'mitigation':
      return id ? safeDeepLink(`/risks/mitigations/${id}`) : safeDeepLink('/risks/mitigations');
    case 'incident':
      return id ? safeDeepLink(`/incidents/${id}/war-room`) : safeDeepLink('/incidents');
    case 'evidence':
      return id
        ? safeDeepLink(`/compliance/evidence?drawer=evidence&entity=${id}`)
        : safeDeepLink('/compliance/evidence');
    case 'remediation':
    case 'remediation_plan':
      return id
        ? safeDeepLink(`/compliance/remediation/${id}`)
        : safeDeepLink('/compliance/remediation');
    case 'approval_request':
      return safeDeepLink('/governance');
    case 'organization':
      return safeDeepLink('/settings');
    default:
      return null;
  }
}

/** True when clicking the notification can lead somewhere. */
export function hasNotificationTarget(n: Target): boolean {
  if (n.resource_type === 'vendor_assessment') return Boolean(n.resource_id);
  return notificationHref(n) !== null;
}

/**
 * The route for any notification, including the ones whose URL needs a parent
 * id the notification does not carry. A vendor assessment's page is nested
 * under its vendor, so the assessment is fetched (tenant-scoped server-side) to
 * learn which vendor it belongs to. A failed lookup falls back to the register.
 */
export async function resolveNotificationHref(n: Target): Promise<string | null> {
  if (n.resource_type === 'vendor_assessment' && n.resource_id) {
    try {
      const a = await vendorService.getAssessment(n.resource_id);
      return (
        safeDeepLink(
          `/vendors/${encodeURIComponent(a.vendor_asset_id)}/assessments/${encodeURIComponent(n.resource_id)}`,
        ) ?? safeDeepLink('/vendors')
      );
    } catch {
      return safeDeepLink('/vendors');
    }
  }
  return notificationHref(n);
}
