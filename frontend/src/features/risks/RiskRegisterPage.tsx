// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Risk register on the October 2026 redesign (#913).
//
// Header with the active count and the annualised exposure (GET /risks/summary),
// view tabs with counts, a toolbar (search, band pills, status and owner
// listboxes, Tableau / Matrice), the table or the matrix, a footer that states
// our score formula and bands, a bulk bar on selection and the risk drawer.
//
// The table is the shared <DataTable>: server-side sort and pagination, URL
// state, keyboard rows, column chooser, saved filters. Where the design leaves
// something out and the product keeps it, it stays reachable: phase, control
// reference, tags and last update are columns of the chooser; lifecycle phase,
// category and source and "unmapped" are in the Filters panel; the three
// responsibilities, control mappings, lifecycle, score working, SmartScore,
// financial, CTI and AI sit behind the drawer's "Plus" tab.
//
// URL: `f.scope` is the tab, `f.criticality` / `f.status` / `f.owner_id` the
// filters, `view=matrix` the layout (kept for the dashboard's matrix link).

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  Download,
  Eye,
  Pencil,
  Plus,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  Upload,
} from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useRiskStore } from '../../hooks/useRiskStore';
import { formatNumber } from '../../i18n/format';
import { Button, ErrorState, SkeletonRows, TabPanel, Tabs, type TabItem } from '../../shared/ds';
import { EmptyState, PageFrame, PageHeader } from '../../shared/ui';
import {
  DataTable,
  useTableState,
  type BulkAction,
  type Facet,
  type RowAction,
} from '../../shared/datatable';
import { ImpactDialog } from '../../shared/ImpactDialog';
import { useFocusParam } from '../../shared/useFocusParam';
import { isMissingRows } from '../../services/bulkService';
import { riskService, type RiskStatus as ApiRiskStatus } from '../../services/riskService';
import { EditRiskModal } from './components/EditRiskModal';
import { CreateMitigationModal } from '../mitigations/CreateMitigationModal';
import { millions } from '../financial/money';
import { DrawerStatus, RiskDrawer } from './RiskDrawer';
import { RiskMatrixPanel } from './RiskMatrixPanel';
import { LayoutSwitch, RegisterFilters, SearchBox } from './RegisterToolbar';
import { memberName } from './riskMembers';
import { exportRisksCsv, type RiskCsvContext } from './riskExport';
import { resolveDrawerTarget } from './riskDrawerTarget';
import { mapRisk, type UiRisk } from './riskMap';
import { riskRegisterService } from './riskRegisterService';
import {
  MATRIX_LIMIT,
  RISK_VIEWS,
  SCOPE_FACET,
  buildRiskParams,
  buildSummaryParams,
  parseLayout,
  readFilters,
  type RegisterLayout,
  type RiskStatusValue,
  type RiskView,
} from './riskViews';
import { useRiskColumns } from './useRiskColumns';
import {
  RISK_SUMMARY_KEY,
  useRegisterMembers,
  useRegisterMoney,
  useRiskSummary,
} from './useRiskRegisterData';
import { useRiskCategories } from './useTaxonomy';
import type { Criticality } from '../../shared/riskColors';

// The legacy tabs are reached from the drawer's "Plus"; the CTI one is exported
// for its own test.
export { DrawerCTI } from './RiskDrawerSections';

/** Statuses a bulk or single edit may write; "closed" goes through the lifecycle. */
const PICKABLE: readonly RiskStatusValue[] = ['open', 'in_progress', 'mitigated', 'accepted'];

type Params = Record<string, string | number>;

export function RiskRegisterPage() {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();

  const risks = useRiskStore((s) => s.risks);
  const total = useRiskStore((s) => s.total);
  const isLoading = useRiskStore((s) => s.isLoading);
  const loadError = useRiskStore((s) => s.error);
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const deleteRisk = useRiskStore((s) => s.deleteRisk);
  const bulkDelete = useRiskStore((s) => s.bulkDelete);
  const patchRisks = useRiskStore((s) => s.patchRisks);

  // A read-only member is not offered creation: the server would refuse it
  // (risks:create), after a form had been filled (#739).
  const canCreate = useAuthStore((s) => s.hasPermission('risks:create'));
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const canDelete = useAuthStore((s) => s.hasPermission('risks:delete'));

  const { data: categories } = useRiskCategories();
  const { members, isLoading: membersLoading } = useRegisterMembers();
  const money = useRegisterMoney();
  // Review dates are judged against the moment the page was opened.
  const [now] = useState(() => new Date());

  const [drawerId, setDrawerId] = useState<string | null>(null);
  const [editRaw, setEditRaw] = useState<UiRisk['raw'] | null>(null);
  const [mitiRiskId, setMitiRiskId] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<UiRisk | null>(null);
  const [bulkToDelete, setBulkToDelete] = useState<string[] | null>(null);
  const [deleting, setDeleting] = useState(false);
  // Bumped after a bulk action so the table forgets a selection that no longer
  // means anything (the rows moved, changed or are gone).
  const [selectionEpoch, setSelectionEpoch] = useState(0);

  // ?view=matrix is the layout; it predates the tabs and the dashboard links to it.
  const layout: RegisterLayout = parseLayout(searchParams.get('view'));
  const setLayout = (next: RegisterLayout) =>
    setSearchParams(
      (prev) => {
        const out = new URLSearchParams(prev);
        if (next === 'matrix') out.set('view', 'matrix');
        else out.delete('view');
        out.delete('page');
        return out;
      },
      { replace: true },
    );

  const table = useTableState({ defaultSort: { key: 'score', dir: 'desc' }, defaultPageSize: 50 });
  const { state } = table;
  const filters = readFilters(state.filters);

  // The query, recomputed only when what the server is asked actually changes —
  // not when an unrelated URL param (?focus) comes or goes.
  const paramsKey = JSON.stringify(buildRiskParams(state, layout));
  const params = useMemo(() => JSON.parse(paramsKey) as Params, [paramsKey]);
  const reload = useCallback(() => {
    void fetchRisks(params).catch(() => {});
    void qc.invalidateQueries({ queryKey: RISK_SUMMARY_KEY });
  }, [fetchRisks, params, qc]);
  // The list refetches whenever the query changes. An effect is right here: it
  // synchronises with the server, it does not derive state.
  // `hasLoaded` closes the frame between the first render (nothing loading yet,
  // nothing to show) and the effect: without it the "no risks yet" call to action
  // flashes on every visit before the skeleton takes over.
  const [hasLoaded, setHasLoaded] = useState(false);
  useEffect(() => {
    void fetchRisks(params)
      .catch(() => {})
      .finally(() => setHasLoaded(true));
  }, [fetchRisks, params]);

  // A risk created from anywhere (header, sidebar, N shortcut) re-reads the list.
  useEffect(() => {
    window.addEventListener('openrisk:risk-created', reload);
    return () => window.removeEventListener('openrisk:risk-created', reload);
  }, [reload]);

  const summary = useRiskSummary(buildSummaryParams(state));

  const ui: UiRisk[] = useMemo(() => risks.map((r) => mapRisk(r, locale)), [risks, locale]);

  /* ------------------------------------------------------- drawer / deep link */
  // /risks?focus=<id>&tab=<name> (universal search, the timeline redirect, the
  // vulnerability page): the drawer is open while the param is there. A risk that
  // is not on the loaded page is fetched on its own.
  const { focusId, focusTab, clearFocus } = useFocusParam();
  const openId = drawerId ?? focusId;
  const inList = openId ? ui.find((r) => r.id === openId) : undefined;
  const detail = useQuery({
    queryKey: ['risks', 'detail', openId],
    queryFn: ({ signal }) => riskRegisterService.get(openId as string, signal),
    enabled: !!openId && !inList && !isLoading,
    retry: false,
  });
  const detailRisk = useMemo(
    () => (detail.data ? mapRisk(detail.data, locale) : undefined),
    [detail.data, locale],
  );
  const drawerRisk = inList ?? detailRisk;
  const target = useMemo(
    () => resolveDrawerTarget(drawerId ? null : focusTab),
    [drawerId, focusTab],
  );
  const closeDrawer = () => {
    setDrawerId(null);
    clearFocus();
  };
  const openDrawer = (id: string) => {
    if (focusId) clearFocus();
    setDrawerId(id);
  };

  /* ----------------------------------------------------------------- export */
  const csvCtx: RiskCsvContext = useMemo(
    () => ({ locale, money, label: (key, p) => t(key, p) }),
    [locale, money, t],
  );
  const exportRows = (rows: readonly UiRisk[], filename: string) => {
    if (rows.length === 0) {
      toast.info(t('riskRegister.export.empty'));
      return;
    }
    const n = exportRisksCsv(rows, csvCtx, filename);
    toast.success(t('riskRegister.export.done', { count: n }));
  };

  /* ---------------------------------------------------------------- deletion */
  // Important and irreversible (a risk carries mitigation plans and history), so
  // an impact readout, not a bare confirm.
  const confirmDeleteRisk = async () => {
    const r = toDelete;
    if (!r) return;
    setDeleting(true);
    try {
      await deleteRisk(r.id);
      toast.success(t('riskRegister.delete.done'));
      if (drawerId === r.id) setDrawerId(null);
      setToDelete(null);
      void qc.invalidateQueries({ queryKey: RISK_SUMMARY_KEY });
    } catch {
      toast.error(t('riskRegister.delete.failed'));
    } finally {
      setDeleting(false);
    }
  };

  const confirmBulkDelete = async () => {
    const ids = bulkToDelete;
    if (!ids) return;
    setDeleting(true);
    try {
      // ONE request: transactional and audited (#598) — every named risk goes or none does.
      await bulkDelete(ids);
      toast.success(t('riskRegister.bulk.deleted', { count: ids.length }));
      setBulkToDelete(null);
      setSelectionEpoch((n) => n + 1);
      reload();
    } catch (err) {
      toast.error(
        isMissingRows(err)
          ? t('riskRegister.bulk.deleteStale')
          : t('riskRegister.bulk.deleteFailed', { count: ids.length }),
      );
    } finally {
      setDeleting(false);
    }
  };

  /* ------------------------------------------------------------ filter state */
  const setScope = (view: RiskView) => table.setFilter(SCOPE_FACET, view === 'all' ? [] : [view]);
  const hasFilters =
    state.q.trim() !== '' ||
    Object.entries(state.filters).some(([k, v]) => k !== SCOPE_FACET && v.length > 0);
  // The design's "Réinitialiser" clears the search and the filters, not the tab.
  const resetFilters = () =>
    table.apply({
      q: '',
      filters: filters.view === 'all' ? {} : { [SCOPE_FACET]: [filters.view] },
    });

  const toolbarControls = (
    <RegisterFilters
      filters={filters}
      summary={summary.data}
      summaryLoading={summary.isLoading}
      members={members}
      membersLoading={membersLoading}
      onToggleBand={(band: Criticality) => table.toggleFilter('criticality', band)}
      onStatus={(s) => table.setFilter('status', s ? [s] : [])}
      onOwner={(id) => table.setFilter('owner_id', id ? [id] : [])}
      hasFilters={hasFilters}
      onReset={resetFilters}
    />
  );

  /* ------------------------------------------------------------------- tabs */
  const tabItems: TabItem<RiskView>[] = RISK_VIEWS.map((v) => ({
    id: v,
    label: t(`riskRegister.view.${v}`),
    count: summary.data?.views?.[v],
    testId: `risk-tab-${v}`,
  }));

  /* ------------------------------------------------------------------ facets */
  // Criticality and status are the toolbar's; these are what the design drops and
  // the product keeps.
  const facets: Facet<UiRisk>[] = useMemo(
    () => [
      {
        key: 'phase',
        label: t('riskRegister.facet.phase'),
        // The server keeps the derived `phase` in step with lifecycle_state.
        options: (
          ['identified', 'analyzed', 'evaluated', 'treated', 'monitored', 'closed'] as const
        ).map((p) => ({ value: p, label: t(`riskRegister.phase.${p}`) })),
      },
      {
        key: 'category_id',
        label: t('riskRegister.facet.category'),
        // The CONTROLLED vocabulary; free-text tags would list every typed string.
        options: (categories ?? []).map((c) => ({ value: c.id, label: c.name })),
      },
      {
        key: 'source',
        label: t('riskRegister.facet.source'),
        options: (['manual', 'cti_auto', 'scan_auto', 'import'] as const).map((s) => ({
          value: s,
          label: t(`riskRegister.source.${s}`),
        })),
      },
      {
        key: 'unmapped',
        label: t('riskRegister.facet.unmapped'),
        single: true,
        options: [{ value: 'true', label: t('riskRegister.facet.unmappedOption') }],
      },
    ],
    [t, categories],
  );

  const columns = useRiskColumns({ members, money, now });

  /* ------------------------------------------------------------- row actions */
  const rowActions: RowAction<UiRisk>[] = useMemo(
    () => [
      {
        key: 'view',
        label: t('riskRegister.row.view'),
        icon: Eye,
        onSelect: (r) => openDrawer(r.id),
      },
      {
        key: 'edit',
        label: t('riskRegister.row.edit'),
        icon: Pencil,
        hidden: () => !canUpdate,
        onSelect: (r) => setEditRaw(r.raw),
      },
      {
        key: 'mitigate',
        // Derived from what exists: offering "create" on a risk that already has
        // three plans is how duplicates get made.
        label: t('riskRegister.row.mitigate'),
        icon: ShieldCheck,
        hidden: () => !canUpdate,
        onSelect: (r) => {
          const n = r.raw.mitigations_count ?? r.raw.mitigations?.length ?? 0;
          if (n > 0) navigate(`/risks/mitigations?risk_id=${r.id}`);
          else setMitiRiskId(r.id);
        },
      },
      {
        key: 'export',
        label: t('riskRegister.row.export'),
        icon: Download,
        onSelect: (r) => exportRows([r], 'risque'),
      },
      {
        key: 'delete',
        label: t('riskRegister.row.delete'),
        icon: Trash2,
        danger: true,
        separatorBefore: true,
        hidden: () => !canDelete,
        onSelect: (r) => setToDelete(r),
      },
    ],
    // openDrawer / exportRows close over stable setters and the csv context.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, canUpdate, canDelete, navigate, csvCtx, focusId],
  );

  /* ------------------------------------------------------------ bulk actions */
  const afterBulk = () => {
    setSelectionEpoch((n) => n + 1);
    reload();
  };

  const bulkActions: BulkAction<UiRisk>[] = useMemo(
    () => [
      {
        key: 'status',
        label: t('riskRegister.bulk.status'),
        hidden: !canUpdate,
        // The governed endpoint takes explicit ids and caps a batch at 100.
        selectionOnly: true,
        choices: PICKABLE.map((s) => ({ value: s, label: t(`riskRegister.status.${s}`) })),
        run: async ({ ids, rows }, choice) => {
          const status = PICKABLE.find((s) => s === choice);
          if (!status) return;
          // What each row held, to put it back on Undo.
          const before = rows.map((r) => ({ id: r.id, status: r.statusValue }));
          const rollback = patchRisks(ids, { status });
          try {
            await riskService.bulkAction({
              type: 'change_status',
              risk_ids: ids,
              status: status as ApiRiskStatus,
            });
          } catch (err) {
            rollback();
            toast.error(
              isMissingRows(err) ? t('riskRegister.bulk.stale') : t('riskRegister.bulk.failed'),
            );
            throw err;
          }
          toast.success(
            t('riskRegister.bulk.statusDone', {
              count: ids.length,
              status: t(`riskRegister.status.${status}`),
            }),
            {
              duration: 8000,
              action: {
                label: t('riskRegister.bulk.undo'),
                onClick: () => void undoStatus(before),
              },
            },
          );
          afterBulk();
        },
      },
      {
        key: 'owner',
        label: t('riskRegister.bulk.owner'),
        hidden: !canUpdate || members.length === 0,
        selectionOnly: true,
        choices: members.map((m) => ({ value: m.user_id, label: memberName(m) })),
        run: async ({ ids, rows }, choice) => {
          const member = members.find((m) => m.user_id === choice);
          if (!member) return;
          const before = rows.map((r) => ({ id: r.id, ownerId: r.ownerId }));
          const rollback = patchRisks(ids, { owner_id: member.user_id, owner_email: member.email });
          try {
            await riskService.bulkAction({
              type: 'set_owner',
              risk_ids: ids,
              owner_id: member.user_id,
            });
          } catch (err) {
            rollback();
            toast.error(
              isMissingRows(err) ? t('riskRegister.bulk.stale') : t('riskRegister.bulk.failed'),
            );
            throw err;
          }
          // Undo only when every row had a previous owner to go back to: the API
          // cannot un-assign through this endpoint.
          const canUndo = before.every((b) => b.ownerId);
          toast.success(
            t('riskRegister.bulk.ownerDone', { count: ids.length, name: memberName(member) }),
            canUndo
              ? {
                  duration: 8000,
                  action: {
                    label: t('riskRegister.bulk.undo'),
                    onClick: () => void undoOwner(before),
                  },
                }
              : undefined,
          );
          afterBulk();
        },
      },
      {
        key: 'export',
        label: t('riskRegister.bulk.export'),
        icon: Download,
        selectionOnly: true,
        run: ({ rows }) => exportRows(rows, 'risques-selection'),
      },
      {
        key: 'delete',
        label: t('riskRegister.bulk.delete'),
        icon: Trash2,
        danger: true,
        hidden: !canDelete,
        selectionOnly: true,
        // Confirmed with an impact readout before anything is deleted.
        run: ({ ids }) => setBulkToDelete(ids),
      },
    ],
    // undoStatus / undoOwner / exportRows / afterBulk only use stable setters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, canUpdate, canDelete, members, patchRisks, csvCtx, reload],
  );

  const undoStatus = async (before: { id: string; status: RiskStatusValue }[]) => {
    const groups = new Map<RiskStatusValue, string[]>();
    for (const b of before) groups.set(b.status, [...(groups.get(b.status) ?? []), b.id]);
    try {
      for (const [status, ids] of groups) {
        await riskService.bulkAction({
          type: 'change_status',
          risk_ids: ids,
          status: status as ApiRiskStatus,
        });
      }
      toast.success(t('riskRegister.bulk.undone'));
    } catch {
      toast.error(t('riskRegister.bulk.undoFailed'));
    }
    reload();
  };

  const undoOwner = async (before: { id: string; ownerId: string | null }[]) => {
    const groups = new Map<string, string[]>();
    for (const b of before) {
      if (b.ownerId) groups.set(b.ownerId, [...(groups.get(b.ownerId) ?? []), b.id]);
    }
    try {
      for (const [ownerId, ids] of groups) {
        await riskService.bulkAction({ type: 'set_owner', risk_ids: ids, owner_id: ownerId });
      }
      toast.success(t('riskRegister.bulk.undone'));
    } catch {
      toast.error(t('riskRegister.bulk.undoFailed'));
    }
    reload();
  };

  /* ------------------------------------------------------------------ header */
  const activeCount = summary.data?.active.count;
  const ale = summary.data?.active.ale_xaf;
  const subtitle = summary.isError ? (
    <span data-testid="risk-subtitle">
      {t('riskRegister.subtitle.error')}{' '}
      <button
        type="button"
        onClick={() => void summary.refetch()}
        className="font-semibold text-accent-strong hover:underline"
      >
        {t('riskRegister.retry')}
      </button>
    </span>
  ) : activeCount === undefined ? (
    <span
      aria-hidden="true"
      className="or-skeleton inline-block h-3.5 w-72 rounded-md align-middle"
      data-testid="risk-subtitle-loading"
    />
  ) : (
    <span data-testid="risk-subtitle">
      {typeof ale === 'number'
        ? t('riskRegister.subtitle.withAle', {
            count: activeCount,
            ale: millions(money.toDisplay(ale), locale),
            currency: money.currency,
          })
        : t('riskRegister.subtitle.noAle', { count: activeCount })}
    </span>
  );

  const emptyAll = (
    <EmptyState
      icon={ShieldAlert}
      title={t('riskRegister.empty.title')}
      description={canCreate ? t('riskRegister.empty.body') : t('riskRegister.empty.readOnly')}
      primaryAction={
        canCreate ? (
          <Button
            variant="primary"
            icon={Plus}
            onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))}
          >
            {t('riskRegister.newRisk')}
          </Button>
        ) : undefined
      }
    />
  );

  const shown = ui.length;
  // No row to show and the first answer has not come: skeleton, not "empty".
  const loadingFirst = (isLoading || !hasLoaded) && ui.length === 0;
  const matrixTruncated = layout === 'matrix' && total > MATRIX_LIMIT;

  return (
    <PageFrame wide>
      <PageHeader
        title={t('riskRegister.title')}
        subtitle={subtitle}
        actions={
          <>
            {canCreate && (
              <Button variant="secondary" icon={Upload} onClick={() => navigate('/risks/import')}>
                {t('riskRegister.import')}
              </Button>
            )}
            <Button
              variant="secondary"
              icon={Download}
              disabled={shown === 0}
              onClick={() => exportRows(ui, 'risques')}
              data-testid="risk-export"
            >
              {t('riskRegister.export.action')}
            </Button>
            {canCreate && (
              <Button
                variant="primary"
                icon={Plus}
                onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))}
                data-testid="risk-new"
              >
                {t('riskRegister.newRisk')}
              </Button>
            )}
          </>
        }
      />

      <div className="flex items-center gap-3 mb-3.5">
        <Tabs<RiskView>
          id="risk-views"
          label={t('riskRegister.view.label')}
          items={tabItems}
          value={filters.view}
          onChange={setScope}
          className="flex-1 min-w-0"
        />
        {summary.isError && (
          <button
            type="button"
            onClick={() => void summary.refetch()}
            className="shrink-0 text-[12px] font-semibold text-accent-strong hover:underline"
          >
            {t('riskRegister.counts.retry')}
          </button>
        )}
      </div>

      <TabPanel tabsId="risk-views" id={filters.view} active>
        {layout === 'matrix' ? (
          <>
            <div className="flex items-center gap-2 flex-wrap mb-3.5">
              <SearchBox value={state.q} onChange={table.setQuery} />
              {toolbarControls}
              <LayoutSwitch layout={layout} onChange={setLayout} />
            </div>
            {loadError ? (
              <ErrorState
                title={t('riskRegister.error.title')}
                description={t('riskRegister.error.body')}
                retryLabel={t('riskRegister.retry')}
                onRetry={reload}
              />
            ) : loadingFirst ? (
              <SkeletonRows rows={6} />
            ) : ui.length === 0 && !hasFilters && filters.view === 'all' ? (
              emptyAll
            ) : ui.length === 0 ? (
              <NoMatch onReset={resetFilters} />
            ) : (
              <RiskMatrixPanel risks={ui} onOpen={openDrawer} />
            )}
            {matrixTruncated && (
              <p className="mt-2 text-[12px] text-ink-muted" data-testid="risk-matrix-truncated">
                {t('riskRegister.matrix.truncated', { limit: MATRIX_LIMIT, total })}
              </p>
            )}
          </>
        ) : (
          <DataTable
            key={selectionEpoch}
            id="risks"
            ariaLabel={t('riskRegister.title')}
            rows={ui}
            total={total}
            columns={columns}
            rowKey={(r) => r.id}
            api={table}
            mode="server"
            loading={loadingFirst}
            error={loadError}
            onRetry={reload}
            facets={facets}
            searchPlaceholder={t('riskRegister.search.placeholder')}
            selectable
            rowActions={rowActions}
            bulkActions={bulkActions}
            onRowClick={(r) => openDrawer(r.id)}
            // The header's "Exporter" and the bulk bar's own export replace the
            // table's, so there is one way to export, in one format.
            exportFilename=""
            minWidth={1040}
            toolbarExtra={
              <>
                {toolbarControls}
                <LayoutSwitch layout={layout} onChange={setLayout} />
              </>
            }
            empty={emptyAll}
          />
        )}

        {!loadError && (
          <div
            className="flex justify-between gap-4 flex-wrap text-[12px] text-ink-muted mt-2.5"
            data-testid="risk-footer"
          >
            <span data-testid="risk-count">
              {loadingFirst ? (
                <span aria-hidden="true" className="or-skeleton inline-block h-3 w-32 rounded" />
              ) : (
                t('riskRegister.footer.count', {
                  shown: formatNumber(locale, shown),
                  count: total,
                })
              )}
            </span>
            <span data-testid="risk-formula">
              {t('riskRegister.footer.formula')}
              {money.entitled
                ? ` · ${t('riskRegister.footer.ale', { currency: money.currency })}`
                : ''}
            </span>
          </div>
        )}
      </TabPanel>

      {openId && drawerRisk && (
        <RiskDrawer
          // Keyed on the target so a deep link that names a tab lands on it rather
          // than on whatever the previous open left behind.
          key={`${drawerRisk.id}:${target.tab}:${target.section}`}
          r={drawerRisk}
          target={target}
          members={members}
          money={money}
          onClose={closeDrawer}
          onEdit={() => setEditRaw(drawerRisk.raw)}
          onExport={() => exportRows([drawerRisk], 'risque')}
          onCreateMiti={() => setMitiRiskId(drawerRisk.id)}
          onChanged={() => {
            void qc.invalidateQueries({ queryKey: ['risks', 'detail', drawerRisk.id] });
            reload();
          }}
        />
      )}
      {openId && !drawerRisk && (
        <DrawerStatus
          error={detail.isError}
          onRetry={() => void detail.refetch()}
          onClose={closeDrawer}
        />
      )}

      <EditRiskModal
        isOpen={!!editRaw}
        risk={editRaw}
        onClose={() => setEditRaw(null)}
        onSuccess={() => {
          setEditRaw(null);
          reload();
        }}
      />
      <CreateMitigationModal
        isOpen={!!mitiRiskId}
        riskId={mitiRiskId ?? undefined}
        onClose={() => setMitiRiskId(null)}
        onCreated={() => {
          setMitiRiskId(null);
          reload();
          toast.success(t('riskRegister.miti.linked'));
        }}
      />

      <ImpactDialog
        open={!!toDelete}
        title={t('riskRegister.delete.title')}
        subject={toDelete?.name ?? ''}
        description={t('riskRegister.delete.body')}
        impacts={
          toDelete
            ? [
                {
                  label: t('riskRegister.delete.impactMiti'),
                  detail: String(toDelete.raw.mitigations?.length ?? 0),
                },
                {
                  label: t('riskRegister.delete.impactHistory'),
                  detail: t('riskRegister.delete.lost'),
                },
              ]
            : []
        }
        alternatives={
          toDelete
            ? [
                {
                  label: t('riskRegister.delete.exportFirst'),
                  description: t('riskRegister.delete.exportFirstHint'),
                  onClick: () => exportRows([toDelete], 'risque'),
                },
              ]
            : []
        }
        confirmLabel={t('riskRegister.delete.confirm')}
        cancelLabel={t('riskRegister.cancel')}
        loading={deleting}
        onConfirm={confirmDeleteRisk}
        onClose={() => setToDelete(null)}
      />

      <ImpactDialog
        open={!!bulkToDelete}
        title={t('riskRegister.bulk.deleteTitle', { count: bulkToDelete?.length ?? 0 })}
        subject={t('riskRegister.bulk.deleteSubject', { count: bulkToDelete?.length ?? 0 })}
        description={t('riskRegister.delete.body')}
        impacts={[
          { label: t('riskRegister.delete.impactMiti'), detail: t('riskRegister.delete.lost') },
          { label: t('riskRegister.delete.impactHistory'), detail: t('riskRegister.delete.lost') },
        ]}
        alternatives={[
          {
            label: t('riskRegister.delete.exportFirst'),
            description: t('riskRegister.delete.exportFirstHint'),
            onClick: () => {
              const ids = new Set(bulkToDelete ?? []);
              exportRows(
                ui.filter((r) => ids.has(r.id)),
                'risques-selection',
              );
            },
          },
        ]}
        confirmLabel={t('riskRegister.delete.confirm')}
        cancelLabel={t('riskRegister.cancel')}
        loading={deleting}
        onConfirm={confirmBulkDelete}
        onClose={() => setBulkToDelete(null)}
      />
    </PageFrame>
  );
}

/** Filters matched nothing: the design's own sentence, with the way out. */
function NoMatch({ onReset }: { onReset: () => void }) {
  const { t } = useI18n();
  return (
    <div
      className="px-6 py-12 text-center bg-surface-1 border border-border-subtle rounded-[14px]"
      data-testid="risk-nomatch"
    >
      <div className="text-[14px] font-semibold text-ink">{t('riskRegister.noMatch.title')}</div>
      <div className="text-[13px] text-ink-muted mt-1">
        {t('riskRegister.noMatch.body')}{' '}
        <button
          type="button"
          onClick={onReset}
          className="font-semibold text-accent-strong underline"
        >
          {t('riskRegister.filter.reset')}
        </button>
      </div>
    </div>
  );
}
