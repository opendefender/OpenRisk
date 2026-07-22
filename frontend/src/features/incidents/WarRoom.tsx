// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// War Room (OpenRisk.dc.html §6.5): full-screen incident console for reviewing a
// REAL incident (/incidents/:id/war-room) — top bar with its live elapsed timer,
// severity/status, and a Close action (persisted), a chat thread seeded from the
// incident's real timeline, plus a responder roster and task board. The roster,
// task board and chat input are still fixtures (no collaboration backend yet) —
// hence the Aperçu/Preview badge; the incident context + timeline + close are real.

import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { toast } from 'sonner';
import { Check, Plus, Send, AlertTriangle, ArrowLeft } from 'lucide-react';
import { Avatar, PreviewBadge, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { relTime } from '../risks/riskMap';
import { useIncident, useIncidentTimeline, useIncidents } from './useIncidents';
import { sevMeta, statusMeta } from './incidentMeta';

interface Note { who: string; name: string; text: string; time: string }

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
  const [notes, setNotes] = useState<Note[]>([]);
  const [input, setInput] = useState('');
  const [confirmClose, setConfirmClose] = useState(false);
  const chatRef = useRef<HTMLDivElement>(null);

  // Freeze the timer once the incident is resolved/closed.
  const frozenMs = incident?.resolved_at ? new Date(incident.resolved_at).getTime() : undefined;
  useEffect(() => {
    if (frozenMs) return;
    const t = setInterval(() => setTick((v) => v + 1), 1000);
    return () => clearInterval(t);
  }, [frozenMs]);
  useEffect(() => { if (chatRef.current) chatRef.current.scrollTop = chatRef.current.scrollHeight; }, [timeline, notes]);
  // tick is only read through Date.now() in elapsedParts; reference it so the
  // re-render actually advances the clock.
  void tick;

  const send = () => {
    const t = input.trim(); if (!t) return;
    const d = new Date();
    const time = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
    setNotes((m) => [...m, { who: 'ME', name: tr('Vous', 'You'), text: t, time }]);
    setInput('');
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

  const mention = (text: string) =>
    text.split(/(\s+)/).map((w, i) => (w.startsWith('@') ? <span key={i} className="font-semibold" style={{ color: 'var(--accent)' }}>{w}</span> : w));

  // Responder roster + task board remain design fixtures (no collaboration backend).
  const parts: [string, string, string, boolean, boolean][] = [
    ['Amir Diallo', tr('RSSI · Commandant', 'CISO · Commander'), 'AD', true, false],
    ['Fatou Sy', tr('Analyste SOC', 'SOC Analyst'), 'FS', true, true],
    ['Kofi Mensah', tr('Ingénieur infra', 'Infra Engineer'), 'KM', true, false],
    ['Léa Traoré', tr('Forensic', 'Forensics'), 'LT', false, false],
  ];
  const taskCols: [string, string, [string, string, string, boolean][]][] = [
    [tr('Urgent', 'Urgent'), 'var(--critical)', [[tr('Isoler le système affecté', 'Isolate affected system'), 'KM', tr('en cours', 'ongoing'), false]]],
    [tr('En cours', 'In progress'), 'var(--high)', [[tr('Analyse forensic', 'Forensic analysis'), 'LT', '—', false]]],
    [tr('Résolu', 'Resolved'), 'var(--low)', []],
  ];

  if (isLoading) {
    return <div className="p-6"><SkeletonRows rows={6} height={56} /></div>;
  }
  if (!incident) {
    return (
      <div className="p-6">
        <button onClick={() => navigate('/incidents')} className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink-soft hover:text-ink transition-colors mb-4"><ArrowLeft size={15} /> {tr('Incidents', 'Incidents')}</button>
        <EmptyState icon={AlertTriangle} title={tr('Incident introuvable', 'Incident not found')} />
      </div>
    );
  }

  const sev = sevMeta(incident.severity);
  const st = statusMeta(incident.status);
  const closed = incident.status === 'closed' || incident.status === 'resolved';

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
            <PreviewBadge label={tr('Aperçu', 'Preview')} />
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
            <button onClick={() => setConfirmClose(true)} className="h-9 px-[15px] rounded-[10px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5 transition-all hover:brightness-110" style={{ background: 'var(--critical)' }}><Check size={16} /> {tr('Clore l’incident', 'Close incident')}</button>
          )}
        </div>
      </div>

      {/* body */}
      <div className="flex-1 flex min-h-0">
        {/* roster (fixture) */}
        <div className="w-[240px] shrink-0 overflow-y-auto p-4 hidden md:block" style={{ borderRight: '1px solid var(--border)' }}>
          <div className="text-[11px] font-semibold uppercase tracking-[.05em] text-ink-muted mb-3.5">{tr('Participants', 'Responders')} · {parts.length}</div>
          {parts.map(([name, role, init, online, typing]) => (
            <div key={init} className="flex items-center gap-2.5 px-2 py-2 rounded-[10px] mb-0.5">
              <div className="relative shrink-0">
                <Avatar initials={init} size={34} />
                <span className="absolute bottom-0 right-0 w-2.5 h-2.5 rounded-full" style={{ background: online ? 'var(--low)' : 'var(--text-muted)', border: '2px solid var(--bg-primary)' }} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-[13px] font-semibold text-ink truncate">{name}</div>
                <div className="text-[11px]" style={{ color: typing ? 'var(--accent)' : 'var(--text-muted)' }}>{typing ? tr('écrit…', 'typing…') : role}</div>
              </div>
            </div>
          ))}
        </div>

        {/* chat: real timeline + ephemeral notes */}
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
            {notes.map((x, i) => (
              <div key={i} className="flex gap-2.5 max-w-[80%] self-end flex-row-reverse">
                <Avatar initials={x.who} size={30} />
                <div>
                  <div className="flex items-baseline gap-2 mb-1 justify-end">
                    <span className="text-[12px] font-semibold text-ink">{x.name}</span>
                    <span className="text-[10.5px] text-ink-muted">{x.time}</span>
                  </div>
                  <div className="text-[13.5px] leading-relaxed text-ink px-3 py-2 rounded-[13px]" style={{ background: 'var(--accent-soft)', border: '1px solid var(--accent-line)' }}>
                    {mention(x.text)}
                  </div>
                </div>
              </div>
            ))}
          </div>
          <div className="shrink-0 px-6 py-3.5 flex gap-2.5 items-center" style={{ borderTop: '1px solid var(--border)' }}>
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && send()}
              placeholder={tr('Note de coordination… (éphémère)', 'Coordination note… (ephemeral)')}
              className="flex-1 h-11 px-4 rounded-xl text-[14px] text-ink outline-none"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
            />
            <button onClick={send} className="w-11 h-11 rounded-xl flex items-center justify-center text-white shrink-0" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}><Send size={19} /></button>
          </div>
        </div>

        {/* tasks (fixture) */}
        <div className="w-[290px] shrink-0 overflow-y-auto p-4 hidden lg:block" style={{ borderLeft: '1px solid var(--border)' }}>
          <button className="w-full h-9 rounded-[10px] text-[12.5px] font-semibold inline-flex items-center justify-center gap-1.5 mb-4" style={{ border: '1px solid var(--critical)', background: 'color-mix(in srgb,var(--critical) 8%,transparent)', color: 'var(--critical)' }}><Plus size={15} /> {tr('Tâche urgente', 'Urgent task')}</button>
          {taskCols.map(([lbl, col, tasks]) => (
            <div key={lbl} className="mb-4">
              <div className="flex items-center gap-1.5 mb-2.5">
                <span className="w-[7px] h-[7px] rounded-full" style={{ background: col }} />
                <span className="text-[12px] font-semibold text-ink">{lbl}</span>
                <span className="text-[11px] text-ink-muted">{tasks.length}</span>
              </div>
              {tasks.map((t2, i) => (
                <div key={i} className="rounded-[10px] px-3 py-2.5 mb-2" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>
                  <div className="text-[12.5px] font-medium text-ink mb-2 leading-snug" style={{ textDecoration: t2[3] ? 'line-through' : 'none', opacity: t2[3] ? 0.6 : 1 }}>{t2[0]}</div>
                  <div className="flex items-center justify-between">
                    <Avatar initials={t2[1]} size={22} />
                    <span className="text-[11px] text-ink-muted">{t2[2]}</span>
                  </div>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>

      {confirmClose && (
        <div className="fixed inset-0 z-[90] flex items-center justify-center" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(6px)', animation: 'or-fadein .16s ease' }} onClick={() => setConfirmClose(false)}>
          <div onClick={(e) => e.stopPropagation()} className="glass-strong rounded-[18px] shadow-card-lg p-[26px]" style={{ width: 'min(90vw,420px)', animation: 'or-scalein .18s ease' }}>
            <div className="w-[46px] h-[46px] rounded-[13px] flex items-center justify-center mb-4" style={{ background: 'color-mix(in srgb,var(--critical) 14%,transparent)', color: 'var(--critical)' }}><AlertTriangle size={24} /></div>
            <div className="disp text-[18px] font-bold text-ink mb-2">{tr('Clore cet incident ?', 'Close this incident?')}</div>
            <div className="text-[13.5px] text-ink-soft leading-relaxed mb-[22px]">{tr('Le statut passera à « Clos » et l’incident sortira des incidents actifs.', 'The status will be set to "Closed" and it will leave the active incidents.')}</div>
            <div className="flex gap-2.5">
              <button onClick={() => setConfirmClose(false)} className="flex-1 h-[42px] rounded-[11px] text-[13.5px] font-semibold text-ink" style={{ border: '1px solid var(--border-strong)' }}>{tr('Annuler', 'Cancel')}</button>
              <button onClick={closeIncident} disabled={updateIncident.isPending} className="flex-1 h-[42px] rounded-[11px] text-[13.5px] font-semibold text-white disabled:opacity-60" style={{ background: 'var(--critical)' }}>{tr('Clore', 'Close')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
