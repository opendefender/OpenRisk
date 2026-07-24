// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Onboarding progress (UX-01/UX-07/UX-17/UX-32). The account is created first
// (email + password only); the guided onboarding runs AFTER, on the dashboard,
// and drives the user to the "Aha moment" (first risk) then to workspace
// personalization. Steps that can be derived from real data (a risk exists, a
// framework exists) are computed live; the rest are persisted here.

import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface OnboardingState {
  /** The user invited a teammate (set from the invite modal success). */
  invited: boolean;
  /** The user personalized their workspace (theme/accent) after the Aha. */
  personalized: boolean;
  /** The checklist was dismissed / completed and should stay collapsed. */
  dismissed: boolean;
  markInvited: () => void;
  markPersonalized: () => void;
  dismiss: () => void;
}

export const useOnboarding = create<OnboardingState>()(
  persist(
    (set) => ({
      invited: false,
      personalized: false,
      dismissed: false,
      markInvited: () => set({ invited: true }),
      markPersonalized: () => set({ personalized: true }),
      dismiss: () => set({ dismissed: true }),
    }),
    { name: 'openrisk-onboarding' }
  )
);
