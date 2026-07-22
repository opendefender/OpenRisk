// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Incident detail / edit drawer — slides in when a register row is clicked.
// View and modify the incident (title, description, severity, status, assignee,
// resolution), see its real timeline, jump into the War Room, or delete it.

import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { X, Save, Trash2, Activity, Clock, Loader2 } from 'lucide-react';
import { SkeletonRows } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { relTime } from '../risks/riskMap';
import { useIncidents, useIncidentTimeline } from './useIncidents';
import { SEV, STATUS, STATUSES, SEVERITIES, sevMeta, statusMeta } from './incidentMeta';
import type { Incident, IncidentSeverity, IncidentStatus, UpdateIncidentInput } from './incidentService';

export function IncidentDrawer({ incident, canWrite, onClose }: { incident: Incident; canWrite: boolean; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { updateIncident, deleteIncident } = useIncidents();
  const { data: timeline = [], isLoading: tlLoading } = useIncidentTimeline(incident.id);

  const [title, setTitle] = useState(incident.title);
  const [description, setDescription] = useState(incident.description ?? '');
  const [severity, setSeverity] = useState<IncidentSeverity>(incident.severity);
  const [status, setStatus] = useState<IncidentStatus>(incident.status);
  const [assignedTo, setAssignedTo] = useState(incident.assigned_to ?? '');
  const [resolution, setResolution] = useState(incident.resolution ?? '');

  const dirty = useMemo(
    () =>
      title !== incident.title ||
      description !== (incident.description ?? '') ||
      severity !== incident.severity ||
      status !== incident.status ||
      assignedTo !== (incident.assigned_to ?? '') ||
      resolution !== (incident.resolution ?? ''),
    [title, description, severity, status, assignedTo, resolution, incident]
  );

  const save = () => {
    const input: UpdateIncidentInput = {};
    if (title !== incident.title) input.title = title;
    if (description !== (incident.description ?? '')) input.description = description;
    if (severity !== incident.severity) input.severity = severity;
    if (status !== incident.status) input.status = status;
    if (assignedTo !== (incident.assigned_to ?? '')) input.assigned_to = assignedTo;
    if (resolution !== (incident.resolution ?? '')) input.resolution = resolution;
    updateIncident.mutate(
      { id: incident.id, input },
      {
        onSuccess: () => toast.success(tr('Incident mis à jour', 'Incident updated')),
        onError: () => toast.error(tr('Mise à jour échouée', 'Update failed')),
      }
    );
  };

  const remove = () => {
    if (!window.confirm(tr(`Supprimer l’incident « ${incident.title} » ?`, `Delete incident "${incident.title}"?`))) return;
    deleteIncident.mutate(incident.id, {
      onSuccess: () => { toast.success(tr('Incident supprimé', 'Incident deleted')); onClose(); },
      onError: () => toast.error(tr('Suppression échouée', 'Delete failed')),
    });
  };

  const labelCls = 'text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted';
  const fieldStyle = { border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' } as const;
  const inputCls = 'w-full h-10 px-3.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors disabled:opacity-70';

  return (
    <div
      className="fixed inset-0 z-[70] flex justify-end"
      style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)', animation: 'or-fadein .2s ease' }}
      onClick={onClose}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="h-full flex flex-col"
        style={{ width: 'min(94vw,540px)', background: 'var(--bg-secondary)', borderLeft: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-slidein .3s cubic-bezier(.2,.8,.2,1)' }}
      >
        {/* header */}
        <div className="px-[22px] pt-5 pb-4" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-start gap-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1.5">
                <span className="mono text-[12px] font-semibold text-ink-muted">INC-{incident.id}</span>
                <span className="inline-flex items-center gap-1.5 text-[11px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: sevMeta(severity).color, background: `color-mix(in srgb,${sevMeta(severity).color} 15%,transparent)` }}>
                  <span className="w-1.5 h-1.5 rounded-full" style={{ background: sevMeta(severity).color }} />
                  {tr(sevMeta(severity).fr, sevMeta(severity).en)}
                </span>
                <span className="inline-flex items-center gap-1.5 text-[11px] font-semibold px-[9px] py-[3px] rounded-full" style={{ color: statusMeta(status).color, background: `color-mix(in srgb,${statusMeta(status).color} 12%,transparent)` }}>
                  {tr(statusMeta(status).fr, statusMeta(status).en)}
                </span>
              </div>
              <div className="disp text-[16px] font-bold text-ink leading-snug">{incident.title}</div>
              <div className="text-[11.5px] text-ink-muted mt-1">{tr('Signalé par', 'Reported by')} {incident.reported_by || '—'} · {relTime(incident.created_at, lang)}</div>
            </div>
            <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft hover:text-ink transition-colors" style={{ background: 'var(--bg-hover)' }} aria-label="Close"><X size={18} /></button>
          </div>
        </div>

        {/* body */}
        <div className="flex-1 overflow-y-auto p-[22px] flex flex-col gap-4">
          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>{tr('Titre', 'Title')}</span>
            <input value={title} disabled={!canWrite} onChange={(e) => setTitle(e.target.value)} className={inputCls} style={fieldStyle} />
          </label>

          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>{tr('Description', 'Description')}</span>
            <textarea value={description} disabled={!canWrite} onChange={(e) => setDescription(e.target.value)} rows={3} className="w-full px-3.5 py-2.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors resize-none disabled:opacity-70" style={fieldStyle} />
          </label>

          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1.5">
              <span className={labelCls}>{tr('Sévérité', 'Severity')}</span>
              <select value={severity} disabled={!canWrite} onChange={(e) => setSeverity(e.target.value as IncidentSeverity)} className={inputCls} style={fieldStyle}>
                {SEVERITIES.map((s) => <option key={s} value={s}>{tr(SEV[s].fr, SEV[s].en)}</option>)}
              </select>
            </label>
            <label className="flex flex-col gap-1.5">
              <span className={labelCls}>{tr('Statut', 'Status')}</span>
              <select value={status} disabled={!canWrite} onChange={(e) => setStatus(e.target.value as IncidentStatus)} className={inputCls} style={fieldStyle}>
                {STATUSES.map((s) => <option key={s} value={s}>{tr(STATUS[s].fr, STATUS[s].en)}</option>)}
              </select>
            </label>
          </div>

          <label className="flex flex-col gap-1.5">
            <span className={labelCls}>{tr('Assigné à', 'Assigned to')}</span>
            <input value={assignedTo} disabled={!canWrite} onChange={(e) => setAssignedTo(e.target.value)} placeholder={tr('Responsable de la réponse', 'Response owner')} className={inputCls} style={fieldStyle} />
          </label>

          {(status === 'resolved' || status === 'closed' || resolution) && (
            <label className="flex flex-col gap-1.5">
              <span className={labelCls}>{tr('Résolution', 'Resolution')}</span>
              <textarea value={resolution} disabled={!canWrite} onChange={(e) => setResolution(e.target.value)} rows={2} placeholder={tr('Comment l’incident a été résolu…', 'How the incident was resolved…')} className="w-full px-3.5 py-2.5 rounded-[10px] text-[13px] text-ink outline-none focus:border-accent transition-colors resize-none disabled:opacity-70" style={fieldStyle} />
            </label>
          )}

          {/* timeline */}
          <div className="mt-1">
            <div className="flex items-center gap-1.5 mb-2.5"><Clock size={13} className="text-ink-muted" /><span className={labelCls}>{tr('Chronologie', 'Timeline')}</span></div>
            {tlLoading ? (
              <SkeletonRows rows={2} height={40} />
            ) : timeline.length === 0 ? (
              <div className="text-[12.5px] text-ink-muted">{tr('Aucun événement.', 'No events yet.')}</div>
            ) : (
              <div className="flex flex-col gap-0">
                {timeline.map((e, i) => (
                  <div key={e.id} className="flex gap-3">
                    <div className="flex flex-col items-center">
                      <span className="w-2 h-2 rounded-full mt-1.5" style={{ background: 'var(--accent)' }} />
                      {i < timeline.length - 1 && <span className="w-px flex-1" style={{ background: 'var(--border)' }} />}
                    </div>
                    <div className="pb-3.5 flex-1 min-w-0">
                      <div className="text-[12.5px] text-ink">{e.message}</div>
                      <div className="text-[11px] text-ink-muted mt-0.5">{e.created_by || 'system'} · {relTime(e.created_at, lang)}</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* footer */}
        <div className="px-[22px] py-4 flex items-center gap-2.5" style={{ borderTop: '1px solid var(--border)' }}>
          <button
            onClick={() => navigate(`/incidents/${incident.id}/war-room`)}
            className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold text-ink inline-flex items-center gap-1.5 hover:bg-hover transition-colors"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          >
            <Activity size={15} /> {tr('War Room', 'War Room')}
          </button>
          <div className="flex-1" />
          {canWrite && (
            <button onClick={remove} className="w-9 h-9 rounded-[10px] inline-flex items-center justify-center transition-all hover:brightness-110" style={{ border: '1px solid color-mix(in srgb,var(--critical) 30%,transparent)', background: 'color-mix(in srgb,var(--critical) 10%,transparent)', color: 'var(--critical)' }} title={tr('Supprimer', 'Delete')} aria-label={tr('Supprimer', 'Delete')}>
              <Trash2 size={15} />
            </button>
          )}
          {canWrite && (
            <button
              onClick={save}
              disabled={!dirty || updateIncident.isPending}
              className="h-9 px-4 rounded-[10px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5 transition-all disabled:opacity-50"
              style={{ border: 'none', background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))', boxShadow: '0 3px 12px var(--accent-glow)' }}
            >
              {updateIncident.isPending ? <Loader2 size={15} className="animate-spin" /> : <Save size={15} />}
              {tr('Enregistrer', 'Save')}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
