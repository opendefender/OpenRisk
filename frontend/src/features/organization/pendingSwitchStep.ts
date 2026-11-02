// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The second-factor step a switch stopped at (GHSA-45mr-j2w6-m4jv), handed from
// the switcher to the screen that collects the code. Held in memory only: the
// token is never put in the URL, in history state or in storage, and a reload
// drops it, which simply leaves the user in the organization they were in.

export interface PendingSwitchStep {
  token: string;
  enrolling: boolean;
}

let pending: PendingSwitchStep | null = null;

export function holdSwitchStep(step: PendingSwitchStep): void {
  pending = step;
}

export function peekSwitchStep(): PendingSwitchStep | null {
  return pending;
}

export function clearSwitchStep(): void {
  pending = null;
}
