// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The signed-in person's effective date preferences (#719), as the server
// resolved them: their own choice, else their organization's default.
//
// Not persisted. The server is the record; this is only the copy the
// formatters read synchronously, refilled from GET /users/me on every session.

import { create } from 'zustand';

import { registerTenantStore } from '../../lib/sessionScope';

import type { DatePattern } from '../../i18n/format';

interface PreferenceState {
  timeZone?: string;
  pattern?: DatePattern;
  setDatePreferences: (p: { timeZone?: string; pattern?: DatePattern }) => void;
  clear: () => void;
}

export const usePreferenceStore = create<PreferenceState>((set) => ({
  timeZone: undefined,
  pattern: undefined,
  setDatePreferences: ({ timeZone, pattern }) => set({ timeZone, pattern }),
  clear: () => set({ timeZone: undefined, pattern: undefined }),
}));

// A new session must not inherit the previous person's zone and pattern.
registerTenantStore(usePreferenceStore, { timeZone: undefined, pattern: undefined });

export function toDatePattern(value: string | undefined): DatePattern | undefined {
  switch (value) {
    case 'DD/MM/YYYY':
    case 'MM/DD/YYYY':
    case 'YYYY-MM-DD':
      return value;
    default:
      return undefined;
  }
}
