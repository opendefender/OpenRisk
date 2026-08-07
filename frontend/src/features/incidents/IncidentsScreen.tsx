// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Incidents register (routed at /incidents) — the real, backend-wired incident
// hub: KPI header (from /incidents/stats), status filter chips, a table with
// inline status change + row click to open the detail/edit drawer, CSV export,
// and a create dialog. The fixture War Room console lives at
// /incidents/:id/war-room (Preview) and is reachable per-incident.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { toast } from 'sonner';
import { Siren, Plus, Trash2, Radio, ShieldAlert, CheckCircle2, Activity, X, Loader2, Download } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, EmptyState, softFill } from '../../shared/ui';
import { DataTable, useTableState, type BulkAction, type Column, type Facet, type RowAction } from '../../shared/datatable';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { relTime } from '../risks/riskMap';
import { useSoftDelete } from '../../shared/useSoftDelete';
import { useIncidents, useIncidentStats } from './useIncidents';
import { IncidentDrawer } from './IncidentDrawer';
import { SEV, STATUS, STATUSES, SEVERITIES, TYPES, sevMeta, statusMeta } from './incidentMeta';
import { exportIncidentsCsv } from './incidentService';
import type { Incident, IncidentSeverity, IncidentStatus, CreateIncidentInput } from './incidentService';

export function IncidentsScreen() {
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const hasRole = useAuthStore((s) => s.hasRole);
  const canWrite = hasRole('admin') || hasRole('analyst');

  const [showCreate, setShowCreate] = useState(false);
  const [selected, setSelected] = useState<Incident | null>(null);
  const [exporting, setExporting] = useState(false);

  // URL-backed table state. GET /incidents has filters but no pagination, so the
  // table runs in client mode over the loaded register.
  const table = useTableState({ defaultSort: { key: 'reported', dir: 'desc' }, defaultPageSize: 50 });
  const statusFilter = (table.state.filters.status?.[0] ?? '') as '' | IncidentStatus;

  const { data: stats } = useIncidentStats();
  const { incidents, isLoading, isError, refetch, updateIncident, deleteIncident } = useIncidents(statusFilter ? { status: statusFilter } : {});

  const setStatus = (inc: Incident, status: IncidentStatus) => {
    if (status === inc.status) return;
    updateIncident.mutate(
      { id: inc.id, input: { status } },
      { onError: () => toast.error(tr('Mise à jour échouée', 'Update failed')) }
    );
  };

  // Soft delete: hide the row immediately + Undo toast; the API delete defers.
  // (Incident ids are numeric; the hook keys on strings.)
  const { pending, remove: softDeleteIncident } = useSoftDelete<Incident>({
    idOf: (inc) => String(inc.id),
    onCommit: (id) => deleteIncident.mutateAsync(Number(id)),
    message: (inc, lang) => (lang === 'fr' ? `Incident « ${inc.title} » supprimé` : `Incident "${inc.title}" deleted`),
  });
  const visibleIncidents = useMemo(() => incidents.filter((inc) => !pending.has(String(inc.id))), [incidents, pending]);
  const remove = (inc: Incident) => softDeleteIncident(inc);

  const exportCsv = async () => {
    setExporting(true);
    try {
      const n = await exportIncidentsCsv(statusFilter ? { status: statusFilter } : {});
      toast.success(tr(`${n} incident(s) exporté(s)`, `${n} incident(s) exported`));
    } catch {
      toast.error(tr('Export échoué', 'Export failed'));
    } finally {
      setExporting(false);
    }
  };

  const openWarRoom = () => {
    const target = incidents.find((i) => i.status === 'open' || i.status === 'in_progress') ?? incidents[0];
    if (!target) { toast.message(tr('Aucun incident à ouvrir', 'No incident to open')); return; }
    navigate(`/incidents/${target.id}/war-room`);
  };

  const kpis = [
    { icon: Siren, label: tr('Total', 'Total'), value: stats?.total_incidents ?? 0, color: 'var(--accent)' },
    { icon: Radio, label: tr('Ouverts', 'Open'), value: stats?.open_incidents ?? 0, color: 'var(--critical)' },
    { icon: ShieldAlert, label: tr('Critiques', 'Critical'), value: stats?.critical_incidents ?? 0, color: 'var(--high)' },
    { icon: CheckCircle2, label: tr('Taux de résolution', 'Resolution rate'), value: `${Math.round(stats?.resolution_rate ?? 0)}%`, color: 'var(--low)' },
  ];

  /* --------------------------------------------------------------- facets */
  const facets: Facet<Incident>[] = useMemo(() => [
    {
      // Single-choice: this facet is pushed to the API (?status=), which takes one.
      key: 'status',
      label: tr('Statut', 'Status'),
      single: true,
      options: STATUSES.map((st) => ({ value: st, label: tr(STATUS[st].fr, STATUS[st].en), color: STATUS[st].color })),
      matches: () => true,
    },
    {
      key: 'severity',
      label: tr('Sévérité', 'Severity'),
      options: SEVERITIES.map((sv) => ({ value: sv, label: tr(SEV[sv].fr, SEV[sv].en), color: SEV[sv].color })),
      matches: (inc, selectedValues) => selectedValues.includes(inc.severity),
    },
    {
      key: 'type',
      label: tr('Type', 'Type'),
      options: TYPES.map((ty) => ({ value: ty, label: ty.replace('_', ' ') })),
      matches: (inc, selectedValues) => selectedValues.includes(inc.incident_type ?? ''),
    },
  ], [lang]); // eslint-disable-line react-hooks/exhaustive-deps

  /* -------------------------------------------------------------- columns */
  const columns: Column<Incident>[] = useMemo(() => [
    {
      key: 'title',
      header: tr('Incident', 'Incident'),
      frozen: true,
      hideable: false,
      sortValue: (inc) => inc.title.toLowerCase(),
      exportValue: (inc) => inc.title,
      render: (inc) => (
        <>
          <div className="text-[13.5px] font-medium text-ink">{inc.title}</div>
          {inc.description && <div className="text-[12px] text-ink-muted mt-0.5 max-w-[420px] leading-snug truncate">{inc.description}</div>}
        </>
      ),
    },
    { key: 'type', header: tr('Type', 'Type'), sortValue: (inc) => inc.incident_type ?? '', exportValue: (inc) => inc.incident_type ?? '', render: (inc) => <span className="text-[12px] text-ink-soft capitalize">{inc.incident_type?.replace('_', ' ') || '—'}</span> },
    {
      key: 'severity',
      header: tr('Sévérité', 'Severity'),
      sortValue: (inc) => SEVERITIES.indexOf(inc.severity),
      exportValue: (inc) => inc.severity,
      render: (inc) => (
        <span className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: sevMeta(inc.severity).color, background: softFill(sevMeta(inc.severity).color, 15) }}>
          <span className="w-1.5 h-1.5 rounded-full" style={{ background: sevMeta(inc.severity).color }} />
          {tr(sevMeta(inc.severity).fr, sevMeta(inc.severity).en)}
        </span>
      ),
    },
    {
      key: 'status',
      header: tr('Statut', 'Status'),
      sortValue: (inc) => inc.status,
      exportValue: (inc) => inc.status,
      render: (inc) => (
        <div className="relative inline-flex items-center" onClick={(e) => e.stopPropagation()}>
          <span className="w-2 h-2 rounded-full absolute left-2.5 pointer-events-none" style={{ background: statusMeta(inc.status).color }} />
          <select
            value={inc.status}
            disabled={!canWrite}
            aria-label={tr('Statut de l’incident', 'Incident status')}
            onChange={(e) => setStatus(inc, e.target.value as IncidentStatus)}
            className="appearance-none text-[12px] font-semibold rounded-full pl-6 pr-6 py-1.5 outline-none disabled:opacity-70"
            style={{ color: statusMeta(inc.status).color, background: `color-mix(in srgb,${statusMeta(inc.status).color} 12%,transparent)`, border: `1px solid color-mix(in srgb,${statusMeta(inc.status).color} 30%,transparent)`, cursor: canWrite ? 'pointer' : 'not-allowed' }}
          >
            {STATUSES.map((st) => (
              <option key={st} value={st} style={{ color: 'var(--text-primary)', background: 'var(--bg-elevated)' }}>{tr(STATUS[st].fr, STATUS[st].en)}</option>
            ))}
          </select>
        </div>
      ),
    },
    {
      key: 'reported',
      header: tr('Signalé', 'Reported'),
      sortValue: (inc) => new Date(inc.created_at ?? 0).getTime(),
      exportValue: (inc) => inc.created_at ?? '',
      render: (inc) => (
        <>
          <div className="text-[12.5px] text-ink-soft">{relTime(inc.created_at, lang)}</div>
          {inc.reported_by && <div className="text-[11.5px] text-ink-muted">{inc.reported_by}</div>}
        </>
      ),
    },
  ], [lang, canWrite]); // eslint-disable-line react-hooks/exhaustive-deps

  /* -------------------------------------------------------------- actions */
  const rowActions: RowAction<Incident>[] = useMemo(() => [
    { key: 'open', label: tr('Ouvrir', 'Open'), icon: Activity, onSelect: (inc) => setSelected(inc) },
    { key: 'warroom', label: tr('War Room', 'War Room'), icon: Radio, onSelect: (inc) => navigate(`/incidents/${inc.id}/war-room`) },
    { key: 'delete', label: tr('Supprimer', 'Delete'), icon: Trash2, danger: true, separatorBefore: true, hidden: () => !canWrite, onSelect: (inc) => remove(inc) },
  ], [lang, canWrite, navigate]); // eslint-disable-line react-hooks/exhaustive-deps

  const bulkActions: BulkAction<Incident>[] = useMemo(() => [
    {
      key: 'resolve',
      label: tr('Marquer résolus', 'Mark resolved'),
      icon: CheckCircle2,
      hidden: !canWrite,
      selectionOnly: true,
      run: async ({ rows }) => {
        await Promise.all(rows.map((inc) => updateIncident.mutateAsync({ id: inc.id, input: { status: 'resolved' } })));
        toast.success(tr(`${rows.length} incident(s) résolu(s)`, `${rows.length} incident(s) resolved`));
      },
    },
    {
      key: 'delete',
      label: tr('Supprimer', 'Delete'),
      icon: Trash2,
      danger: true,
      hidden: !canWrite,
      selectionOnly: true,
      run: async ({ rows }) => { rows.forEach((inc) => remove(inc)); },
    },
  ], [lang, canWrite, updateIncident]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Incidents', 'Incidents')}
        count={stats ? `${stats.open_incidents} ${tr('ouverts', 'open')}` : undefined}
        actions={
          <>
            <Btn label={exporting ? tr('Export…', 'Exporting…') : tr('Exporter', 'Export')} icon={Download} onClick={exportCsv} />
            <Btn label={tr('War Room', 'War Room')} icon={Activity} onClick={openWarRoom} />
            {canWrite && <Btn label={tr('Nouvel incident', 'New incident')} icon={Plus} primary onClick={() => setShowCreate(true)} />}
          </>
        }
      />

      {/* KPI header */}
      <div className="grid gap-3.5 mb-4" style={{ gridTemplateColumns: 'repeat(auto-fit,minmax(180px,1fr))' }}>
        {kpis.map((k, i) => (
          <Card key={k.label} style={{ padding: '16px 18px', animation: 'or-fadeup .4s ease both', animationDelay: `${i * 0.05}s` }}>
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-[11px] flex items-center justify-center shrink-0" style={{ background: softFill(k.color, 14), color: k.color }}>
                <k.icon size={19} strokeWidth={1.9} />
              </div>
              <div>
                <div className="disp mono text-[22px] font-bold text-ink leading-none">{k.value}</div>
                <div className="text-[12px] text-ink-muted mt-1">{k.label}</div>
              </div>
            </div>
          </Card>
        ))}
      </div>

      <DataTable
        id="incidents"
        ariaLabel={tr('Incidents', 'Incidents')}
        rows={visibleIncidents}
        columns={columns}
        rowKey={(inc) => String(inc.id)}
        api={table}
        mode="client"
        loading={isLoading}
        error={isError}
        onRetry={() => void refetch()}
        facets={facets}
        clientSearch={(inc, q) => `${inc.title} ${inc.description ?? ''} ${inc.reported_by ?? ''}`.toLowerCase().includes(q)}
        searchPlaceholder={tr('Titre, description ou déclarant…', 'Title, description or reporter…')}
        selectable={canWrite}
        rowActions={rowActions}
        bulkActions={bulkActions}
        onRowClick={(inc) => setSelected(inc)}
        exportFilename="incidents"
        minWidth={880}
        empty={
          <EmptyState
            icon={Siren}
            title={tr('Aucun incident', 'No incidents')}
            description={tr('Rien à signaler ici. Ouvrez un incident pour coordonner la réponse.', 'Nothing to report. Open an incident to coordinate the response.')}
            primaryAction={canWrite ? <Btn label={tr('Nouvel incident', 'New incident')} icon={Plus} primary onClick={() => setShowCreate(true)} /> : undefined}
          />
        }
      />

      {showCreate && <CreateIncidentModal onClose={() => setShowCreate(false)} />}
      {selected && <IncidentDrawer incident={selected} canWrite={canWrite} onClose={() => setSelected(null)} />}
    </PageFrame>
  );
}

/* ---------------- create dialog ---------------- */
function CreateIncidentModal({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const user = useAuthStore((s) => s.user);
  const { createIncident } = useIncidents();

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [type, setType] = useState('breach');
  const [severity, setSeverity] = useState<IncidentSeverity>('high');
  const [source, setSource] = useState('internal');
  const [error, setError] = useState('');

  const submit = () => {
    if (title.trim().length < 3) {
      setError(tr('Le titre doit comporter au moins 3 caractères.', 'Title must be at least 3 characters.'));
      return;
    }
    const input: CreateIncidentInput = {
      title: title.trim(),
      description: description.trim(),
      incident_type: type,
      severity,
      source,
      reported_by: user?.full_name || user?.email || 'unknown',
    };
    createIncident.mutate(input, {
      onSuccess: () => { toast.success(tr('Incident créé', 'Incident created')); onClose(); },
      onError: () => toast.error(tr('Création échouée', 'Creation failed')),
    });
  };

  const labelCls = 'text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted';
  const fieldCls = 'w-full h-10 px-3.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors';
  const fieldStyle = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' } as const;

  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)', animation: 'or-fadein .2s ease' }} onClick={onClose}>
      <form
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => { e.preventDefault(); submit(); }}
        className="w-full max-w-[480px] max-h-[90vh] flex flex-col rounded-[16px] overflow-hidden"
        style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-scalein .22s cubic-bezier(.2,.8,.2,1)' }}
      >
        <div className="px-[22px] pt-5 pb-4 flex items-center gap-3" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="w-9 h-9 rounded-[10px] flex items-center justify-center shrink-0" style={{ background: softFill('var(--critical)', 14), color: 'var(--critical)' }}><Siren size={18} /></div>
          <div className="disp text-[17px] font-bold text-ink flex-1">{tr('Nouvel incident', 'New incident')}</div>
          <button type="button" onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft hover:text-ink transition-colors" style={{ background: 'var(--bg-hover)' }} aria-label="Close"><X size={18} /></button>
        </div>

        <div className="flex-1 overflow-y-auto p-[22px] flex flex-col gap-4">
          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>{tr('Titre', 'Title')} <span style={{ color: 'var(--critical)' }}>*</span></span>
            <input autoFocus value={title} onChange={(e) => { setTitle(e.target.value); setError(''); }} placeholder={tr('ex. Exfiltration suspectée · srv-paie-01', 'e.g. Suspected exfiltration · payroll-srv-01')} className={fieldCls} style={{ ...fieldStyle, borderColor: error ? 'var(--critical)' : 'var(--border-strong)' }} />
            {error && <span className="text-[11.5px]" style={{ color: 'var(--critical)' }}>{error}</span>}
          </label>

          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>{tr('Description', 'Description')}</span>
            <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} placeholder={tr('Ce qui a été observé…', 'What was observed…')} className="w-full px-3.5 py-2.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors resize-none" style={fieldStyle} />
          </label>

          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1.5">
              <span className={labelCls}>{tr('Type', 'Type')}</span>
              <select value={type} onChange={(e) => setType(e.target.value)} className={fieldCls} style={fieldStyle}>
                {TYPES.map((t) => <option key={t} value={t} className="capitalize">{t.replace('_', ' ')}</option>)}
              </select>
            </label>
            <label className="flex flex-col gap-1.5">
              <span className={labelCls}>{tr('Sévérité', 'Severity')}</span>
              <select value={severity} onChange={(e) => setSeverity(e.target.value as IncidentSeverity)} className={fieldCls} style={fieldStyle}>
                {SEVERITIES.map((s) => <option key={s} value={s}>{tr(SEV[s].fr, SEV[s].en)}</option>)}
              </select>
            </label>
          </div>

          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>{tr('Source', 'Source')}</span>
            <select value={source} onChange={(e) => setSource(e.target.value)} className={fieldCls} style={fieldStyle}>
              <option value="internal">{tr('Interne', 'Internal')}</option>
              <option value="external">{tr('Externe', 'External')}</option>
              <option value="third_party">{tr('Tiers', 'Third party')}</option>
            </select>
          </label>
        </div>

        <div className="px-[22px] py-4 flex justify-end gap-2.5" style={{ borderTop: '1px solid var(--border)' }}>
          <button type="button" onClick={onClose} className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold text-ink-soft hover:text-ink transition-colors" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>{tr('Annuler', 'Cancel')}</button>
          <button type="submit" disabled={createIncident.isPending} className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-text-primary inline-flex items-center gap-1.5 transition-all disabled:opacity-60" style={{ border: 'none', background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 3px 12px var(--accent-glow)' }}>
            {createIncident.isPending && <Loader2 size={15} className="animate-spin" />}
            {tr('Créer', 'Create')}
          </button>
        </div>
      </form>
    </div>
  );
}
