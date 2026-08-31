// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// War Room (OpenRisk.dc.html §6.5): full-screen console for a REAL incident
// (/incidents/:id/war-room). Top bar with its live elapsed timer, severity and
// status, a persisted Close action, the incident's real timeline, the people the
// record actually names, and a response task board backed by
// GET/POST/PUT /incidents/:id/actions.
//
// Two W0-05 corrections:
//
//  D6 — the composer at the foot of the thread appended to local state and sent
//  nothing anywhere. It carried an "(ephemeral)" placeholder, which vanishes the
//  moment you start typing; what remained was a message bubble indistinguishable
//  from a delivered one. In an incident console that is the worst place in the
//  product to fake a send: somebody writes "@alice take the DB offline", sees it
//  appear, and believes it was communicated. There is no messaging backend, so
//  the composer is gone rather than restyled.
//
//  D7 — the task board declared itself unavailable while a real, tenant-scoped
//  actions API had existed since the incident module shipped. It is now wired.
//
// Everything on this screen is real; the Preview badge is gone with the fixtures
// that justified it.

import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router';
import { toast } from 'sonner';
import { Check, Send, AlertTriangle, ArrowLeft } from 'lucide-react';
import { Avatar, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { relTime } from '../risks/riskMap';
import { useIncident, useIncidentTimeline, useIncidents, useIncidentActions } from './useIncidents';
import { sevMeta, statusMeta } from './incidentMeta';

function elapsedParts(fromISO?: string, toMs?: number): string {
  if (!fromISO) return '00:00:00';
  const start = new Date(fromISO).getTime();
  const end = toMs ?? Date.now();
  const total = Math.max(0, Math.floor((end - start) / 1000));
  const hh = String(Math.floor(total / 3600)).padStart(2, '0');
  const mm = String(Math.floor((total % 3600) / 60)).padStart(2, '0');
  const ss = String(total % 60).padStart(2, '0');
  return `${hh}:${mm}:${ss}`;
}

export function WarRoom() {
  const { id } = useParams<{ id: string }>();
  const incidentId = id ? Number(id) : undefined;
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const { data: incident, isLoading } = useIncident(incidentId);
  const { data: timeline = [] } = useIncidentTimeline(incidentId);
  const { updateIncident } = useIncidents();

  const [tick, setTick] = useState(0);
  const [confirmClose, setConfirmClose] = useState(false);
  const chatRef = useRef<HTMLDivElement>(null);

  // Response actions — the task board. Real, tenant-scoped, persisted.
  const { actions, isLoading: actionsLoading, isError: actionsError, create, setStatus, refetch: refetchActions } =
    useIncidentActions(incidentId);
  const [newAction, setNewAction] = useState('');
  const [newAssignee, setNewAssignee] = useState('');
  const [actionError, setActionError] = useState<string | null>(null);

  // Freeze the timer once the incident is resolved/closed.
  const frozenMs = incident?.resolved_at ? new Date(incident.resolved_at).getTime() : undefined;
  useEffect(() => {
    if (frozenMs) return;
    const t = setInterval(() => setTick((v) => v + 1), 1000);
    return () => clearInterval(t);
  }, [frozenMs]);
  useEffect(() => { if (chatRef.current) chatRef.current.scrollTop = chatRef.current.scrollHeight; }, [timeline]);
  // tick is only read through Date.now() in elapsedParts; reference it so the
  // re-render actually advances the clock.
  void tick;

  const addAction = async () => {
    const title = newAction.trim();
    if (!title) return;
    setActionError(null);
    try {
      // Awaited, and the list is refetched from the server on success: an action
      // that only exists in the browser is exactly what this screen must not
      // show during an incident.
      await create.mutateAsync({ title, assigned_to: newAssignee.trim() || undefined });
      setNewAction('');
      setNewAssignee('');
    } catch {
      setActionError(tr('Ajout impossible — réessayez.', 'Could not add — retry.'));
    }
  };

  const closeIncident = () => {
    if (!incidentId) return;
    updateIncident.mutate(
      { id: incidentId, input: { status: 'closed' } },
      {
        onSuccess: () => { toast.success(tr('Incident clos', 'Incident closed')); setConfirmClose(false); navigate('/incidents'); },
        onError: () => { toast.error(tr('Clôture échouée', 'Close failed')); setConfirmClose(false); },
      }
    );
  };

  if (isLoading) {
    return <div className="p-6"><SkeletonRows rows={6} height={56} /></div>;
  }
  if (!incident) {
    return (
      <div className="p-6">
        <button onClick={() => navigate('/incidents')} className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink-soft hover:text-ink transition-colors mb-4"><ArrowLeft size={15} /> {tr('Incidents', 'Incidents')}</button>
        <EmptyState variant="no-results" title={tr('Incident introuvable', 'Incident not found')} description={tr('Cet incident n’existe plus ou ne vous est pas accessible.', 'This incident no longer exists or is not accessible to you.')} />
      </div>
    );
  }

  const sev = sevMeta(incident.severity);
  const st = statusMeta(incident.status);
  const closed = incident.status === 'closed' || incident.status === 'resolved';

  // Responders are the two people the incident record actually names. There is no
  // participants table yet (see docs/ENDPOINTS.md "Planned"), so the roster shows
  // who is really on the record rather than a cast of invented responders — it
  // previously listed four hardcoded names on every incident in every tenant.
  const initialsOf = (v: string) =>
    v.split(/[\s@._-]+/).filter(Boolean).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('') || '?';
  const parts = [
    incident.assigned_to
      ? { name: incident.assigned_to, role: tr('Assigné', 'Assignee'), init: initialsOf(incident.assigned_to) }
      : null,
    incident.reported_by
      ? { name: incident.reported_by, role: tr('Déclarant', 'Reporter'), init: initialsOf(incident.reported_by) }
      : null,
  ].filter((x): x is { name: string; role: string; init: string } => x !== null);

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 58px)' }}>
      {/* top bar */}
      <div className="shrink-0 flex items-center gap-[18px] px-6 py-4 flex-wrap" style={{ borderBottom: '1px solid var(--border)', background: `linear-gradient(90deg,color-mix(in srgb,${sev.color} 7%,transparent),transparent 60%)` }}>
        <button onClick={() => navigate('/incidents')} className="w-9 h-9 rounded-[9px] flex items-center justify-center text-ink-soft hover:bg-hover hover:text-ink transition-colors shrink-0" aria-label={tr('Retour', 'Back')}><ArrowLeft size={18} /></button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2.5 mb-1 flex-wrap">
            <span className="inline-flex items-center gap-1.5 text-[10.5px] font-bold tracking-[.06em] uppercase px-[9px] py-[3px] rounded-full" style={{ color: st.color, background: `color-mix(in srgb,${st.color} 14%,transparent)` }}>
              {!closed && <span className="w-1.5 h-1.5 rounded-full" style={{ background: st.color, animation: 'or-pulsedot 1.4s infinite' }} />}
              {tr(st.fr, st.en)}
            </span>
            <span className="mono text-[12px] text-ink-muted">INC-{incident.id}</span>
          </div>
          <div className="disp text-[19px] font-bold text-ink truncate">{incident.title}</div>
        </div>
        <div className="text-center">
          <div className="disp mono text-[22px] font-bold text-ink leading-none">{elapsedParts(incident.created_at, frozenMs)}</div>
          <div className="text-[10.5px] text-ink-muted mt-[3px]">{tr('Durée', 'Elapsed')}</div>
        </div>
        <div className="text-center">
          <div className="text-[13px] font-bold" style={{ color: sev.color }}>{tr(sev.fr, sev.en)}</div>
          <div className="text-[10.5px] text-ink-muted mt-[3px]">{tr('Sévérité', 'Severity')}</div>
        </div>
        <div className="flex gap-2.5">
          {!closed && (
            <button onClick={() => setConfirmClose(true)} className="h-9 px-[15px] rounded-[10px] text-[13px] font-semibold text-fg-on-solid inline-flex items-center gap-1.5 transition-all hover:brightness-110" style={{ background: 'var(--danger-solid)' }}><Check size={16} /> {tr('Clore l’incident', 'Close incident')}</button>
          )}
        </div>
      </div>

      {/* body */}
      <div className="flex-1 flex min-h-0">
        {/* Responders — the people the incident record actually names. */}
        <div className="w-[240px] shrink-0 overflow-y-auto p-4 hidden md:block" style={{ borderRight: '1px solid var(--border)' }}>
          <div className="text-[11px] font-semibold uppercase tracking-[.05em] text-ink-muted mb-3.5">{tr('Participants', 'Responders')} · {parts.length}</div>
          {parts.length === 0 ? (
            <div className="text-[12px] text-ink-muted px-2 py-3 leading-relaxed">
              {tr('Personne n’est encore assigné à cet incident.', 'Nobody is assigned to this incident yet.')}
            </div>
          ) : parts.map((pp) => (
            <div key={pp.role} className="flex items-center gap-2.5 px-2 py-2 rounded-[10px] mb-0.5">
              <Avatar initials={pp.init} size={34} />
              <div className="flex-1 min-w-0">
                <div className="text-[13px] font-semibold text-ink truncate">{pp.name}</div>
                <div className="text-[11px]" style={{ color: 'var(--fg-muted)' }}>{pp.role}</div>
              </div>
            </div>
          ))}
        </div>

        {/* The incident's real timeline. Read-only: there is no messaging
            backend, and until W0-05 the composer below faked one. Actions taken
            during the incident are recorded on the task board, which persists. */}
        <div className="flex-1 min-w-0 flex flex-col">
          <div ref={chatRef} className="flex-1 overflow-y-auto px-6 py-5 flex flex-col gap-3.5">
            <div className="self-center inline-flex items-center gap-2 text-[11px] text-ink-muted px-3 py-1.5 rounded-full" style={{ background: 'var(--bg-hover)' }}>
              {tr('Chronologie de l’incident', 'Incident timeline')}
            </div>
            {timeline.map((e) => (
              <div key={e.id} className="self-center inline-flex items-center gap-2 text-[11.5px] text-ink-muted px-3 py-1.5 rounded-full" style={{ background: 'var(--bg-hover)' }}>
                <AlertTriangle size={13} /> {e.message} <span className="opacity-70">· {relTime(e.created_at, lang)}</span>
              </div>
            ))}
          </div>
          <div
            className="shrink-0 px-6 py-3.5 text-[12px] text-ink-muted leading-relaxed"
            style={{ borderTop: '1px solid var(--border)' }}
            data-testid="warroom-no-chat"
          >
            {tr(
              'La messagerie temps réel n’est pas disponible dans OpenRisk. Consignez les décisions dans les tâches de réponse — elles sont enregistrées et restent lisibles après l’incident.',
              'Real-time messaging is not available in OpenRisk. Record decisions as response tasks — they are persisted and stay readable after the incident.',
            )}
          </div>
        </div>

        {/* Response task board — real, persisted, tenant-scoped
            (GET/POST/PUT /incidents/:id/actions). */}
        <div className="w-[290px] shrink-0 overflow-y-auto p-4 hidden lg:block" style={{ borderLeft: '1px solid var(--border)' }}>
          <div className="text-[12px] font-semibold text-ink mb-2.5">
            {tr('Tâches de réponse', 'Response tasks')}
            {!actionsLoading && !actionsError && actions.length > 0 && (
              <span className="text-ink-muted font-normal"> · {actions.length}</span>
            )}
          </div>

          {!closed && (
            <div className="mb-4 flex flex-col gap-2">
              <input
                value={newAction}
                onChange={(e) => setNewAction(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') void addAction(); }}
                placeholder={tr('Nouvelle tâche…', 'New task…')}
                aria-label={tr('Titre de la tâche', 'Task title')}
                className="h-9 px-3 rounded-[10px] text-[13px] text-ink outline-none"
                style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
                data-testid="warroom-action-title"
              />
              <input
                value={newAssignee}
                onChange={(e) => setNewAssignee(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') void addAction(); }}
                placeholder={tr('Assignée à (facultatif)', 'Assignee (optional)')}
                aria-label={tr('Assignée à', 'Assignee')}
                className="h-9 px-3 rounded-[10px] text-[13px] text-ink outline-none"
                style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
              />
              <button
                onClick={() => void addAction()}
                disabled={!newAction.trim() || create.isPending}
                className="h-9 rounded-[10px] text-[12.5px] font-semibold text-fg-on-solid inline-flex items-center justify-center gap-1.5 disabled:opacity-50"
                style={{ background: 'var(--accent)' }}
                data-testid="warroom-action-add"
              >
                <Send size={14} /> {create.isPending ? tr('Ajout…', 'Adding…') : tr('Ajouter', 'Add')}
              </button>
              {actionError && (
                <div role="alert" className="text-[11.5px]" style={{ color: 'var(--critical)' }}>{actionError}</div>
              )}
            </div>
          )}

          {actionsLoading ? (
            <SkeletonRows rows={3} height={44} />
          ) : actionsError ? (
            // An error is not an empty board. During an incident, "no tasks" and
            // "we could not read the tasks" must never look the same.
            <div className="text-[12px] leading-relaxed" style={{ color: 'var(--critical)' }}>
              {tr('Lecture des tâches impossible.', 'Could not load the tasks.')}
              <button onClick={() => void refetchActions()} className="ml-1 underline">{tr('Réessayer', 'Retry')}</button>
            </div>
          ) : actions.length === 0 ? (
            <div className="text-[11.5px] text-ink-muted leading-relaxed">
              {tr('Aucune tâche de réponse pour le moment.', 'No response tasks yet.')}
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              {actions.map((a) => {
                const done = a.status === 'completed';
                const next: typeof a.status = done ? 'pending' : a.status === 'pending' ? 'in_progress' : 'completed';
                return (
                  <div key={a.id} className="rounded-[10px] px-3 py-2.5" style={{ border: '1px solid var(--border)' }}>
                    <div className="flex items-start gap-2">
                      <button
                        onClick={() => setStatus.mutate({ actionId: a.id, status: next })}
                        disabled={setStatus.isPending}
                        aria-label={tr('Changer le statut', 'Change status')}
                        className="mt-0.5 w-4 h-4 rounded-[5px] shrink-0 flex items-center justify-center disabled:opacity-50"
                        style={{
                          border: `1.5px solid ${done ? 'var(--low)' : 'var(--border-strong)'}`,
                          background: done ? 'var(--low)' : 'transparent',
                        }}
                        data-testid={`warroom-action-toggle-${a.id}`}
                      >
                        {done && <Check size={11} className="text-fg-on-solid" />}
                      </button>
                      <div className="flex-1 min-w-0">
                        <div
                          className="text-[12.5px] text-ink leading-snug"
                          style={{ textDecoration: done ? 'line-through' : 'none', opacity: done ? 0.6 : 1 }}
                        >
                          {a.title}
                        </div>
                        <div className="text-[11px] text-ink-muted mt-0.5">
                          {a.assigned_to || tr('Non assignée', 'Unassigned')}
                          {' · '}
                          {a.status === 'in_progress' ? tr('en cours', 'in progress') : done ? tr('terminée', 'done') : tr('à faire', 'to do')}
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {confirmClose && (
        <div className="fixed inset-0 z-90 flex items-center justify-center" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(6px)', animation: 'or-fadein .16s ease' }} onClick={() => setConfirmClose(false)}>
          <div onClick={(e) => e.stopPropagation()} className="glass-strong rounded-[18px] shadow-card-lg p-[26px]" style={{ width: 'min(90vw,420px)', animation: 'or-scalein .18s ease' }}>
            <div className="w-[46px] h-[46px] rounded-[13px] flex items-center justify-center mb-4" style={{ background: 'color-mix(in srgb,var(--critical) 14%,transparent)', color: 'var(--critical)' }}><AlertTriangle size={24} /></div>
            <div className="disp text-[18px] font-bold text-ink mb-2">{tr('Clore cet incident ?', 'Close this incident?')}</div>
            <div className="text-[13.5px] text-ink-soft leading-relaxed mb-[22px]">{tr('Le statut passera à « Clos » et l’incident sortira des incidents actifs.', 'The status will be set to "Closed" and it will leave the active incidents.')}</div>
            <div className="flex gap-2.5">
              <button onClick={() => setConfirmClose(false)} className="flex-1 h-[42px] rounded-[11px] text-[13.5px] font-semibold text-ink" style={{ border: '1px solid var(--border-strong)' }}>{tr('Annuler', 'Cancel')}</button>
              <button onClick={closeIncident} disabled={updateIncident.isPending} className="flex-1 h-[42px] rounded-[11px] text-[13.5px] font-semibold text-fg-on-solid disabled:opacity-60" style={{ background: 'var(--danger-solid)' }}>{tr('Clore', 'Close')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
