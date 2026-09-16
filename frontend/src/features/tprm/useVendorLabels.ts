// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Words for the vendor screens' backend values (#673). A value the client does
// not know is shown as the server sent it, never dropped.

import { useI18n } from '../../hooks/useI18n';
import { knownServiceCriticality, knownStatus, knownTier, verbKey } from './vendorDisplay';

export function useVendorLabels() {
  const { t } = useI18n();
  return {
    tier: (v: string | null | undefined): string => {
      const k = knownTier(v);
      return k ? t(`vendors.tiers.${k}`) : (v ?? '');
    },
    status: (v: string | null | undefined): string => {
      const k = knownStatus(v);
      return k ? t(`vendors.status.${k}`) : (v ?? '');
    },
    serviceCriticality: (v: string | null | undefined): string => {
      const k = knownServiceCriticality(v);
      return k ? t(`vendors.serviceCriticality.${k}`) : (v ?? '');
    },
    verb: (v: string | null | undefined): string => {
      const k = verbKey(v);
      return k === 'other' ? (v ?? '') : t(`vendors.verbs.${k}`);
    },
  };
}
