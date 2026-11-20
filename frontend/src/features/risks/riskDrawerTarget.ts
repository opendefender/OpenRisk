// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Which tab of the risk drawer a deep link names (#913).
//
// `/risks?focus=<id>&tab=<name>` predates the redesign. The drawer now shows the
// design's three tabs first and every older tab after them, so each old name
// still lands on the tab it always named. The old "details" tab held the
// overview content, which is now Aperçu; its responsibilities and control
// mappings are the "ownership" tab. An unknown name lands on the overview.

export const DRAWER_TABS = [
  'overview',
  'miti',
  'history',
  'ownership',
  'lifecycle',
  'score',
  'smart',
  'financial',
  'cti',
  'ai',
] as const;
export type DrawerTab = (typeof DRAWER_TABS)[number];

export function resolveDrawerTab(raw: string | null | undefined): DrawerTab {
  const name = (raw ?? '').trim().toLowerCase();
  switch (name) {
    case 'mitigations':
    case 'miti':
      return 'miti';
    case 'timeline':
    case 'history':
      return 'history';
    case 'ownership':
    case 'lifecycle':
    case 'score':
    case 'smart':
    case 'financial':
    case 'cti':
    case 'ai':
      return name;
    case 'details':
    case 'overview':
    default:
      return 'overview';
  }
}
