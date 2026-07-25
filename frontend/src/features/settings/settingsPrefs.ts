// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Client-side settings preferences, persisted so a change survives a reload
// (UX-23 autosave, OR-BUG-004). These are per-user UX/notification preferences
// held in localStorage — not org-wide policy enforcement (which is admin-config,
// server-side, and surfaced honestly in the Settings screen).

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type PrefKey =
  | 'strict_compliance'
  | 'auto_recalc'
  | 'notif_email'
  | 'notif_slack'
  | 'notif_sms'
  | 'alert_critical_risk'
  | 'alert_score_up'
  | 'alert_warroom'
  | 'alert_mitigation'
  | 'alert_digest';

const DEFAULTS: Record<PrefKey, boolean> = {
  strict_compliance: true,
  auto_recalc: true,
  notif_email: true,
  notif_slack: false,
  notif_sms: false,
  alert_critical_risk: true,
  alert_score_up: true,
  alert_warroom: true,
  alert_mitigation: true,
  alert_digest: false,
};

interface PrefsState {
  prefs: Record<PrefKey, boolean>;
  setPref: (key: PrefKey, value: boolean) => void;
}

export const useSettingsPrefs = create<PrefsState>()(
  persist(
    (set, get) => ({
      prefs: { ...DEFAULTS },
      setPref: (key, value) => set({ prefs: { ...get().prefs, [key]: value } }),
    }),
    {
      name: 'openrisk-settings-prefs',
      // Keep newly-added default keys present when older state is rehydrated.
      merge: (persisted, current) => {
        const p = (persisted as Partial<PrefsState>)?.prefs ?? {};
        return { ...current, prefs: { ...DEFAULTS, ...p } };
      },
    }
  )
);
