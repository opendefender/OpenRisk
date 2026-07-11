// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// War Room (OpenRisk.dc.html §6.5): full-screen incident console — top bar with a
// live elapsed timer, responder roster (online dots / typing), a chat thread with
// @mentions and system events, a task board, and a close-incident confirm dialog.

import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileText, Check, Plus, Send, AlertTriangle } from 'lucide-react';
import { Avatar, PreviewBadge } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

interface Msg { sys?: boolean; who?: string; name?: string; text: string; time: string }

export function WarRoom() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const [tick, setTick] = useState(0);
  const [msgs, setMsgs] = useState<Msg[]>([
    { sys: true, text: tr('Incident ouvert · srv-paie-01 · exfiltration suspectée', 'Incident opened · srv-paie-01 · suspected exfiltration'), time: '14:02' },
    { who: 'FS', name: 'Fatou Sy', text: tr('Trafic sortant anormal vers 185.220.x.x depuis srv-paie-01', 'Abnormal outbound traffic to 185.220.x.x from srv-paie-01'), time: '14:03' },
    { who: 'AD', name: 'Amir Diallo', text: tr('@Kofi isole le serveur du réseau immédiatement', '@Kofi isolate the server from the network immediately'), time: '14:05' },
    { sys: true, text: tr('srv-paie-01 isolé du réseau · règle firewall appliquée', 'srv-paie-01 isolated · firewall rule applied'), time: '14:07' },
    { who: 'KM', name: 'Kofi Mensah', text: tr('Isolation confirmée. Snapshot mémoire en cours pour le forensic.', 'Isolation confirmed. Memory snapshot in progress for forensics.'), time: '14:09' },
  ]);
  const [input, setInput] = useState('');
  const [confirmClose, setConfirmClose] = useState(false);
  const chatRef = useRef<HTMLDivElement>(null);

  useEffect(() => { const t = setInterval(() => setTick((v) => v + 1), 1000); return () => clearInterval(t); }, []);
  useEffect(() => { if (chatRef.current) chatRef.current.scrollTop = chatRef.current.scrollHeight; }, [msgs]);

  const total = 6432 + tick;
  const hh = String(Math.floor(total / 3600)).padStart(2, '0');
  const mm = String(Math.floor((total % 3600) / 60)).padStart(2, '0');
  const ss = String(total % 60).padStart(2, '0');

  const send = () => {
    const t = input.trim(); if (!t) return;
    const d = new Date();
    const time = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
    setMsgs((m) => [...m, { who: 'AD', name: 'Amir Diallo', text: t, time }]);
    setInput('');
  };

  const mention = (text: string) =>
    text.split(/(\s+)/).map((w, i) => (w.startsWith('@') ? <span key={i} className="font-semibold" style={{ color: 'var(--accent)' }}>{w}</span> : w));

  const parts: [string, string, string, boolean, boolean][] = [
    ['Amir Diallo', tr('RSSI · Commandant', 'CISO · Commander'), 'AD', true, false],
    ['Fatou Sy', tr('Analyste SOC', 'SOC Analyst'), 'FS', true, true],
    ['Kofi Mensah', tr('Ingénieur infra', 'Infra Engineer'), 'KM', true, false],
    ['Léa Traoré', tr('Forensic', 'Forensics'), 'LT', false, false],
  ];
  const taskCols: [string, string, [string, string, string, boolean][]][] = [
    [tr('Urgent', 'Urgent'), 'var(--critical)', [[tr('Isoler srv-paie-01', 'Isolate srv-paie-01'), 'KM', tr('fait', 'done'), true], [tr('Réinitialiser les accès IAM', 'Reset IAM access'), 'FS', tr('en cours', 'ongoing'), false]]],
    [tr('En cours', 'In progress'), 'var(--high)', [[tr('Analyse forensic du snapshot', 'Forensic snapshot analysis'), 'LT', '—', false], [tr('Revue des logs firewall', 'Firewall log review'), 'AD', '—', false]]],
    [tr('Résolu', 'Resolved'), 'var(--low)', [[tr('Blocage IP sortante', 'Block outbound IP'), 'KM', '14:07', true]]],
  ];

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 58px)' }}>
      {/* top bar */}
      <div className="shrink-0 flex items-center gap-[18px] px-6 py-4 flex-wrap" style={{ borderBottom: '1px solid var(--border)', background: 'linear-gradient(90deg,color-mix(in srgb,var(--critical) 7%,transparent),transparent 60%)' }}>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2.5 mb-1">
            <span className="inline-flex items-center gap-1.5 text-[10.5px] font-bold tracking-[.06em] uppercase px-[9px] py-[3px] rounded-full" style={{ color: 'var(--critical)', background: 'color-mix(in srgb,var(--critical) 14%,transparent)' }}>
              <span className="w-1.5 h-1.5 rounded-full" style={{ background: 'var(--critical)', animation: 'or-pulsedot 1.4s infinite' }} />{tr('En cours', 'Active')}
            </span>
            <span className="mono text-[12px] text-ink-muted">INC-2026-014</span>
            <PreviewBadge label={tr('Aperçu', 'Preview')} />
          </div>
          <div className="disp text-[19px] font-bold text-ink">{tr('Exfiltration suspectée · serveur de paie', 'Suspected exfiltration · payroll server')}</div>
        </div>
        <div className="text-center">
          <div className="disp mono text-[22px] font-bold text-ink leading-none">{hh}:{mm}:{ss}</div>
          <div className="text-[10.5px] text-ink-muted mt-[3px]">{tr('Durée', 'Elapsed')}</div>
        </div>
        <div className="text-center">
          <div className="text-[13px] font-bold" style={{ color: 'var(--critical)' }}>{tr('Critique', 'Critical')}</div>
          <div className="text-[10.5px] text-ink-muted mt-[3px]">{tr('Sévérité', 'Severity')}</div>
        </div>
        <div className="flex gap-2.5">
          <button className="h-9 px-[15px] rounded-[10px] text-[13px] font-semibold text-ink inline-flex items-center gap-1.5 hover:bg-hover" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}><FileText size={16} /> {L.exportPdf}</button>
          <button onClick={() => setConfirmClose(true)} className="h-9 px-[15px] rounded-[10px] text-[13px] font-semibold text-white inline-flex items-center gap-1.5" style={{ background: 'var(--critical)' }}><Check size={16} /> {tr('Clore l’incident', 'Close incident')}</button>
        </div>
      </div>

      {/* body */}
      <div className="flex-1 flex min-h-0">
        {/* roster */}
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

        {/* chat */}
        <div className="flex-1 min-w-0 flex flex-col">
          <div ref={chatRef} className="flex-1 overflow-y-auto px-6 py-5 flex flex-col gap-3.5">
            {msgs.map((x, i) =>
              x.sys ? (
                <div key={i} className="self-center inline-flex items-center gap-2 text-[11.5px] text-ink-muted px-3 py-1.5 rounded-full" style={{ background: 'var(--bg-hover)' }}>
                  <AlertTriangle size={13} /> {x.text} <span className="opacity-70">· {x.time}</span>
                </div>
              ) : (
                <div key={i} className="flex gap-2.5 max-w-[80%]" style={{ alignSelf: x.who === 'AD' ? 'flex-end' : 'flex-start', flexDirection: x.who === 'AD' ? 'row-reverse' : 'row' }}>
                  <Avatar initials={x.who!} size={30} />
                  <div>
                    <div className="flex items-baseline gap-2 mb-1" style={{ justifyContent: x.who === 'AD' ? 'flex-end' : 'flex-start' }}>
                      <span className="text-[12px] font-semibold text-ink">{x.name}</span>
                      <span className="text-[10.5px] text-ink-muted">{x.time}</span>
                    </div>
                    <div className="text-[13.5px] leading-relaxed text-ink px-3 py-2 rounded-[13px]" style={{ background: x.who === 'AD' ? 'var(--accent-soft)' : 'var(--bg-elevated)', border: `1px solid ${x.who === 'AD' ? 'var(--accent-line)' : 'var(--border)'}` }}>
                      {mention(x.text)}
                    </div>
                  </div>
                </div>
              )
            )}
          </div>
          <div className="shrink-0 px-6 py-3.5 flex gap-2.5 items-center" style={{ borderTop: '1px solid var(--border)' }}>
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && send()}
              placeholder={tr('Message à l’équipe…  (@ pour mentionner)', 'Message the team…  (@ to mention)')}
              className="flex-1 h-11 px-4 rounded-xl text-[14px] text-ink outline-none"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
            />
            <button onClick={send} className="w-11 h-11 rounded-xl flex items-center justify-center text-white shrink-0" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}><Send size={19} /></button>
          </div>
        </div>

        {/* tasks */}
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
            <div className="text-[13.5px] text-ink-soft leading-relaxed mb-[22px]">{tr('La timeline sera archivée et un post-mortem sera généré automatiquement. Cette action est définitive.', 'The timeline will be archived and a post-mortem generated automatically. This action is final.')}</div>
            <div className="flex gap-2.5">
              <button onClick={() => setConfirmClose(false)} className="flex-1 h-[42px] rounded-[11px] text-[13.5px] font-semibold text-ink" style={{ border: '1px solid var(--border-strong)' }}>{tr('Annuler', 'Cancel')}</button>
              <button onClick={() => { setConfirmClose(false); navigate('/'); }} className="flex-1 h-[42px] rounded-[11px] text-[13.5px] font-semibold text-white" style={{ background: 'var(--critical)' }}>{tr('Clore', 'Close')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
