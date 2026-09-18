// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// A notification in the bell panel takes the user to the exact thing it is
// about. Every destination asserted here resolves in routeModel.

import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../tprm/vendorService', () => ({
  vendorService: { getAssessment: vi.fn() },
}));

import { vendorService } from '../../tprm/vendorService';
import type { VendorAssessment } from '../../tprm/vendorService';
import {
  hasNotificationTarget,
  notificationHref,
  resolveNotificationHref,
} from '../notificationLinks';

const getAssessment = vi.mocked(vendorService.getAssessment);
const ID = '7b0c6f1e-2d7a-4c1b-9a51-3a3f2f0c9e11';
const VENDOR = 'c2f7e0d4-1b2a-4e8f-8d3c-5a6b7c8d9e0f';

describe('notificationHref', () => {
  beforeEach(() => {
    getAssessment.mockReset();
  });

  it.each([
    ['risk', `/risks?drawer=risk&entity=${ID}`],
    ['mitigation', `/risks/mitigations/${ID}`],
    ['incident', `/incidents/${ID}/war-room`],
    ['evidence', `/compliance/evidence?drawer=evidence&entity=${ID}`],
    ['remediation_plan', `/compliance/remediation/${ID}`],
    ['approval_request', '/governance'],
    ['organization', '/settings'],
    ['scan', `/infrastructure/scans/${ID}`],
  ])('routes %s to its exact page', (resource_type, href) => {
    expect(notificationHref({ resource_type, resource_id: ID })).toBe(href);
  });

  it('falls back to the list when the id is missing', () => {
    expect(notificationHref({ resource_type: 'incident' })).toBe('/incidents');
    expect(notificationHref({ resource_type: 'scan' })).toBe('/infrastructure');
  });

  it('refuses to link a notification about nothing it knows', () => {
    expect(notificationHref({ resource_type: 'action', resource_id: ID })).toBeNull();
    expect(notificationHref({})).toBeNull();
    expect(hasNotificationTarget({})).toBe(false);
  });

  it('never lets an id escape the path', () => {
    const href = notificationHref({ resource_type: 'mitigation', resource_id: '../../x?y' });
    expect(href).toBe('/risks/mitigations/..%2F..%2Fx%3Fy');
  });

  it('nests a vendor assessment under the vendor it belongs to', async () => {
    getAssessment.mockResolvedValue({ vendor_asset_id: VENDOR } as VendorAssessment);
    expect(hasNotificationTarget({ resource_type: 'vendor_assessment', resource_id: ID })).toBe(
      true,
    );
    await expect(
      resolveNotificationHref({ resource_type: 'vendor_assessment', resource_id: ID }),
    ).resolves.toBe(`/vendors/${VENDOR}/assessments/${ID}`);
  });

  it('falls back to the vendor register when the assessment is gone', async () => {
    getAssessment.mockRejectedValue(new Error('404'));
    await expect(
      resolveNotificationHref({ resource_type: 'vendor_assessment', resource_id: ID }),
    ).resolves.toBe('/vendors');
  });
});
