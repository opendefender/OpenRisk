// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Inventory on the October 2026 redesign (#906): one dense table with type
// pills, the two actions that matter (topology, discovery), and on each asset
// its type, criticality, owner, location, linked risks, open vulnerabilities
// with a KEV tag and the last detection. A row opens the asset's drawer.
//
// Kept from the previous page, outside the mockup's layout: the typed
// attribute search, selection with governed bulk delete, column chooser and
// export, edit / history / delete per row, and creation, import and attribute
// schemas in the "more" menu.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import {
  Atom,
  Boxes,
  History,
  MoreHorizontal,
  Pencil,
  Plus,
  SlidersHorizontal,
  Trash2,
  Upload,
} from 'lucide-react';

import { PageFrame, PageHeader, CritBadge, EmptyState } from '../../shared/ui';
import { Button, Menu, TabPanel, Tabs, type MenuItem } from '../../shared/ds';
import {
  DataTable,
  useTableState,
  type BulkAction,
  type Column,
  type Facet,
  type RowAction,
} from '../../shared/datatable';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useI18n } from '../../hooks/useI18n';
import { formatDate } from '../../i18n/format';
import { ImpactDialog } from '../../shared/ImpactDialog';
import { critColor, type Criticality } from '../../shared/riskColors';
import { useAssets } from './useAssets';
import { useAssetExposure } from './useAssetExposure';
import { CreateAssetModal } from './CreateAssetModal';
import { EditAssetModal } from './EditAssetModal';
import { AssetHistoryDrawer } from './AssetHistoryDrawer';
import { DiscoverButton } from './DiscoverButton';
import {
  categoryCounts,
  categoryOf,
  locationOf,
  slugOf,
  typeIconOf,
  type CategoryTab,
} from './inventoryRow';
import { useFocusParam } from '../../shared/useFocusParam';
import { useDrawerController } from '../entity-drawer/drawerState';
import { AttributeSearchBar } from '../attackSurface/AttributeSearchBar';
import { CATEGORY_LABELS, type AssetCategory } from '../attackSurface/schemaTypes';
import type { Asset } from '../../types/asset';
import { BulkPreviewDialog, useGovernedBulk, type BulkChangeInput } from '../../shared/bulk';

const CRIT_RANK: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };
const critOf = (a: Asset) => (a.criticality ?? 'LOW').toLowerCase() as Criticality;

// Derived asset score = the max score of its linked risks (null when none).
const scoreOf = (a: Asset): number | null => {
  const rs = a.risks ?? [];
  if (!rs.length) return null;
  return Math.max(...rs.map((r) => r.score ?? 0));
};

/**
 * The pending change, applied to the cached inventory (ABSOLUTE RULE 10).
 * Restored verbatim by the hook if the server refuses — criterion 8.
 *
 * The ['assets'] prefix also caches each asset's history, which is not an array
 * of assets; anything that is not the inventory is handed back untouched.
 * Assets support delete alone, so there is no second branch to write.
 */
function patchInventory(
  cached: unknown,
  ids: ReadonlySet<string>,
  change: BulkChangeInput,
): unknown {
  if (change.action !== 'delete' || !isAssetList(cached)) return cached;
  return cached.filter((a) => !ids.has(a.id as string));
}

function isAssetList(cached: unknown): cached is Asset[] {
  return (
    Array.isArray(cached) && cached.every((a) => typeof a === 'object' && a !== null && 'id' in a)
  );
}

export function InventoryPage() {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const { open: openDrawer } = useDrawerController();
  // Typed-attribute search (Attack Surface §1). The terms travel to the server
  // as ?category=&attr.<key>=<value>; the matching rules live there, once.
  const [attrFilter, setAttrFilter] = useState<{
    category: AssetCategory | '';
    attributes: Record<string, string>;
  }>({ category: '', attributes: {} });
  const [attrOpen, setAttrOpen] = useState(false);
  const [category, setCategory] = useState<CategoryTab>('all');
  const [kevOnly, setKevOnly] = useState(false);
  const { assets, isLoading, isError, refetch, deleteAsset } = useAssets({
    category: attrFilter.category || undefined,
    attributes: attrFilter.attributes,
  });
  const exposure = useAssetExposure();
  // Creation is offered only to a member who may create an asset (#739).
  const canCreate = useAuthStore((s) => s.hasPermission('assets:create'));
  const canUpdate = useAuthStore((s) => s.hasPermission('assets:update'));
  const canDelete = useAuthStore((s) => s.hasPermission('assets:delete'));
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Asset | undefined>(undefined);
  const [historyAssetId, setHistoryAssetId] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<Asset | null>(null);
  const [deleting, setDeleting] = useState(false);

  // GET /assets returns the whole inventory (no server pagination — the graph
  // view needs every node), so the table filters, sorts and pages client-side.
  const table = useTableState({ defaultSort: { key: 'crit', dir: 'desc' }, defaultPageSize: 50 });

  // Deep-link from universal search (/assets?focus=<id>): derived from the URL,
  // so it resolves as soon as the asset lands in the loaded list.
  const { focusId, clearFocus } = useFocusParam();
  const focused = focusId ? assets.find((a) => a.id === focusId) : undefined;
  const editTarget = editing ?? focused;
  const closeEditor = () => {
    setEditing(undefined);
    clearFocus();
  };

  const tabs = useMemo(
    () =>
      categoryCounts(assets).map((c) => ({
        id: c.id,
        label: t(`inventory.cat.${c.id}`),
        count: c.n,
        testId: `inv-cat-${c.id}`,
      })),
    [assets, t],
  );
  const kevAssets = exposure.allowed
    ? assets.filter((a) => (exposure.byAsset.get(a.id as string)?.kev_open ?? 0) > 0).length
    : 0;
  const rows = useMemo(
    () =>
      assets.filter(
        (a) =>
          (category === 'all' || categoryOf(a) === category) &&
          (!kevOnly || (exposure.byAsset.get(a.id as string)?.kev_open ?? 0) > 0),
      ),
    [assets, category, kevOnly, exposure.byAsset],
  );
  const criticalCount = assets.filter((a) => critOf(a) === 'critical').length;

  const facets: Facet<Asset>[] = useMemo(
    () => [
      {
        key: 'criticality',
        label: t('inventory.col.crit'),
        options: (['critical', 'high', 'medium', 'low'] as const).map((c) => ({
          value: c,
          label: t(`inventory.crit.${c}`),
          color: critColor[c],
        })),
        matches: (a, selected) => selected.includes(critOf(a)),
      },
    ],
    [t],
  );

  const columns: Column<Asset>[] = useMemo(
    () => [
      {
        key: 'name',
        header: t('inventory.col.asset'),
        frozen: true,
        hideable: false,
        sortValue: (a) => (a.name ?? '').toLowerCase(),
        exportValue: (a) => a.name ?? '',
        render: (a) => {
          const slug = slugOf(a);
          return (
            <div className="min-w-0">
              <div className="text-[13px] font-medium text-ink truncate">{a.name}</div>
              {slug && <div className="mono text-[11px] text-ink-muted truncate">{slug}</div>}
            </div>
          );
        },
      },
      {
        key: 'type',
        header: t('inventory.col.type'),
        sortValue: (a) => a.type ?? '',
        exportValue: (a) => a.type ?? '',
        render: (a) => {
          const Icon = typeIconOf(a);
          return (
            <span className="inline-flex items-center gap-[7px] text-[13px] text-ink-soft">
              <Icon size={15} className="text-ink-muted shrink-0" aria-hidden="true" />
              {a.type || '—'}
            </span>
          );
        },
      },
      {
        key: 'crit',
        header: t('inventory.col.crit'),
        sortValue: (a) => CRIT_RANK[critOf(a)] ?? 0,
        exportValue: (a) => a.criticality ?? '',
        render: (a) => <CritBadge crit={critOf(a)} />,
      },
      {
        key: 'owner',
        header: t('inventory.col.owner'),
        sortValue: (a) => (a.owner ?? '').toLowerCase(),
        exportValue: (a) => a.owner ?? '',
        render: (a) => (
          <span className="text-[13px] text-ink-soft whitespace-nowrap">{a.owner || '—'}</span>
        ),
      },
      {
        key: 'location',
        header: t('inventory.col.location'),
        sortValue: (a) => locationOf(a).toLowerCase(),
        exportValue: (a) => locationOf(a),
        render: (a) => <span className="text-[13px] text-ink-soft">{locationOf(a) || '—'}</span>,
      },
      {
        key: 'risks',
        header: t('inventory.col.risks'),
        align: 'right',
        sortValue: (a) => a.risks?.length ?? 0,
        exportValue: (a) => a.risks?.length ?? 0,
        render: (a) => <span className="mono font-semibold text-ink">{a.risks?.length ?? 0}</span>,
      },
      {
        key: 'vulns',
        header: t('inventory.col.vulns'),
        align: 'right',
        sortValue: (a) => exposure.byAsset.get(a.id as string)?.open_vulnerabilities ?? 0,
        exportValue: (a) => exposure.byAsset.get(a.id as string)?.open_vulnerabilities ?? '',
        render: (a) => {
          if (!exposure.allowed)
            return (
              <span className="text-ink-muted" title={t('inventory.vulnsHidden')}>
                —
              </span>
            );
          const e = exposure.byAsset.get(a.id as string);
          const n = e?.open_vulnerabilities ?? 0;
          return (
            <span className="whitespace-nowrap" data-testid="inv-vulns">
              <span className={`mono font-semibold ${n ? 'text-ink' : 'text-ink-muted'}`}>{n}</span>
              {(e?.kev_open ?? 0) > 0 && (
                <span
                  className="ml-1.5 text-[10px] font-bold tracking-[.04em] px-[5px] py-0.5 rounded bg-danger-surface text-danger-text"
                  title={t('inventory.kevTitle')}
                >
                  {t('inventory.kev')}
                </span>
              )}
            </span>
          );
        },
      },
      {
        key: 'detected',
        header: <span title={t('inventory.detectedHint')}>{t('inventory.col.detected')}</span>,
        headerLabel: t('inventory.col.detected'),
        sortValue: (a) => {
          const d = exposure.byAsset.get(a.id as string)?.last_detected_at;
          return d ? new Date(d).getTime() : 0;
        },
        exportValue: (a) => exposure.byAsset.get(a.id as string)?.last_detected_at ?? '',
        render: (a) => {
          const d = exposure.allowed
            ? exposure.byAsset.get(a.id as string)?.last_detected_at
            : null;
          return (
            <span className="mono text-[12px] text-ink-soft">
              {d ? formatDate(locale, d, { day: 'numeric', month: 'short' }) : '—'}
            </span>
          );
        },
      },
      {
        key: 'category',
        header: t('inventory.col.category'),
        defaultHidden: true,
        sortValue: (a) => a.category ?? '',
        exportValue: (a) => a.category ?? '',
        render: (a) =>
          a.category ? (
            <span className="text-[12.5px] text-ink-soft">
              {CATEGORY_LABELS[a.category as AssetCategory] ?? a.category}
            </span>
          ) : (
            <span className="text-[12px] text-ink-muted">{t('inventory.untyped')}</span>
          ),
      },
      {
        key: 'score',
        header: t('inventory.col.score'),
        align: 'right',
        defaultHidden: true,
        sortValue: (a) => scoreOf(a) ?? -1,
        exportValue: (a) => scoreOf(a)?.toFixed(1) ?? '',
        render: (a) => {
          const sc = scoreOf(a);
          return sc != null ? (
            <span className="mono font-semibold text-ink">{sc.toFixed(1)}</span>
          ) : (
            <span className="text-ink-muted">—</span>
          );
        },
      },
    ],
    [t, locale, exposure.allowed, exposure.byAsset],
  );

  const rowActions: RowAction<Asset>[] = useMemo(
    () => [
      {
        key: 'edit',
        label: t('inventory.action.edit'),
        icon: Pencil,
        hidden: () => !canUpdate,
        onSelect: (a) => setEditing(a),
      },
      {
        key: 'history',
        label: t('inventory.action.history'),
        icon: History,
        onSelect: (a) => setHistoryAssetId(a.id as string),
      },
      {
        key: 'delete',
        label: t('inventory.action.delete'),
        icon: Trash2,
        danger: true,
        separatorBefore: true,
        hidden: () => !canDelete,
        onSelect: (a) => setToDelete(a),
      },
    ],
    [t, canUpdate, canDelete],
  );

  // Governed (#582): preview → confirm → one transactional, audited request.
  // Previously this fanned the selection out into one DELETE per row, which was
  // neither atomic nor attributable.
  const bulk = useGovernedBulk({
    register: 'assets',
    optimistic: { queryKey: ['assets'], apply: patchInventory },
    onApplied: (result) => {
      const n = result.applied ?? 0;
      toast.success(t('inventory.bulkDeleted', { count: n }));
    },
  });

  const bulkActions: BulkAction<Asset>[] = useMemo(
    () => [
      {
        key: 'delete',
        label: t('inventory.action.delete'),
        icon: Trash2,
        danger: true,
        // The permission the user holds AND what the server says this register
        // can do. Enforcement is the route middleware, not either of these.
        hidden: !canDelete || !bulk.supports('delete'),
        selectionOnly: true,
        run: ({ ids }) => bulk.request({ action: 'delete' }, ids),
      },
    ],
    [t, canDelete, bulk],
  );

  const confirmDelete = async () => {
    if (!toDelete) return;
    setDeleting(true);
    try {
      await deleteAsset.mutateAsync(toDelete.id as string);
      toast.success(t('inventory.delete.done'));
      setToDelete(null);
    } catch {
      toast.error(t('inventory.delete.failed'));
    } finally {
      setDeleting(false);
    }
  };

  const moreItems: MenuItem[] = [
    ...(canCreate
      ? [
          { label: t('inventory.newAsset'), icon: Plus, onSelect: () => setCreating(true) },
          {
            label: t('inventory.import'),
            icon: Upload,
            onSelect: () => navigate('/assets/import'),
          },
        ]
      : []),
    {
      label: t('inventory.schemas'),
      icon: SlidersHorizontal,
      onSelect: () => navigate('/assets/schemas'),
    },
  ];

  const summary = [
    t('inventory.summary', { count: assets.length }),
    t('inventory.critical', { count: criticalCount }),
  ].join(' · ');

  return (
    <PageFrame wide>
      <PageHeader
        className="!mb-[18px]"
        title={t('inventory.title')}
        subtitle={
          isLoading ? undefined : (
            <span data-testid="inv-summary">
              {summary} ·{' '}
              <span
                title={t('inventory.coverageHint')}
                className="underline decoration-dotted underline-offset-2"
              >
                {t('inventory.coverageUnmeasured')}
              </span>
            </span>
          )
        }
        actions={
          <>
            <Menu
              label={t('inventory.more')}
              trigger={
                <Button variant="ghost" icon={MoreHorizontal} aria-label={t('inventory.more')} />
              }
              items={moreItems}
            />
            <Button variant="secondary" icon={Atom} onClick={() => navigate('/assets/topology')}>
              {t('inventory.topology')}
            </Button>
            <DiscoverButton />
          </>
        }
      />

      {attrOpen && (
        <div className="mb-3">
          <AttributeSearchBar
            category={attrFilter.category}
            attributes={attrFilter.attributes}
            onChange={setAttrFilter}
            resultCount={assets.length}
          />
        </div>
      )}

      <Tabs
        id="inv-cat"
        items={tabs}
        value={tabs.some((x) => x.id === category) ? category : 'all'}
        onChange={(id) => setCategory(id)}
        label={t('inventory.typesLabel')}
        className="mb-3"
      />
      <TabPanel tabsId="inv-cat" id={tabs.some((x) => x.id === category) ? category : 'all'} active>
        <DataTable
          id="assets"
          ariaLabel={t('inventory.title')}
          rows={rows}
          columns={columns}
          rowKey={(a) => a.id as string}
          api={table}
          mode="client"
          loading={isLoading}
          error={isError}
          onRetry={() => void refetch()}
          facets={facets}
          clientSearch={(a, q) =>
            `${a.name ?? ''} ${slugOf(a)} ${a.type ?? ''} ${a.owner ?? ''} ${locationOf(a)}`
              .toLowerCase()
              .includes(q)
          }
          searchPlaceholder={t('inventory.search')}
          toolbarExtra={
            <>
              {kevAssets > 0 && (
                <button
                  type="button"
                  aria-pressed={kevOnly}
                  onClick={() => setKevOnly((v) => !v)}
                  title={t('inventory.kevTitle')}
                  data-testid="inv-kev-only"
                  className={`h-9 px-3 rounded-[10px] border text-[12.5px] font-semibold inline-flex items-center gap-1.5 shrink-0 transition-colors ${
                    kevOnly
                      ? 'bg-danger-surface border-danger-text text-danger-text'
                      : 'bg-transparent border-border-default text-ink-soft hover:border-border-strong'
                  }`}
                >
                  {t('inventory.kevOnly')}
                  <span className="mono text-[11px]">{kevAssets}</span>
                </button>
              )}
              <Button
                variant={attrOpen || attrFilter.category ? 'secondary' : 'ghost'}
                icon={SlidersHorizontal}
                aria-pressed={attrOpen}
                aria-label={t('inventory.attrSearch')}
                title={t('inventory.attrSearch')}
                onClick={() => setAttrOpen((o) => !o)}
                data-testid="inv-attr"
              />
            </>
          }
          selectable
          rowActions={rowActions}
          bulkActions={bulkActions}
          onRowClick={(a) => openDrawer('asset', a.id as string)}
          exportFilename="inventaire-actifs"
          minWidth={980}
          empty={
            <EmptyState
              icon={Boxes}
              title={t('inventory.emptyTitle')}
              description={canCreate ? t('inventory.emptyCreate') : t('inventory.emptyRead')}
              primaryAction={
                canCreate ? (
                  <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                    {t('inventory.newAsset')}
                  </Button>
                ) : undefined
              }
            />
          }
        />
      </TabPanel>

      <CreateAssetModal isOpen={creating} onClose={() => setCreating(false)} />
      <EditAssetModal
        asset={editTarget}
        onClose={closeEditor}
        onShowHistory={(id) => {
          closeEditor();
          setHistoryAssetId(id);
        }}
      />
      <AssetHistoryDrawer assetId={historyAssetId} onClose={() => setHistoryAssetId(null)} />

      <BulkPreviewDialog bulk={bulk} entityLabel={t('inventory.delete.entity')} />

      <ImpactDialog
        open={!!toDelete}
        title={t('inventory.delete.title')}
        subject={toDelete?.name ?? ''}
        description={t('inventory.delete.description')}
        impacts={
          toDelete
            ? [
                {
                  label: t('inventory.delete.risks'),
                  detail: String(toDelete.risks?.length ?? 0),
                },
                {
                  label: t('inventory.delete.deps'),
                  detail: t('inventory.delete.depsDetail'),
                },
                {
                  label: t('inventory.delete.history'),
                  detail: t('inventory.delete.historyDetail'),
                },
              ]
            : []
        }
        confirmLabel={t('inventory.delete.confirm')}
        cancelLabel={t('inventory.delete.cancel')}
        loading={deleting}
        onConfirm={confirmDelete}
        onClose={() => setToDelete(null)}
      />
    </PageFrame>
  );
}
