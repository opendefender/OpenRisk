// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// A self-hosted instance bills nothing (#612, D-040), so it has no billing tab:
// no plan grid, no trial, no checkout. A link to ?tab=billing lands on General.
export function settingsTabVisible(tab: string, selfHosted: boolean): boolean {
  return !(selfHosted && tab === 'billing');
}

export function settingsTabShown<T extends string>(tab: T, selfHosted: boolean, fallback: T): T {
  return settingsTabVisible(tab, selfHosted) ? tab : fallback;
}
