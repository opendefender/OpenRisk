// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The universal entity drawer's public surface (W1-02).
//
// A page never imports the drawer component. It mounts once in the shell and is
// driven entirely by the URL, so a page that wants to open an entity calls
// `useDrawerController().open(...)` — or simply links to a `drawerHref`.

export { EntityDrawerHost } from './EntityDrawerHost';
export {
  useDrawerController,
  readDrawer,
  writeDrawer,
  stripDrawer,
  drawerHref,
  DRAWER_PARAM,
  ENTITY_PARAM,
  TAB_PARAM,
  DRAWER_PARAMS,
  type DrawerState,
  type DrawerController,
} from './drawerState';
export { useTenantTimeline, useEntityCatalogue, ENTITY_QUERY_ROOT } from './useEntityDrawer';
export { ENTITY_TYPES, isEntityType } from './types';
export type {
  EntityType,
  EntitySection,
  EntitySummary,
  EntityView,
  EntityAction,
  EntityRelation,
  RelationGroup,
  TimelineEvent,
  TimelinePage,
  AuditRecord,
  AuditPage,
  EntityCatalogueEntry,
  Chip,
  Tone,
} from './types';
