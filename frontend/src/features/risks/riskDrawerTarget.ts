// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Which part of the risk drawer a deep link names (#913).
//
// `/risks?focus=<id>&tab=<name>` predates the redesign: its names were the old
// drawer's nine tabs. The new drawer has four (Aperçu, Mitigations, Historique,
// Plus) and the rest moved behind "Plus", so every old name still lands on the
// content it always named, and an unknown name lands on the overview rather
// than on nothing.

export type DrawerTab = 'overview' | 'miti' | 'history' | 'more';

export const MORE_SECTIONS = [
  'ownership',
  'lifecycle',
  'score',
  'smart',
  'financial',
  'cti',
  'ai',
] as const;
export type MoreSection = (typeof MORE_SECTIONS)[number];

export interface DrawerTarget {
  tab: DrawerTab;
  section: MoreSection;
}

export const DEFAULT_SECTION: MoreSection = 'ownership';

export function resolveDrawerTarget(raw: string | null | undefined): DrawerTarget {
  const name = (raw ?? '').trim().toLowerCase();
  switch (name) {
    case 'miti':
    case 'mitigations':
      return { tab: 'miti', section: DEFAULT_SECTION };
    case 'timeline':
    case 'history':
      return { tab: 'history', section: DEFAULT_SECTION };
    case 'details':
    case 'ownership':
      // The old "details" tab held the responsibilities and control mappings.
      return name === 'ownership'
        ? { tab: 'more', section: 'ownership' }
        : { tab: 'overview', section: DEFAULT_SECTION };
    case 'lifecycle':
    case 'score':
    case 'smart':
    case 'financial':
    case 'cti':
    case 'ai':
      return { tab: 'more', section: name };
    case 'more':
      return { tab: 'more', section: DEFAULT_SECTION };
    case 'overview':
    default:
      return { tab: 'overview', section: DEFAULT_SECTION };
  }
}
