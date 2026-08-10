// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Risk Register (OpenRisk.dc.html §6.3) — wired to the real /risks store.
//
// The register is the app's most demanding table, so it is the reference
// migration onto <DataTable> (shared/datatable): server-side sort + pagination,
// faceted filters mirrored in the URL with saved views, instant search kept
// distinct from those filters, page-vs-all-results selection driving a
// permission-aware bulk bar, a portalled row menu that cannot be clipped, a
// per-user column layout and CSV export of the selection or of the current
// view. The right-side drawer (Details / Lifecycle / Score / Financial / …)
// is unchanged.

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import { Upload, Plus, X, FileText, Pencil, Trash2, Eye, Download, ShieldCheck, ShieldAlert, Clock, Rows3, LayoutGrid, Check, ChevronDown, ArrowRight, ArrowLeft, RotateCcw, Coins, Route as RouteIcon, SlidersHorizontal, Sparkles, Loader2 } from 'lucide-react';
import {
  PageFrame, PageHeader, Btn, Card, CritBadge, StatusPill, Avatar, FwBadge, arcPath,
  SkeletonRows, EmptyState, softFill, type RiskStatus,
} from '../../shared/ui';
import { DataTable, useTableState, type BulkAction, type Column, type Facet, type RowAction } from '../../shared/datatable';
import { scoreColor, critColor } from '../../shared/riskColors';
import type { Criticality } from '../../shared/riskColors';
import { ImpactDialog } from '../../shared/ImpactDialog';
import { ProgressState } from '../../shared/ProgressState';
import { HistoryTimeline, type HistoryEntry } from '../../shared/HistoryTimeline';
import { useRiskTimeline } from './useRiskTimeline';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useRiskStore, type RiskPhase } from '../../hooks/useRiskStore';
import { useFocusParam } from '../../shared/useFocusParam';
import { useAuthStore } from '../../hooks/useAuthStore';
import { mapRisk, relTime, type UiRisk } from './riskMap';
import { EditRiskModal } from './components/EditRiskModal';
import { CreateMitigationModal } from '../mitigations/CreateMitigationModal';
import { useRiskFinancial } from '../financial/useFinancial';
import { useRiskSmartScore } from './useSmartScore';
import { SmartRiskRadar } from './components/SmartRiskRadar';
import { useTreatmentPlan } from '../ai/useAi';
import { useQueryClient } from '@tanstack/react-query';

/* -------------------------------------------------------------- CSV export */

// exportRiskCsv downloads a single risk as CSV (client-side — no per-risk export
// endpoint). Multi-row export goes through <DataTable>'s own exporter, which
// respects the user's visible columns and their order.
function exportRiskCsv(r: UiRisk) {
  const cols: [string, string | number][] = [
    ['id', r.id], ['name', r.name], ['description', r.desc ?? ''], ['asset', r.asset],
    ['score', r.score], ['criticality', r.crit], ['status', r.status], ['framework', r.fw],
    ['owner', r.ownerName], ['probability', r.prob], ['impact', r.impact], ['asset_criticality', r.ac],
    ['updated', r.mod],
  ];
  const esc = (v: unknown) => {
    const s = v == null ? '' : String(v);
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };
  const csv = `${cols.map((c) => c[0]).join(',')}\n${cols.map((c) => esc(c[1])).join(',')}`;
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = `risk-${r.id.slice(0, 8)}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

export function RiskRegisterPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const risks = useRiskStore((s) => s.risks);
  const total = useRiskStore((s) => s.total);
  const isLoading = useRiskStore((s) => s.isLoading);
  const loadError = useRiskStore((s) => s.error);
  const fetchRisks = useRiskStore((s) => s.fetchRisks);
  const deleteRisk = useRiskStore((s) => s.deleteRisk);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const canDelete = useAuthStore((s) => s.hasPermission('risks:delete'));

  const [view, setView] = useState<'table' | 'map'>('table');
  const [drawerId, setDrawerId] = useState<string | null>(null);
  const [editRaw, setEditRaw] = useState<UiRisk['raw'] | null>(null);
  const [mitiRiskId, setMitiRiskId] = useState<string | null>(null);
  const [toDelete, setToDelete] = useState<UiRisk | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Table state lives in the URL: ?q=&sort=score:desc&page=2&f.criticality=critical
  const table = useTableState({ defaultSort: { key: 'score', dir: 'desc' }, defaultPageSize: 50 });
  const { state } = table;

  // Sort, pagination AND filtering are server-side: the register is the one
  // table that routinely holds thousands of rows, and paging 50 of them is the
  // difference between a snappy page and a 4 MB response.
  const params = useMemo(() => {
    const p: Record<string, string | number> = { page: state.page, limit: state.pageSize };
    if (state.q.trim()) p.q = state.q.trim();
    if (state.sort) {
      p.sort_by = state.sort.key;
      p.sort_dir = state.sort.dir;
    }
    for (const [key, values] of Object.entries(state.filters)) {
      if (values.length) p[key] = values.join(',');
    }
    return p;
  }, [state]);

  const reload = useCallback(() => { void fetchRisks(params).catch(() => {}); }, [fetchRisks, params]);
  useEffect(() => { reload(); }, [reload]);

  const ui: UiRisk[] = useMemo(() => risks.map((r) => mapRisk(r, lang)), [risks, lang]);
  const drawer = drawerId ? ui.find((r) => r.id === drawerId) ?? null : null;

  // Deep-link from universal search (/risks?focus=<id>) → open that risk's drawer.
  const { focusId, clearFocus } = useFocusParam();
  useEffect(() => {
    if (focusId && ui.some((r) => r.id === focusId)) {
      setDrawerId(focusId);
      clearFocus();
    }
  }, [focusId, ui, clearFocus]);

  /* ------------------------------------------------------------- deletion */
  // Important + irreversible (a risk carries linked mitigation plans + history) →
  // impact radiography (UX-11), not a bare confirm.
  const confirmDeleteRisk = async () => {
    const r = toDelete;
    if (!r) return;
    setDeleting(true);
    try {
      await deleteRisk(r.id);
      toast.success(tr('Risque supprimé', 'Risk deleted'));
      if (drawerId === r.id) setDrawerId(null);
      setToDelete(null);
    } catch {
      toast.error(tr('Suppression échouée', 'Delete failed'));
    } finally {
      setDeleting(false);
    }
  };

  /* --------------------------------------------------------------- facets */
  const facets: Facet<UiRisk>[] = useMemo(() => [
    {
      key: 'criticality',
      label: tr('Criticité', 'Criticality'),
      options: [
        { value: 'critical', label: L.critical, color: 'var(--critical)' },
        { value: 'high', label: L.high, color: 'var(--high)' },
        { value: 'medium', label: L.medium, color: 'var(--medium)' },
        { value: 'low', label: L.low, color: 'var(--low)' },
      ],
    },
    {
      key: 'status',
      label: tr('Statut', 'Status'),
      options: [
        { value: 'open', label: tr('Ouvert', 'Open') },
        { value: 'in_progress', label: tr('En cours', 'In progress') },
        { value: 'mitigated', label: tr('Atténué', 'Mitigated') },
        { value: 'accepted', label: tr('Accepté', 'Accepted') },
      ],
    },
    {
      key: 'phase',
      label: tr('Phase (ISO 31000)', 'Phase (ISO 31000)'),
      options: PHASE_ORDER.map((p) => ({ value: p, label: phaseLabel(p, lang) })),
    },
    {
      key: 'source',
      label: tr('Origine', 'Source'),
      options: [
        { value: 'manual', label: tr('Manuel', 'Manual') },
        { value: 'cti_auto', label: tr('CTI (auto)', 'CTI (auto)') },
        { value: 'scan_auto', label: tr('Scanner (auto)', 'Scanner (auto)') },
        { value: 'import', label: tr('Import', 'Import') },
      ],
    },
  ], [L, lang]); // eslint-disable-line react-hooks/exhaustive-deps

  /* -------------------------------------------------------------- columns */
  const columns: Column<UiRisk>[] = useMemo(() => [
    {
      key: 'name',
      header: L.col_name,
      sortKey: 'name',
      hideable: false,
      frozen: true,
      exportValue: (r) => r.name,
      render: (r) => (
        <>
          <div className="text-[13.5px] font-medium text-ink max-w-[340px] truncate">{r.name}</div>
          <div className="mono text-[11px] text-ink-muted mt-0.5">#{r.id.slice(0, 8)} · {r.asset}</div>
        </>
      ),
    },
    {
      key: 'score',
      header: L.col_score,
      sortKey: 'score',
      align: 'right',
      exportValue: (r) => r.score.toFixed(1),
      render: (r) => <span className="mono text-[15px] font-bold" style={{ color: scoreColor(r.score) }}>{r.score.toFixed(1)}</span>,
    },
    {
      key: 'criticality',
      header: L.col_crit,
      sortKey: 'criticality',
      exportValue: (r) => r.crit,
      render: (r) => <CritBadge crit={r.crit} />,
    },
    {
      key: 'status',
      header: L.col_status,
      sortKey: 'status',
      exportValue: (r) => r.status,
      render: (r) => (canUpdate ? <InlineStatus risk={r} /> : <StatusPill status={r.status} />),
    },
    {
      key: 'phase',
      header: tr('Phase', 'Phase'),
      defaultHidden: true,
      exportValue: (r) => r.phase,
      render: (r) => <PhasePill phase={r.phase} lang={lang} />,
    },
    {
      key: 'framework',
      header: L.col_fw,
      exportValue: (r) => r.fw,
      render: (r) => (r.fw !== '—' ? <FwBadge fw={r.fw} /> : <span className="text-ink-muted text-[12px]">—</span>),
    },
    {
      key: 'owner',
      header: L.col_owner,
      exportValue: (r) => r.ownerName,
      render: (r) => (r.owner !== '—' ? <Avatar initials={r.owner} title={r.ownerName} /> : <span className="text-ink-muted text-[12px]">—</span>),
    },
    {
      key: 'updated',
      header: L.col_mod,
      sortKey: 'updated_at',
      exportValue: (r) => r.raw.updated_at ?? r.raw.created_at ?? '',
      render: (r) => <span className="text-[12px] text-ink-soft whitespace-nowrap">{r.mod}</span>,
    },
  ], [L, lang, canUpdate]); // eslint-disable-line react-hooks/exhaustive-deps

  /* ---------------------------------------------------------- row actions */
  const rowActions: RowAction<UiRisk>[] = useMemo(() => [
    { key: 'view', label: tr('Voir', 'View'), icon: Eye, onSelect: (r) => setDrawerId(r.id) },
    { key: 'edit', label: L.edit, icon: Pencil, hidden: () => !canUpdate, onSelect: (r) => setEditRaw(r.raw) },
    { key: 'mitigate', label: L.createMiti, icon: ShieldCheck, hidden: () => !canUpdate, onSelect: (r) => setMitiRiskId(r.id) },
    { key: 'export', label: tr('Exporter CSV', 'Export CSV'), icon: Download, onSelect: (r) => exportRiskCsv(r) },
    { key: 'delete', label: L.del, icon: Trash2, danger: true, separatorBefore: true, hidden: () => !canDelete, onSelect: (r) => setToDelete(r) },
  ], [L, canUpdate, canDelete]); // eslint-disable-line react-hooks/exhaustive-deps

  /* --------------------------------------------------------- bulk actions */
  const bulkActions: BulkAction<UiRisk>[] = useMemo(() => [
    {
      key: 'delete',
      label: L.del,
      icon: Trash2,
      danger: true,
      hidden: !canDelete,
      // Per-id API: refuse to pretend we can delete "all N results" in one go.
      selectionOnly: true,
      run: async ({ ids }) => {
        await Promise.all(ids.map((id) => deleteRisk(id)));
        toast.success(tr(`${ids.length} risque(s) supprimé(s)`, `${ids.length} risk(s) deleted`));
        reload();
      },
    },
  ], [L, canDelete, deleteRisk, reload]); // eslint-disable-line react-hooks/exhaustive-deps

  const critCount = ui.filter((r) => r.crit === 'critical').length;

  return (
    <PageFrame wide>
      <PageHeader
        title={L.riskTitle}
        count={`${total} ${tr('risques', 'risks')}`}
        actions={
          <>
            <div className="inline-flex rounded-[10px] p-0.5" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
              {([['table', Rows3, tr('Table', 'Table')], ['map', LayoutGrid, tr('Matrice', 'Matrix')]] as const).map(([v, Icon, lbl]) => (
                <button
                  key={v}
                  onClick={() => setView(v)}
                  className="h-8 px-2.5 rounded-[8px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 transition-colors"
                  style={{ background: view === v ? 'var(--accent-hover)' : 'transparent', color: view === v ? '#fff' : 'var(--text-secondary)' }}
                  title={lbl}
                >
                  <Icon size={15} /> <span className="hidden sm:inline">{lbl}</span>
                </button>
              ))}
            </div>
            <Btn label={L.importCsv} icon={Upload} onClick={() => navigate('/risks/import')} />
            <Btn label={L.newRisk} icon={Plus} primary onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))} />
          </>
        }
      />

      {view === 'map' ? (
        <Card style={{ padding: '8px 8px 4px', overflow: 'hidden' }}>
          {isLoading && ui.length === 0 ? (
            <SkeletonRows rows={6} />
          ) : ui.length === 0 ? (
            <EmptyState
              icon={ShieldAlert}
              title={tr('Aucun risque pour le moment', 'No risks yet')}
              description={tr('Créez votre premier risque pour commencer à cartographier votre exposition.', 'Create your first risk to start mapping your exposure.')}
              primaryAction={<Btn label={L.newRisk} icon={Plus} primary onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))} />}
            />
          ) : (
            <RiskMatrixView risks={ui} onOpen={setDrawerId} />
          )}
        </Card>
      ) : (
        <DataTable
          id="risks"
          ariaLabel={L.riskTitle}
          rows={ui}
          total={total}
          columns={columns}
          rowKey={(r) => r.id}
          api={table}
          mode="server"
          loading={isLoading}
          error={loadError}
          onRetry={reload}
          facets={facets}
          searchPlaceholder={tr('Rechercher par nom ou description…', 'Search by name or description…')}
          selectable
          rowActions={rowActions}
          bulkActions={bulkActions}
          onRowClick={(r) => setDrawerId(r.id)}
          exportFilename="risques"
          minWidth={880}
          toolbarExtra={
            // A shortcut onto the SAME url-backed facet the panel writes — not a
            // second, divergent filter state.
            <button
              type="button"
              onClick={() => table.toggleFilter('criticality', 'critical')}
              data-testid="quick-critical"
              className="h-9 px-3 rounded-[10px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 shrink-0"
              style={
                (state.filters.criticality ?? []).includes('critical')
                  ? { background: softFill('var(--critical)', 16), color: 'var(--critical)', border: '1px solid transparent' }
                  : { background: 'var(--bg-elevated)', color: 'var(--text-secondary)', border: '1px solid var(--border-strong)' }
              }
            >
              <ShieldAlert size={14} /> {L.critical}{critCount ? ` · ${critCount}` : ''}
            </button>
          }
          empty={
            <EmptyState
              icon={ShieldAlert}
              title={tr('Aucun risque pour le moment', 'No risks yet')}
              description={tr('Créez votre premier risque pour commencer à cartographier votre exposition.', 'Create your first risk to start mapping your exposure.')}
              primaryAction={<Btn label={L.newRisk} icon={Plus} primary onClick={() => window.dispatchEvent(new CustomEvent('openrisk:new-risk'))} />}
            />
          }
        />
      )}

      {drawer && (
        <RiskDrawer
          r={drawer}
          onClose={() => setDrawerId(null)}
          onEdit={() => setEditRaw(drawer.raw)}
          onExport={() => exportRiskCsv(drawer)}
          onCreateMiti={() => setMitiRiskId(drawer.id)}
        />
      )}

      <EditRiskModal
        isOpen={!!editRaw}
        risk={editRaw}
        onClose={() => setEditRaw(null)}
        onSuccess={() => { setEditRaw(null); reload(); }}
      />
      <CreateMitigationModal
        isOpen={!!mitiRiskId}
        riskId={mitiRiskId ?? undefined}
        onClose={() => setMitiRiskId(null)}
        onCreated={() => { setMitiRiskId(null); reload(); toast.success(tr('Plan de mitigation lié au risque', 'Mitigation plan linked to the risk')); }}
      />

      <ImpactDialog
        open={!!toDelete}
        title={tr('Supprimer ce risque ?', 'Delete this risk?')}
        subject={toDelete?.name ?? ''}
        description={tr('Action irréversible. Voici ce qui sera supprimé :', 'This cannot be undone. Here is what will be removed:')}
        impacts={toDelete ? [
          { label: tr('Plans de mitigation liés', 'Linked mitigation plans'), detail: String(toDelete.raw.mitigations?.length ?? 0) },
          { label: tr('Historique & scores du risque', 'Risk history & scores'), detail: tr('perdus', 'lost') },
        ] : []}
        alternatives={toDelete ? [
          {
            label: tr('Exporter le risque (CSV) avant de supprimer', 'Export the risk (CSV) first'),
            description: tr('Gardez une trace hors-ligne avant la suppression.', 'Keep an offline record before deleting.'),
            onClick: () => { if (toDelete) exportRiskCsv(toDelete); },
          },
        ] : []}
        confirmLabel={tr('Supprimer définitivement', 'Delete permanently')}
        cancelLabel={tr('Annuler', 'Cancel')}
        loading={deleting}
        onConfirm={confirmDeleteRisk}
        onClose={() => setToDelete(null)}
      />
    </PageFrame>
  );
}

/* ---------------- risk matrix (map) view ---------------- */
// Standard 5×5 GRC risk map: Impact (x, 1→5) × Probability (y, 5→1). Each risk is
// bucketed from its real prob (0–1) and impact (0–10); the cell tint is the cell's
// own severity (prob-bucket × impact-bucket), and each risk chip opens its drawer.
function cellCrit(p: number, i: number): Criticality {
  const v = p * i;
  return v >= 15 ? 'critical' : v >= 8 ? 'high' : v >= 4 ? 'medium' : 'low';
}
/* ---------------- inline status editor (ghost edit + autosave) ---------------- */

const STATUS_TO_BACKEND: Record<RiskStatus, string> = {
  open: 'open', progress: 'in_progress', mitigated: 'mitigated', accepted: 'accepted',
};
const STATUS_OPTIONS: RiskStatus[] = ['open', 'progress', 'mitigated', 'accepted'];

// Click-to-edit status right in the register row: no modal, no Save button — the
// change autosaves optimistically (System-1 quick edit) and a toast confirms it.
function InlineStatus({ risk }: { risk: UiRisk }) {
  const updateRisk = useRiskStore((s) => s.updateRisk);
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);

  const change = async (s: RiskStatus) => {
    setOpen(false);
    if (s === risk.status) return;
    setSaving(true);
    try {
      await updateRisk(risk.id, { status: STATUS_TO_BACKEND[s] });
      toast.success(tr('Statut mis à jour', 'Status updated'));
    } catch {
      toast.error(tr('Mise à jour échouée', 'Update failed'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="relative inline-block" onClick={(e) => e.stopPropagation()}>
      <button
        onClick={() => setOpen((v) => !v)}
        disabled={saving}
        title={tr('Changer le statut', 'Change status')}
        className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 -mx-1 hover:bg-hover transition-colors"
      >
        <StatusPill status={risk.status} />
        {saving ? <Loader2 size={12} className="animate-spin text-ink-muted" /> : <ChevronDown size={12} className="text-ink-muted" />}
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} aria-hidden="true" />
          <div className="absolute left-0 top-full mt-1 z-50 min-w-[150px] rounded-[10px] p-1 shadow-card-lg" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
            {STATUS_OPTIONS.map((s) => (
              <button
                key={s}
                onClick={() => change(s)}
                className="w-full flex items-center justify-between gap-2 px-2 py-1.5 rounded-[7px] hover:bg-hover transition-colors text-left"
              >
                <StatusPill status={s} />
                {s === risk.status && <Check size={13} className="text-accent" />}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function RiskMatrixView({ risks, onOpen }: { risks: UiRisk[]; onOpen: (id: string) => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const bucket = (v: number, max: number) => Math.min(5, Math.max(1, Math.ceil((v / max) * 5) || 1));
  // grid[prob 1..5][impact 1..5] → risks
  const grid: Record<number, Record<number, UiRisk[]>> = {};
  for (let p = 1; p <= 5; p++) { grid[p] = {}; for (let i = 1; i <= 5; i++) grid[p][i] = []; }
  for (const r of risks) grid[bucket(r.prob, 1)][bucket(r.impact, 10)].push(r);

  return (
    <div className="p-4 overflow-x-auto">
      <div className="flex gap-2" style={{ minWidth: 640 }}>
        {/* y-axis label */}
        <div className="flex items-center">
          <span className="text-[11px] font-semibold uppercase tracking-[.06em] text-ink-muted" style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}>{tr('Probabilité', 'Probability')}</span>
        </div>
        <div className="flex-1">
          <div className="grid gap-1.5" style={{ gridTemplateColumns: 'repeat(5,1fr)' }}>
            {[5, 4, 3, 2, 1].map((p) =>
              [1, 2, 3, 4, 5].map((i) => {
                const cell = grid[p][i];
                const crit = cellCrit(p, i);
                const col = critColor[crit];
                return (
                  <div key={`${p}-${i}`} className="rounded-[10px] p-1.5 min-h-[84px] flex flex-col gap-1" style={{ background: softFill(col, 10), border: `1px solid ${softFill(col, 22)}` }}>
                    <div className="flex flex-wrap gap-1 content-start">
                      {cell.slice(0, 6).map((r) => (
                        <button
                          key={r.id}
                          onClick={() => onOpen(r.id)}
                          title={`${r.name} · ${r.score.toFixed(1)}`}
                          className="w-5 h-5 rounded-full text-[9px] font-bold text-text-primary flex items-center justify-center transition-transform hover:scale-110"
                          style={{ background: col }}
                        >
                          {r.score.toFixed(0)}
                        </button>
                      ))}
                      {cell.length > 6 && <span className="text-[10px] font-semibold self-center" style={{ color: col }}>+{cell.length - 6}</span>}
                    </div>
                  </div>
                );
              })
            )}
          </div>
          {/* x-axis ticks */}
          <div className="grid gap-1.5 mt-1.5" style={{ gridTemplateColumns: 'repeat(5,1fr)' }}>
            {[1, 2, 3, 4, 5].map((i) => <div key={i} className="text-center text-[10.5px] text-ink-muted">{i}</div>)}
          </div>
          <div className="text-center text-[11px] font-semibold uppercase tracking-[.06em] text-ink-muted mt-1">{tr('Impact', 'Impact')}</div>
        </div>
      </div>
    </div>
  );
}

/* ---------------- drawer ---------------- */
// DrawerTimeline — the "Timeline" tab: real time-travel history for this risk
// (who changed what, and when) from GET /risks/:id/timeline, rendered with the
// shared HistoryTimeline primitive (UX-25). Replaces the former "coming soon".
const NIL_UUID = '00000000-0000-0000-0000-000000000000';
function DrawerTimeline({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data, isLoading, error } = useRiskTimeline(r.id);

  const kindOf = (t: string): string | undefined => {
    const k = t.toUpperCase();
    if (k === 'CREATE' || k === 'CREATED') return 'create';
    if (k === 'MITIGATE' || k.startsWith('MITIGATION')) return 'mitigate';
    if (k === 'DELETE' || k === 'DELETED') return 'delete';
    if (k) return 'update';
    return undefined;
  };
  const titleOf = (t: string): string => {
    switch (kindOf(t)) {
      case 'create': return tr('Risque créé', 'Risk created');
      case 'mitigate': return tr('Mitigation appliquée', 'Mitigation applied');
      case 'delete': return tr('Risque supprimé', 'Risk deleted');
      case 'update': return tr('Mise à jour', 'Updated');
      default: return t;
    }
  };
  const actorOf = (changedBy: string): string => {
    if (!changedBy || changedBy === NIL_UUID || changedBy.toLowerCase() === 'system') return tr('Système', 'System');
    return changedBy.length > 12 ? changedBy.slice(0, 8) : changedBy;
  };

  const entries: HistoryEntry[] = (data ?? []).map((e) => ({
    id: e.id,
    kind: kindOf(e.change_type),
    title: titleOf(e.change_type),
    actor: actorOf(e.changed_by),
    at: e.created_at,
    fields: [
      { label: tr('Score', 'Score'), value: (e.score ?? 0).toFixed(1) },
      { label: tr('Statut', 'Status'), value: e.status },
    ],
  }));

  return (
    <div className="py-5 px-[22px]">
      <HistoryTimeline
        entries={entries}
        isLoading={isLoading}
        error={!!error}
        emptyLabel={tr('Aucun changement enregistré pour ce risque.', 'No changes recorded for this risk yet.')}
        errorLabel={tr('Chargement de l’historique impossible.', 'Could not load the history.')}
        formatDate={(iso) => relTime(iso, lang)}
      />
    </div>
  );
}

// DrawerAI — the "IA" tab: generates a synthesis + suggested treatment plan for
// this risk via the live /ai/risks/:id/treatment-plan endpoint (spec §12.1). Claude
// when configured, deterministic template otherwise (shown in the provenance line).
function DrawerAI({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const locale = lang === 'fr' ? 'fr' : 'en';
  const plan = useTreatmentPlan();
  const res = plan.data;

  const stratLabel: Record<string, string> = {
    mitigate: tr('Atténuer', 'Mitigate'),
    accept: tr('Accepter', 'Accept'),
    transfer: tr('Transférer', 'Transfer'),
    avoid: tr('Éviter', 'Avoid'),
  };
  const prioColor: Record<string, string> = { high: 'var(--critical)', medium: 'var(--high)', low: 'var(--accent)' };

  return (
    <div className="py-5 px-[22px]">
      <div className="flex items-start gap-2 mb-4">
        <div className="flex-1">
          <div className="text-[14px] font-semibold text-ink mb-1">{tr('Plan de traitement assisté par IA', 'AI-assisted treatment plan')}</div>
          <div className="text-[12.5px] text-ink-soft leading-relaxed">
            {tr(
              "L'IA synthétise ce risque et propose une stratégie et un plan d'actions, à partir de son score, sa criticité et l'actif lié.",
              'The AI synthesises this risk and proposes a strategy and action plan from its score, criticality and linked asset.',
            )}
          </div>
        </div>
      </div>

      <button
        onClick={() => plan.mutate({ riskId: r.id, locale })}
        disabled={plan.isPending}
        className="w-full h-11 rounded-[12px] flex items-center justify-center gap-2 text-text-primary text-[13.5px] font-semibold disabled:opacity-60"
        style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 2px 12px var(--accent-glow)' }}
      >
        {plan.isPending ? <Loader2 size={17} className="animate-spin" /> : <Sparkles size={17} />}
        {plan.isPending ? tr('Génération…', 'Generating…') : res ? tr('Régénérer', 'Regenerate') : tr('Générer avec l’IA', 'Generate with AI')}
      </button>

      {plan.isPending && (
        // Informative wait (UX-09): the LLM call takes a few seconds — surface the
        // conceptual steps + the risk being analysed instead of a bare spinner.
        <ProgressState
          title={tr('Analyse en cours…', 'Analysing…')}
          stat={tr(`Risque : ${r.name}`, `Risk: ${r.name}`)}
          steps={[
            tr('Analyse du risque et de l’actif lié', 'Analysing the risk and linked asset'),
            tr('Choix de la stratégie de traitement', 'Choosing the treatment strategy'),
            tr('Priorisation du plan d’actions', 'Prioritising the action plan'),
          ]}
        />
      )}

      {plan.isError && (
        <div className="mt-4 text-[12.5px]" style={{ color: 'var(--critical)' }}>
          {tr('La génération a échoué. Réessayez.', 'Generation failed. Please try again.')}
        </div>
      )}

      {res && (
        <div className="mt-5 space-y-4" style={{ animation: 'or-fadeup .25s ease' }}>
          <div>
            <div className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide mb-1.5">{tr('Synthèse', 'Summary')}</div>
            <div className="text-[13px] text-ink leading-relaxed">{res.plan.summary}</div>
          </div>

          <div className="flex items-center gap-2">
            <span className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide">{tr('Stratégie', 'Strategy')}</span>
            <span
              className="text-[12px] font-semibold px-2.5 py-1 rounded-full"
              style={{ background: 'var(--accent-soft)', color: 'var(--accent)', border: '1px solid var(--accent-line)' }}
            >
              {stratLabel[res.plan.recommended_strategy] ?? res.plan.recommended_strategy}
            </span>
          </div>

          <div>
            <div className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide mb-2">{tr('Plan d’actions', 'Action plan')}</div>
            <div className="space-y-2.5">
              {res.plan.actions.map((a, i) => (
                <div key={i} className="p-3 rounded-[11px]" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                  <div className="flex items-center gap-2 mb-1">
                    <span className="w-5 h-5 rounded-full flex items-center justify-center text-[11px] font-bold text-text-primary shrink-0" style={{ background: prioColor[a.priority] ?? 'var(--accent)' }}>{i + 1}</span>
                    <span className="text-[13px] font-semibold text-ink flex-1">{a.title}</span>
                    <span className="text-[10.5px] font-semibold uppercase" style={{ color: prioColor[a.priority] ?? 'var(--accent)' }}>{a.priority}</span>
                  </div>
                  <div className="text-[12.5px] text-ink-soft leading-relaxed pl-7">{a.description}</div>
                </div>
              ))}
            </div>
          </div>

          {res.plan.rationale && (
            <div className="text-[12px] text-ink-soft italic leading-relaxed">{res.plan.rationale}</div>
          )}

          <div className="text-[11px] text-ink-muted pt-1" style={{ borderTop: '1px solid var(--border)' }}>
            <span className="pt-2 inline-block">
              {tr('Généré par', 'Generated by')} : <span className="font-semibold">{res.generated_by}</span>
              {res.generated_by === 'template' && ' · ' + tr('mode local (configurez ANTHROPIC_API_KEY pour Claude)', 'local mode (set ANTHROPIC_API_KEY for Claude)')}
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

function RiskDrawer({ r, onClose, onEdit, onExport, onCreateMiti }: { r: UiRisk; onClose: () => void; onEdit: () => void; onExport: () => void; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const [tab, setTab] = useState<'details' | 'lifecycle' | 'score' | 'smart' | 'financial' | 'miti' | 'timeline' | 'cti' | 'ai'>('details');
  const tabDef: [typeof tab, string][] = [
    ['details', L.tab_details], ['lifecycle', tr('Cycle de vie', 'Lifecycle')], ['score', L.tab_score],
    ['smart', tr('Score intelligent', 'Smart score')],
    ['financial', tr('Financier', 'Financial')], ['miti', L.tab_miti],
    ['timeline', L.tab_timeline], ['cti', L.tab_cti], ['ai', L.tab_ai],
  ];
  return (
    <div className="fixed inset-0 z-[70] flex justify-end" style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)', animation: 'or-fadein .2s ease' }} onClick={onClose}>
      <div
        onClick={(e) => e.stopPropagation()}
        className="h-full flex flex-col"
        style={{ width: 'min(94vw,560px)', background: 'var(--bg-secondary)', borderLeft: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-slidein .3s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="px-[22px] pt-5 pb-3.5">
          <div className="flex items-start gap-3 mb-3">
            <div className="flex-1">
              <div className="mono text-[11px] text-ink-muted mb-[5px]">#{r.id.slice(0, 8)}</div>
              <div className="disp text-[18px] font-bold text-ink leading-snug">{r.name}</div>
            </div>
            <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
          </div>
          <div className="flex items-center gap-2.5 flex-wrap">
            <CritBadge crit={r.crit} />
            <StatusPill status={r.status} />
            <PhasePill phase={r.phase} lang={lang} />
            <span className="mono text-[13px] font-bold ml-auto" style={{ color: scoreColor(r.score) }}>Score {r.score.toFixed(1)}</span>
          </div>
          <div className="flex gap-2 mt-3.5">
            <Btn label={L.edit} icon={Pencil} onClick={onEdit} />
            <Btn label={L.exportCsv} icon={FileText} onClick={onExport} />
          </div>
        </div>

        <div className="flex gap-0.5 px-[22px] overflow-x-auto" style={{ borderBottom: '1px solid var(--border)' }}>
          {tabDef.map(([k, lbl]) => (
            <button key={k} onClick={() => setTab(k)} className="px-3 py-[11px] text-[13px] whitespace-nowrap" style={{ color: tab === k ? 'var(--text-primary)' : 'var(--text-secondary)', fontWeight: tab === k ? 600 : 500, borderBottom: `2px solid ${tab === k ? 'var(--accent)' : 'transparent'}`, marginBottom: -1 }}>{lbl}</button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto">
          {tab === 'details' && <DrawerDetails r={r} onCreateMiti={onCreateMiti} />}
          {tab === 'lifecycle' && <DrawerLifecycle r={r} />}
          {tab === 'score' && <DrawerScore r={r} />}
          {tab === 'smart' && <DrawerSmart r={r} />}
          {tab === 'financial' && <DrawerFinancial r={r} />}
          {tab === 'miti' && <DrawerMiti r={r} onCreateMiti={onCreateMiti} />}
          {tab === 'ai' && <DrawerAI r={r} />}
          {tab === 'timeline' && <DrawerTimeline r={r} />}
          {tab === 'cti' && <div className="py-10 px-[22px] text-center text-[13px] text-ink-soft">{L.soon}</div>}
        </div>
      </div>
    </div>
  );
}

function DrawerDetails({ r, onCreateMiti }: { r: UiRisk; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const field = (lbl: string, val: string) => (
    <div className="mb-4">
      <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">{lbl}</div>
      <div className="text-[13.5px] text-ink">{val}</div>
    </div>
  );
  return (
    <div className="px-[22px] py-5">
      {field('Description', r.desc || '—')}
      <div className="grid grid-cols-2 gap-x-5">
        {field(lang === 'fr' ? 'Actif concerné' : 'Asset', r.asset)}
        {field('Framework', r.fw)}
        {field(L.col_owner, r.ownerName)}
        {field(L.col_mod, r.mod)}
      </div>
      <button onClick={onCreateMiti} className="mt-2 w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-text-primary transition-all hover:brightness-110" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
        <ShieldCheck size={16} /> {L.createMiti}
      </button>
    </div>
  );
}

function DrawerScore({ r }: { r: UiRisk }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const gauge = (val: number, max: number, lbl: string, col: string) => {
    const pct = Math.max(0, Math.min(1, val / max)), cx = 52, cy = 52, rr = 42;
    const track = arcPath(cx, cy, rr, -130, 130);
    const prog = arcPath(cx, cy, rr, -130, -130 + 260 * pct);
    return (
      <div className="text-center">
        <div className="relative mx-auto" style={{ width: 104, height: 92 }}>
          <svg viewBox="0 0 104 96" width={104} height={96}>
            <path d={track} fill="none" stroke="var(--bg-hover)" strokeWidth={9} strokeLinecap="round" />
            <path d={prog} fill="none" stroke={col} strokeWidth={9} strokeLinecap="round" />
          </svg>
          <div className="mono absolute left-0 right-0 text-center text-[20px] font-bold text-ink" style={{ top: 30 }}>{val.toFixed(1)}</div>
        </div>
        <div className="text-[11.5px] text-ink-soft font-medium">{lbl}</div>
      </div>
    );
  };
  return (
    <div className="p-[22px]">
      <div className="flex justify-around mb-[22px]">
        {gauge(r.prob * 10, 10, L.proba, 'var(--accent)')}
        {gauge(r.impact, 10, L.impact, 'var(--high)')}
        {gauge(r.ac, 3, lang === 'fr' ? 'Criticité actif' : 'Asset criticality', 'var(--info)')}
      </div>
      <div className="text-center p-[18px] rounded-[14px]" style={{ background: 'var(--bg-hover)' }}>
        <div className="mono text-[15px] text-ink-soft">
          <span>{r.prob.toFixed(1)}</span><span className="mx-2 text-ink-muted">×</span>
          <span>{r.impact.toFixed(1)}</span><span className="mx-2 text-ink-muted">×</span>
          <span>{r.ac.toFixed(1)}</span><span className="mx-2.5 text-ink-muted">=</span>
          <span className="text-[22px] font-bold" style={{ color: scoreColor(r.score) }}>{r.score.toFixed(1)}</span>
        </div>
        <div className="text-[12px] text-ink-muted mt-2">{lang === 'fr' ? 'Probabilité × Impact × Criticité de l’actif' : 'Probability × Impact × Asset criticality'}</div>
      </div>
    </div>
  );
}

/* ---------------- smart score (multifactor, spec §8) ---------------- */
function DrawerSmart({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const { data, isLoading, isError } = useRiskSmartScore(r.id);

  if (isLoading) {
    return (
      <div className="p-[22px]">
        <SkeletonRows rows={5} />
      </div>
    );
  }
  if (isError || !data) {
    return (
      <div className="py-10 px-[22px] text-center text-[13px] text-ink-soft">
        {tr('Impossible de calculer le score intelligent.', 'Could not compute the smart score.')}
      </div>
    );
  }
  return (
    <div className="p-[22px]">
      <SmartRiskRadar data={data} lang={lang} />
      <button
        onClick={() => navigate('/risks/weighting')}
        className="mt-4 w-full h-9 rounded-[10px] flex items-center justify-center gap-2 text-[12.5px] font-semibold transition-all hover:bg-hover"
        style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}
      >
        <SlidersHorizontal size={15} /> {tr('Configurer les pondérations', 'Configure weights')}
      </button>
      <p className="mt-3 text-[11px] text-ink-muted leading-snug">
        {tr(
          'Score multifactoriel (0–100) : criticité métier, exposition Internet, vulnérabilités, maturité des contrôles, historique d’incidents, exploitabilité, valeur financière et menaces actives (CTI). Le score classique P × I × Criticité reste inchangé.',
          'Multifactor score (0–100): business criticality, internet exposure, vulnerabilities, control maturity, incident history, exploitability, financial value and active threats (CTI). The classic P × I × Criticality score is unchanged.',
        )}
      </p>
    </div>
  );
}

/* ---------------- lifecycle (ISO 31000) ---------------- */
const PHASE_ORDER: RiskPhase[] = ['identified', 'analyzed', 'evaluated', 'treated', 'monitored', 'closed'];
const PHASE_LABELS: Record<RiskPhase, [string, string]> = {
  identified: ['Identifier', 'Identify'],
  analyzed: ['Analyser', 'Analyze'],
  evaluated: ['Évaluer', 'Evaluate'],
  treated: ['Traiter', 'Treat'],
  monitored: ['Surveiller', 'Monitor'],
  closed: ['Clôturer', 'Close'],
};
function phaseLabel(p: RiskPhase, lang: 'fr' | 'en') { return PHASE_LABELS[p][lang === 'fr' ? 0 : 1]; }
function phaseIdx(p: RiskPhase) { return PHASE_ORDER.indexOf(p); }
// Mirrors domain.RiskPhase.CanTransitionTo on the backend (±1 step, →closed from
// anywhere, or reopen from closed). The backend is the source of truth; this only
// decides which buttons to show.
function canTransition(from: RiskPhase, to: RiskPhase): boolean {
  const f = phaseIdx(from), t = phaseIdx(to);
  if (f < 0 || t < 0 || f === t) return false;
  if (to === 'closed') return true;
  if (from === 'closed') return true;
  return Math.abs(t - f) === 1;
}

function PhasePill({ phase, lang }: { phase: RiskPhase; lang: 'fr' | 'en' }) {
  const closed = phase === 'closed';
  const col = closed ? 'var(--text-secondary)' : 'var(--accent)';
  return (
    <span className="inline-flex items-center gap-1.5 h-[22px] px-2.5 rounded-full text-[11.5px] font-semibold" style={{ color: col, background: 'color-mix(in srgb,var(--accent) 12%,transparent)' }}>
      <RouteIcon size={12} /> {phaseLabel(phase, lang)}
    </span>
  );
}

function DrawerLifecycle({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const transitionPhase = useRiskStore((s) => s.transitionPhase);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  const current = r.phase;
  const curIdx = phaseIdx(current);

  const go = async (to: RiskPhase) => {
    setBusy(true);
    try {
      await transitionPhase(r.id, to, note.trim() || undefined);
      toast.success(tr(`Phase : ${phaseLabel(to, lang)}`, `Phase: ${phaseLabel(to, lang)}`));
      setNote('');
    } catch {
      toast.error(tr('Transition refusée', 'Transition rejected'));
    } finally {
      setBusy(false);
    }
  };

  const prev = curIdx > 0 ? PHASE_ORDER[curIdx - 1] : null;
  const next = curIdx < PHASE_ORDER.length - 1 ? PHASE_ORDER[curIdx + 1] : null;

  return (
    <div className="px-[22px] py-5">
      <div className="text-[13px] text-ink-soft mb-4">{tr('Cycle de vie du risque — ISO 31000. La phase est indépendante du statut.', 'Risk lifecycle — ISO 31000. Phase is independent of status.')}</div>

      {/* Vertical stepper */}
      <div className="relative pl-1">
        {PHASE_ORDER.map((p, i) => {
          const done = curIdx > i || current === 'closed' && i < 5;
          const isCurrent = p === current;
          const dotBg = isCurrent ? 'var(--accent)' : done ? 'var(--low)' : 'var(--bg-hover)';
          const dotFg = isCurrent || done ? '#fff' : 'var(--text-muted)';
          return (
            <div key={p} className="flex items-start gap-3 pb-1">
              <div className="flex flex-col items-center">
                <div className="w-[26px] h-[26px] rounded-full flex items-center justify-center text-[12px] font-bold shrink-0" style={{ background: dotBg, color: dotFg, boxShadow: isCurrent ? '0 0 0 4px color-mix(in srgb,var(--accent) 20%,transparent)' : 'none' }}>
                  {done ? <Check size={14} /> : i + 1}
                </div>
                {i < PHASE_ORDER.length - 1 && <div className="w-[2px] h-6" style={{ background: curIdx > i ? 'var(--low)' : 'var(--border)' }} />}
              </div>
              <div className="pt-0.5">
                <div className="text-[13.5px] font-semibold" style={{ color: isCurrent ? 'var(--accent)' : 'var(--text-primary)' }}>{phaseLabel(p, lang)}</div>
                {isCurrent && <div className="text-[11.5px] text-ink-muted">{tr('Phase actuelle', 'Current phase')}</div>}
              </div>
            </div>
          );
        })}
      </div>

      {/* Actions */}
      {canUpdate ? (
        <div className="mt-5 rounded-[12px] p-3.5" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
          <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">{tr('Note (optionnelle)', 'Note (optional)')}</div>
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={2}
            placeholder={tr('Justification de la transition…', 'Transition rationale…')}
            className="w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none mb-3"
            style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}
          />
          <div className="flex flex-wrap gap-2">
            {prev && canTransition(current, prev) && (
              <button disabled={busy} onClick={() => go(prev)} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <ArrowLeft size={14} /> {phaseLabel(prev, lang)}
              </button>
            )}
            {next && canTransition(current, next) && (
              <button disabled={busy} onClick={() => go(next)} className="h-9 px-3.5 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 text-text-primary disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
                {phaseLabel(next, lang)} <ArrowRight size={14} />
              </button>
            )}
            {current !== 'closed' && (
              <button disabled={busy} onClick={() => go('closed')} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <Check size={14} /> {tr('Clôturer', 'Close')}
              </button>
            )}
            {current === 'closed' && (
              <button disabled={busy} onClick={() => go('monitored')} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
                <RotateCcw size={14} /> {tr('Rouvrir', 'Reopen')}
              </button>
            )}
          </div>
        </div>
      ) : (
        <div className="mt-5 text-[12.5px] text-ink-muted">{tr('Lecture seule — permission « risks:update » requise pour faire évoluer la phase.', 'Read-only — the “risks:update” permission is required to advance the phase.')}</div>
      )}
    </div>
  );
}

/* ---------------- financial (CRQ) ---------------- */
function DrawerFinancial({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const updateRisk = useRiskStore((s) => s.updateRisk);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const qc = useQueryClient();
  const raw = r.raw;
  // Live full assessment from the backend (SLE, downtime, worst/avg, ROSI).
  const { data: fin } = useRiskFinancial(r.id);

  const s = (v: number | null | undefined) => (v == null ? '' : String(v));
  const [sle, setSle] = useState(s(raw.sle_xaf));
  const [aro, setAro] = useState(s(raw.aro));
  const [dh, setDh] = useState(s(raw.downtime_hours));
  const [hc, setHc] = useState(s(raw.hourly_downtime_cost_xaf));
  const [dl, setDl] = useState(s(raw.data_loss_cost_xaf));
  const [fn, setFn] = useState(s(raw.fines_xaf));
  const [oc, setOc] = useState(s(raw.other_direct_cost_xaf));
  const [rc, setRc] = useState(s(raw.remediation_cost_xaf));
  const [eff, setEff] = useState(raw.mitigation_effectiveness != null ? Number(raw.mitigation_effectiveness) : 0);
  const [busy, setBusy] = useState(false);

  const fmtXAF = (v?: number) => (v == null ? '—' : `${Math.round(v).toLocaleString('fr-FR')} FCFA`);
  const fmtUSD = (v?: number) => (v == null ? '—' : `$${v.toLocaleString('en-US', { maximumFractionDigits: 0 })}`);
  const fmtPct = (ratio: number) => `${ratio >= 0 ? '+' : ''}${Math.round(ratio * 100)}%`;
  const num = (v: string) => (v.trim() === '' ? null : Number(v));

  const save = async () => {
    setBusy(true);
    try {
      await updateRisk(r.id, {
        sle_xaf: num(sle), aro: num(aro),
        downtime_hours: num(dh), hourly_downtime_cost_xaf: num(hc),
        data_loss_cost_xaf: num(dl), fines_xaf: num(fn), other_direct_cost_xaf: num(oc),
        remediation_cost_xaf: num(rc), mitigation_effectiveness: eff,
      });
      await qc.invalidateQueries({ queryKey: ['financial'] });
      toast.success(tr('Exposition recalculée', 'Exposure recalculated'));
    } catch {
      toast.error(tr('Échec du recalcul', 'Recalculation failed'));
    } finally {
      setBusy(false);
    }
  };

  const stat = (label: string, value: string, tone?: string) => (
    <div className="rounded-[10px] p-2.5" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
      <div className="text-[10px] uppercase tracking-[.05em] text-ink-muted">{label}</div>
      <div className="mono text-[14px] font-bold mt-0.5" style={{ color: tone ?? 'var(--ink)' }}>{value}</div>
    </div>
  );
  const field = (label: string, val: string, set: (v: string) => void, step?: string) => (
    <label className="block">
      <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">{label}</span>
      <input value={val} onChange={(e) => set(e.target.value)} type="number" step={step} min={0} className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }} />
    </label>
  );

  return (
    <div className="px-[22px] py-5">
      <div className="text-[13px] text-ink-soft mb-4">{tr('Quantification financière (FAIR/CRQ) — ALE = SLE × ARO ; ROSI = (ALE − ALE résiduel − remédiation) / remédiation.', 'Financial quantification (FAIR/CRQ) — ALE = SLE × ARO; ROSI = (ALE − residual ALE − remediation) / remediation.')}</div>

      {/* Headline ALE (XAF + USD) */}
      <div className="grid grid-cols-2 gap-3 mb-3">
        <div className="rounded-[12px] p-4" style={{ border: '1px solid color-mix(in srgb,var(--accent) 30%,transparent)', background: 'color-mix(in srgb,var(--accent) 6%,transparent)' }}>
          <div className="text-[10.5px] uppercase tracking-[.06em] text-ink-muted">ALE (FCFA)</div>
          <div className="mono text-[20px] font-bold text-ink mt-1">{fmtXAF(fin?.ale.xaf ?? raw.ale_xaf)}</div>
        </div>
        <div className="rounded-[12px] p-4" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
          <div className="text-[10.5px] uppercase tracking-[.06em] text-ink-muted">ALE (USD)</div>
          <div className="mono text-[20px] font-bold text-ink mt-1">{fmtUSD(fin?.ale.usd ?? raw.ale_usd)}</div>
        </div>
      </div>

      {/* Full assessment breakdown */}
      {fin && (
        <div className="grid grid-cols-3 gap-2 mb-3">
          {stat('SLE', fmtXAF(fin.sle.xaf))}
          {stat(tr('Coût interruptions', 'Downtime cost'), fmtXAF(fin.downtime_cost.xaf))}
          {stat(tr('Pire cas (ALE)', 'Worst-case ALE'), fmtXAF(fin.ale_worst.xaf), 'var(--critical)')}
          {stat(tr('ALE moyen', 'Average ALE'), fmtXAF(fin.ale_average.xaf))}
          {stat(tr('ALE résiduel', 'Residual ALE'), fmtXAF(fin.ale_after.xaf), 'var(--low)')}
          {stat('ROSI', fin.rosi_computable ? fmtPct(fin.rosi) : '—', fin.rosi_computable ? (fin.rosi >= 0 ? 'var(--low)' : 'var(--critical)') : undefined)}
        </div>
      )}
      <div className="text-[11.5px] text-ink-muted mb-4">
        {tr('Base SLE : ', 'SLE basis: ')}
        {fin?.sle_basis === 'explicit'
          ? tr('saisie explicite', 'explicit input')
          : fin?.sle_basis === 'composed'
            ? tr('composé (interruptions + amendes + perte de données)', 'composed (downtime + fines + data loss)')
            : tr('valeur de référence par criticité', 'reference value by criticality')}
      </div>

      {canUpdate && (
        <div className="rounded-[12px] p-3.5" style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}>
          <div className="text-[11px] font-semibold uppercase tracking-[.05em] text-ink-muted mb-2">{tr('Pertes', 'Losses')}</div>
          <div className="grid grid-cols-2 gap-3 mb-3">
            {field(tr('SLE explicite (FCFA)', 'Explicit SLE (FCFA)'), sle, setSle)}
            {field(tr('ARO — Fréquence / an', 'ARO — Frequency / yr'), aro, setAro, '0.1')}
            {field(tr('Heures d’interruption', 'Downtime hours'), dh, setDh, '0.5')}
            {field(tr('Coût horaire (FCFA)', 'Hourly cost (FCFA)'), hc, setHc)}
            {field(tr('Perte de données (FCFA)', 'Data loss (FCFA)'), dl, setDl)}
            {field(tr('Amendes (FCFA)', 'Fines (FCFA)'), fn, setFn)}
            {field(tr('Autre coût direct (FCFA)', 'Other direct cost (FCFA)'), oc, setOc)}
          </div>
          <div className="text-[11px] font-semibold uppercase tracking-[.05em] text-ink-muted mb-2">{tr('Remédiation (ROSI)', 'Remediation (ROSI)')}</div>
          <div className="grid grid-cols-2 gap-3 mb-3 items-end">
            {field(tr('Coût de remédiation (FCFA)', 'Remediation cost (FCFA)'), rc, setRc)}
            <label className="block">
              <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
                {tr('Efficacité', 'Effectiveness')}<span className="mono text-ink">{Math.round(eff * 100)}%</span>
              </span>
              <input value={eff} onChange={(e) => setEff(Number(e.target.value))} type="range" min={0} max={1} step={0.05} className="mt-3 w-full accent-[var(--accent)]" />
            </label>
          </div>
          <button disabled={busy} onClick={save} className="w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-text-primary disabled:opacity-60" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}>
            <Coins size={16} /> {tr('Recalculer l’exposition', 'Recalculate exposure')}
          </button>
          <div className="text-[11px] text-ink-muted mt-2">{tr('SLE explicite prioritaire ; sinon composé depuis interruptions + amendes + perte de données ; sinon référence par criticité.', 'Explicit SLE wins; else composed from downtime + fines + data loss; else reference by criticality.')}</div>
        </div>
      )}
    </div>
  );
}

function DrawerMiti({ r, onCreateMiti }: { r: UiRisk; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const linked = r.raw.mitigations ?? [];
  if (!linked.length) {
    return (
      <div className="py-10 px-[22px] text-center">
        <div className="text-[13px] text-ink-soft mb-3.5">{lang === 'fr' ? 'Aucun plan de mitigation lié.' : 'No linked mitigation plan.'}</div>
        <div className="flex justify-center"><Btn label={L.createMiti} icon={Plus} primary onClick={onCreateMiti} /></div>
      </div>
    );
  }
  return (
    <div className="px-[22px] py-5">
      {linked.map((x) => (
        <div key={x.id} className="p-3.5 rounded-[12px] mb-2.5" style={{ border: '1px solid var(--border)' }}>
          <div className="text-[13.5px] font-medium text-ink mb-2.5">{x.title}</div>
          <div className="h-[5px] rounded-[5px] overflow-hidden mb-2" style={{ background: 'var(--bg-hover)' }}>
            <div className="h-full rounded-[5px]" style={{ width: `${x.progress ?? 0}%`, background: 'var(--low)' }} />
          </div>
          <div className="flex items-center justify-between text-[11.5px] text-ink-muted">
            <span>{x.progress ?? 0}%</span>
            <span className="inline-flex items-center gap-1"><Clock size={12} /> {x.status}</span>
          </div>
        </div>
      ))}
      <div className="flex justify-center mt-2"><Btn label={L.createMiti} icon={Plus} onClick={onCreateMiti} /></div>
    </div>
  );
}
