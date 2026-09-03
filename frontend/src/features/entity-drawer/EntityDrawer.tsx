// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The Universal Entity Drawer (W1-02).
//
// ONE component for eight business objects. Before this there were eight
// separate drawers — risk, incident, control, evidence, asset history, CVE,
// vulnerability, mitigation — each with its own header, its own tab model, its
// own idea of what a loading state looks like, and all eight holding the
// selected entity in a local useState, so none of them could be shared,
// survived a refresh, or closed on Back.
//
// What makes one component sufficient is that the SERVER decides what this
// drawer contains: which sections a type has, which of those this caller may
// open, which relations exist, and which actions are permitted. There is no
// per-type branch in here. Adding a ninth type is a resolver on the server.
//
// Each section loads on its own (§27). A relation query that fails leaves the
// summary readable; a timeline that fails does not blank the record. The
// alternative — one request for everything — lets the slowest and least
// important part of the drawer decide whether any of it renders at all.

import { useEffect, useMemo, useRef } from 'react';
import {
  Drawer,
  Tabs,
  TabPanel,
  Button,
  ErrorState,
  PermissionDenied,
  SkeletonRows,
  type TabItem,
} from '../../shared/ds';
import { EmptyState } from '../../shared/EmptyState';
import { isEntityError } from './entityService';
import {
  useEntity,
  useEntityAudit,
  useEntityRelations,
  useEntityTimeline,
} from './useEntityDrawer';
import { AuditSection } from './sections/AuditSection';
import { RelationsSection } from './sections/RelationsSection';
import { SummarySection } from './sections/SummarySection';
import { TimelineSection } from './sections/TimelineSection';
import { useUIStrings } from '../../shared/uiStrings';
import type { EntityAction, EntitySection, EntityType } from './types';

const SECTION_LABEL_KEY: Record<
  EntitySection,
  'ed_sec_summary' | 'ed_sec_relations' | 'ed_sec_timeline' | 'ed_sec_audit'
> = {
  summary: 'ed_sec_summary',
  relations: 'ed_sec_relations',
  timeline: 'ed_sec_timeline',
  audit: 'ed_sec_audit',
};

export interface EntityDrawerProps {
  type: EntityType;
  id: string;
  /** The open section. Comes from the URL, so a link can name a tab. */
  tab: EntitySection;
  onTabChange: (tab: EntitySection) => void;
  onClose: () => void;
  /** Open a related entity in this same drawer, pushing history. */
  onOpenEntity: (type: EntityType, id: string) => void;
}

export function EntityDrawer({
  type,
  id,
  tab,
  onTabChange,
  onClose,
  onOpenEntity,
}: EntityDrawerProps) {
  const L = useUIStrings();
  const entity = useEntity(type, id);
  const sections = useMemo(() => entity.data?.sections ?? [], [entity.data]);

  // A tab named in the URL that this entity does not offer — an old link, or a
  // caller who has since lost the audit permission — falls back to the first
  // section the server DID offer, rather than rendering an empty panel.
  const activeTab: EntitySection = sections.includes(tab) ? tab : (sections[0] ?? 'summary');

  // Each section fetches only while it is the open one. Opening a drawer on
  // Overview never pays for a relation query nobody asked for.
  const relations = useEntityRelations(type, id, activeTab === 'relations');
  const timeline = useEntityTimeline(type, id, activeTab === 'timeline');
  const audit = useEntityAudit(type, id, activeTab === 'audit');

  const items: TabItem<EntitySection>[] = useMemo(
    () => sections.map((s) => ({ id: s, label: L[SECTION_LABEL_KEY[s]] })),
    [sections, L],
  );

  const timelineEvents = useMemo(
    () => timeline.data?.pages.flatMap((p) => p.events) ?? [],
    [timeline.data],
  );

  // Screen readers are told which record the drawer now shows. Without this a
  // relation click is silent: the visible content changes completely, and a
  // non-sighted user gets no signal that anything happened.
  const liveRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (entity.data && liveRef.current) {
      liveRef.current.textContent = `${entity.data.summary.type_label}: ${entity.data.summary.title}`;
    }
  }, [entity.data]);

  const title = entity.data?.summary.title ?? (entity.isLoading ? L.ed_loading : L.ed_record);
  const subtitle = entity.data?.summary.subtitle || entity.data?.summary.type_label;
  const tabsId = `entity-${type}-${id}`;

  return (
    <Drawer
      open
      onClose={onClose}
      title={title}
      subtitle={subtitle}
      size="lg"
      actions={entity.data ? <DrawerActions actions={entity.data.actions} /> : undefined}
    >
      <div ref={liveRef} className="sr-only" role="status" aria-live="polite" />

      {entity.isLoading && <SkeletonRows rows={6} />}

      {entity.isError && (
        <EntityLoadError error={entity.error} type={type} onRetry={() => void entity.refetch()} />
      )}

      {entity.data && (
        <div className="flex flex-col gap-4">
          {items.length > 1 && (
            <Tabs
              id={tabsId}
              items={items}
              value={activeTab}
              onChange={onTabChange}
              label={`${entity.data.summary.type_label} sections`}
            />
          )}

          <TabPanel tabsId={tabsId} id="summary" active={activeTab === 'summary'}>
            <SummarySection summary={entity.data.summary} />
          </TabPanel>

          <TabPanel tabsId={tabsId} id="relations" active={activeTab === 'relations'}>
            <RelationsSection
              groups={relations.data}
              isLoading={relations.isLoading}
              error={relations.error}
              onRetry={() => void relations.refetch()}
              onOpen={onOpenEntity}
            />
          </TabPanel>

          <TabPanel tabsId={tabsId} id="timeline" active={activeTab === 'timeline'}>
            <TimelineSection
              events={timelineEvents}
              isLoading={timeline.isLoading}
              error={timeline.error}
              hasMore={!!timeline.hasNextPage}
              isLoadingMore={timeline.isFetchingNextPage}
              onLoadMore={() => void timeline.fetchNextPage()}
              onRetry={() => void timeline.refetch()}
            />
          </TabPanel>

          <TabPanel tabsId={tabsId} id="audit" active={activeTab === 'audit'}>
            <AuditSection
              records={audit.data?.events}
              total={audit.data?.total ?? 0}
              isLoading={audit.isLoading}
              error={audit.error}
              onRetry={() => void audit.refetch()}
            />
          </TabPanel>
        </div>
      )}
    </Drawer>
  );
}

/**
 * The failure states for the record itself.
 *
 * 403 and 404 are different facts with different remedies and are never
 * collapsed into "something went wrong". Neither offers a retry: both are final
 * answers, and a Retry button on a permission wall is a control that cannot work.
 *
 * Note what the 404 copy does NOT say. It does not say the record exists
 * elsewhere or belongs to another organisation — the server answers a foreign id
 * and a fabricated one identically on purpose, and copy that distinguished them
 * would undo that (§31).
 */
function EntityLoadError({
  error,
  type,
  onRetry,
}: {
  error: unknown;
  type: EntityType;
  onRetry: () => void;
}) {
  const L = useUIStrings();
  if (isEntityError(error)) {
    if (error.status === 403) {
      return <PermissionDenied resource={`this ${type}`} />;
    }
    if (error.status === 404) {
      return (
        <EmptyState variant="no-results" title={L.ed_notFound} description={L.ed_notFoundDesc} />
      );
    }
    if (error.status === 400) {
      return (
        <EmptyState variant="no-results" title={L.ed_badLink} description={L.ed_badLinkDesc} />
      );
    }
  }
  return <ErrorState title={L.ed_loadFailed} description={L.ed_retryDesc} onRetry={onRetry} />;
}

/**
 * The header's primary action.
 *
 * The server returns ONLY the actions this caller may perform, so there is
 * nothing to disable and nothing to grey out. The action navigates to the screen
 * that performs it rather than mutating inline: the drawer is an inspection
 * surface, and a mutation launched from a panel that can be closed mid-flight is
 * how half-applied changes happen.
 */
function DrawerActions({ actions }: { actions: EntityAction[] }) {
  const primary = actions.find((a) => a.kind === 'primary');
  if (!primary) return null;
  return (
    <Button
      variant="secondary"
      size="sm"
      onClick={() =>
        window.dispatchEvent(new CustomEvent('openrisk:entity-action', { detail: primary }))
      }
    >
      {primary.label}
    </Button>
  );
}
