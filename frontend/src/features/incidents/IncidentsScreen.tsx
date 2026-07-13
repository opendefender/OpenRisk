// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Incidents register (routed at /incidents) — the real, backend-wired incident
// hub: KPI header (from /incidents/stats), status filter chips, a table with
// inline status change + delete, and a create dialog. The fixture-only War Room
// console lives at /incidents/war-room (Preview) and is linked from the header.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { Siren, Plus, Trash2, Radio, ShieldAlert, CheckCircle2, Activity, X, Loader2 } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Chip, Card, SkeletonRows, EmptyState, softFill } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { relTime } from '../risks/riskMap';
import { useIncidents, useIncidentStats } from './useIncidents';
import type { Incident, IncidentSeverity, IncidentStatus, CreateIncidentInput } from './incidentService';

const SEV: Record<IncidentSeverity, { color: string; fr: string; en: string }> = {
  critical: { color: 'var(--critical)', fr: 'Critique', en: 'Critical' },
  high: { color: 'var(--high)', fr: 'Élevée', en: 'High' },
  medium: { color: 'var(--medium)', fr: 'Moyenne', en: 'Medium' },
  low: { color: 'var(--low)', fr: 'Faible', en: 'Low' },
};
const STATUS: Record<IncidentStatus, { color: string; fr: string; en: string }> = {
  open: { color: 'var(--critical)', fr: 'Ouvert', en: 'Open' },
  in_progress: { color: 'var(--high)', fr: 'En cours', en: 'In progress' },
  resolved: { color: 'var(--low)', fr: 'Résolu', en: 'Resolved' },
  closed: { color: 'var(--text-muted)', fr: 'Clos', en: 'Closed' },
};
const STATUSES: IncidentStatus[] = ['open', 'in_progress', 'resolved', 'closed'];
const SEVERITIES: IncidentSeverity[] = ['critical', 'high', 'medium', 'low'];
const TYPES = ['breach', 'attack', 'vulnerability', 'data_loss', 'phishing', 'malware', 'other'];

export function IncidentsScreen() {
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const hasRole = useAuthStore((s) => s.hasRole);
  const canWrite = hasRole('admin') || hasRole('analyst');

  const [filter, setFilter] = useState<'' | IncidentStatus>('');
  const [showCreate, setShowCreate] = useState(false);

  const { data: stats } = useIncidentStats();
  const { incidents, isLoading, updateIncident, deleteIncident } = useIncidents(
    filter ? { status: filter } : {}
  );

  const setStatus = (inc: Incident, status: IncidentStatus) => {
    if (status === inc.status) return;
    updateIncident.mutate(
      { id: inc.id, input: { status } },
      { onError: () => toast.error(tr('Mise à jour échouée', 'Update failed')) }
    );
  };

  const remove = (inc: Incident) => {
    if (!window.confirm(tr(`Supprimer l’incident « ${inc.title} » ?`, `Delete incident "${inc.title}"?`))) return;
    deleteIncident.mutate(inc.id, { onError: () => toast.error(tr('Suppression échouée', 'Delete failed')) });
  };

  const kpis = [
    { icon: Siren, label: tr('Total', 'Total'), value: stats?.total_incidents ?? 0, color: 'var(--accent)' },
    { icon: Radio, label: tr('Ouverts', 'Open'), value: stats?.open_incidents ?? 0, color: 'var(--critical)' },
    { icon: ShieldAlert, label: tr('Critiques', 'Critical'), value: stats?.critical_incidents ?? 0, color: 'var(--high)' },
    { icon: CheckCircle2, label: tr('Taux de résolution', 'Resolution rate'), value: `${Math.round(stats?.resolution_rate ?? 0)}%`, color: 'var(--low)' },
  ];

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Incidents', 'Incidents')}
        count={stats ? `${stats.open_incidents} ${tr('ouverts', 'open')}` : undefined}
        actions={
          <>
            <Btn label={tr('War Room', 'War Room')} icon={Activity} onClick={() => navigate('/incidents/war-room')} />
            {canWrite && <Btn label={tr('Nouvel incident', 'New incident')} icon={Plus} primary onClick={() => setShowCreate(true)} />}
          </>
        }
      />

      {/* KPI header */}
      <div className="grid gap-3.5 mb-4" style={{ gridTemplateColumns: 'repeat(auto-fit,minmax(180px,1fr))' }}>
        {kpis.map((k) => (
          <Card key={k.label} style={{ padding: '16px 18px' }}>
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

      {/* filters */}
      <div className="flex gap-2 mb-4 flex-wrap">
        <Chip label={tr('Tous', 'All')} active={filter === ''} onClick={() => setFilter('')} />
        {STATUSES.map((s) => (
          <Chip key={s} label={tr(STATUS[s].fr, STATUS[s].en)} active={filter === s} onClick={() => setFilter(s)} color={STATUS[s].color} />
        ))}
      </div>

      <Card style={{ padding: '8px 8px 4px', overflow: 'hidden' }}>
        {isLoading ? (
          <SkeletonRows rows={6} />
        ) : incidents.length === 0 ? (
          <EmptyState
            icon={Siren}
            title={tr('Aucun incident', 'No incidents')}
            sub={tr('Rien à signaler ici. Ouvrez un incident pour coordonner la réponse.', 'Nothing to report. Open an incident to coordinate the response.')}
            cta={canWrite ? <Btn label={tr('Nouvel incident', 'New incident')} icon={Plus} primary onClick={() => setShowCreate(true)} /> : undefined}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse" style={{ minWidth: 820 }}>
              <thead style={{ borderBottom: '1px solid var(--border)' }}>
                <tr>
                  {[tr('Incident', 'Incident'), tr('Type', 'Type'), tr('Sévérité', 'Severity'), tr('Statut', 'Status'), tr('Signalé', 'Reported'), ''].map((t, i) => (
                    <th key={i} className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{t}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {incidents.map((inc) => (
                  <tr key={inc.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td className="px-3 py-3">
                      <div className="text-[13.5px] font-medium text-ink">{inc.title}</div>
                      {inc.description && <div className="text-[12px] text-ink-muted mt-0.5 max-w-[420px] leading-snug truncate">{inc.description}</div>}
                    </td>
                    <td className="px-3 py-3 align-top"><span className="text-[12px] text-ink-soft capitalize">{inc.incident_type?.replace('_', ' ') || '—'}</span></td>
                    <td className="px-3 py-3 align-top">
                      <span className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: SEV[inc.severity]?.color, background: softFill(SEV[inc.severity]?.color ?? 'var(--text-muted)', 15) }}>
                        <span className="w-1.5 h-1.5 rounded-full" style={{ background: SEV[inc.severity]?.color }} />
                        {tr(SEV[inc.severity]?.fr ?? inc.severity, SEV[inc.severity]?.en ?? inc.severity)}
                      </span>
                    </td>
                    <td className="px-3 py-3 align-top">
                      <div className="relative inline-flex items-center">
                        <span className="w-2 h-2 rounded-full absolute left-2.5 pointer-events-none" style={{ background: STATUS[inc.status]?.color }} />
                        <select
                          value={inc.status}
                          disabled={!canWrite}
                          onChange={(e) => setStatus(inc, e.target.value as IncidentStatus)}
                          className="appearance-none text-[12px] font-semibold rounded-full pl-6 pr-6 py-1.5 outline-none disabled:opacity-70"
                          style={{ color: STATUS[inc.status]?.color, background: `color-mix(in srgb,${STATUS[inc.status]?.color} 12%,transparent)`, border: `1px solid color-mix(in srgb,${STATUS[inc.status]?.color} 30%,transparent)`, cursor: canWrite ? 'pointer' : 'not-allowed' }}
                        >
                          {STATUSES.map((s) => (
                            <option key={s} value={s} style={{ color: 'var(--text-primary)', background: 'var(--bg-elevated)' }}>{tr(STATUS[s].fr, STATUS[s].en)}</option>
                          ))}
                        </select>
                      </div>
                    </td>
                    <td className="px-3 py-3 align-top">
                      <div className="text-[12.5px] text-ink-soft">{relTime(inc.created_at, lang)}</div>
                      {inc.reported_by && <div className="text-[11.5px] text-ink-muted">{inc.reported_by}</div>}
                    </td>
                    <td className="px-3 py-3 align-top text-right">
                      {canWrite && (
                        <button onClick={() => remove(inc)} className="w-8 h-8 rounded-lg inline-flex items-center justify-center transition-colors hover:bg-hover" style={{ color: 'var(--critical)' }} title={tr('Supprimer', 'Delete')} aria-label={tr('Supprimer', 'Delete')}>
                          <Trash2 size={14} />
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {showCreate && <CreateIncidentModal onClose={() => setShowCreate(false)} />}
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
          <button type="submit" disabled={createIncident.isPending} className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5 transition-all disabled:opacity-60" style={{ border: 'none', background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 3px 12px var(--accent-glow)' }}>
            {createIncident.isPending && <Loader2 size={15} className="animate-spin" />}
            {tr('Créer', 'Create')}
          </button>
        </div>
      </form>
    </div>
  );
}
