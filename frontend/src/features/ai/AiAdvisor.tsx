// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// IA Advisor (OpenRisk.dc.html §6.15): a centered conversational UI. AI bubbles
// (gradient sparkles avatar) vs. right-aligned user bubbles, suggested prompts on
// first load, and keyword-driven contextual replies.

import { useEffect, useRef, useState } from 'react';
import { Sparkles, Send } from 'lucide-react';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

interface AiMsg { role: 'ai' | 'user'; text: string }

export function AiAdvisor() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const [msgs, setMsgs] = useState<AiMsg[]>([
    { role: 'ai', text: tr('Bonjour Amir. Je suis votre conseiller sécurité OpenRisk. Je peux analyser vos risques, proposer des plans de mitigation ou préparer un rapport pour le COMEX. Que puis-je faire pour vous ?', 'Hello Amir. I am your OpenRisk security advisor. I can analyze your risks, propose mitigation plans or prepare a report for the board. How can I help?') },
  ]);
  const [input, setInput] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => { if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight; }, [msgs]);

  const reply = (q: string): string => {
    const s = q.toLowerCase();
    if (s.includes('critique') || s.includes('critical'))
      return tr('Vos 3 risques les plus critiques : (1) RSK-1042 — RDP exposé sur le serveur de paie (score 9.2), (2) RSK-1039 — absence de MFA sur les comptes cloud admin (8.6), (3) RSK-1031 — TLS obsolète sur la passerelle bancaire (6.8). Je recommande de traiter RSK-1042 en priorité : restreindre le RDP par liste blanche IP réduirait le score à ~4.1.', 'Your 3 most critical risks: (1) RSK-1042 — exposed RDP on the payroll server (score 9.2), (2) RSK-1039 — missing MFA on cloud admin accounts (8.6), (3) RSK-1031 — outdated TLS on the banking gateway (6.8). I recommend tackling RSK-1042 first: IP-allowlisting RDP would drop the score to ~4.1.');
    if (s.includes('paie') || s.includes('srv-paie') || s.includes('payroll'))
      return tr('Plan de mitigation proposé pour srv-paie-01 :\n1. Restreindre le port RDP (3389) à la liste blanche VPN.\n2. Activer la MFA sur tous les comptes administrateurs.\n3. Déployer la journalisation avancée + alerte SIEM sur les connexions.\nImpact estimé : score de risque 9.2 → 3.8. Souhaitez-vous que je crée ce plan dans Mitigations ?', 'Proposed mitigation plan for srv-paie-01:\n1. Restrict RDP (3389) to the VPN allowlist.\n2. Enable MFA on all admin accounts.\n3. Deploy advanced logging + SIEM alerting on connections.\nEstimated impact: risk score 9.2 → 3.8. Shall I create this plan in Mitigations?');
    if (s.includes('iso') || s.includes('conform') || s.includes('compli'))
      return tr('Posture ISO 27001 : 82 % de conformité (94/114 contrôles). Les écarts principaux concernent A.8 (gestion des actifs) et A.12 (sécurité d’exploitation). 12 contrôles doivent être traités avant l’audit du 18 sept. Je peux générer le rapport de conformité détaillé.', 'ISO 27001 posture: 82% compliance (94/114 controls). Main gaps are in A.8 (asset management) and A.12 (operations security). 12 controls must be addressed before the Sep 18 audit. I can generate the detailed compliance report.');
    return tr('J’ai analysé votre environnement. Votre score de sécurité global est de 72/100, en hausse de 3 points sur 7 jours. Le principal foyer de risque reste le serveur de paie. Voulez-vous que je détaille les risques critiques, prépare un plan de mitigation ou génère un rapport ?', 'I analyzed your environment. Your overall security score is 72/100, up 3 points over 7 days. The main risk hotspot remains the payroll server. Would you like me to detail the critical risks, prepare a mitigation plan, or generate a report?');
  };

  const send = (q?: string) => {
    const t = (q ?? input).trim(); if (!t) return;
    setMsgs((m) => [...m, { role: 'user', text: t }]);
    setInput('');
    setTimeout(() => setMsgs((m) => [...m, { role: 'ai', text: reply(t) }]), 400);
  };

  const sugg = [
    tr('Quels sont mes 3 risques les plus critiques ?', 'What are my 3 most critical risks?'),
    tr('Génère un plan pour srv-paie-01', 'Draft a plan for srv-paie-01'),
    tr('Résume ma conformité ISO 27001', 'Summarize my ISO 27001 compliance'),
  ];

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 58px)' }}>
      <div ref={scrollRef} className="flex-1 overflow-y-auto py-7">
        <div className="max-w-[740px] mx-auto px-6 flex flex-col gap-5">
          {msgs.map((x, i) =>
            x.role === 'ai' ? (
              <div key={i} className="flex gap-3.5">
                <div className="w-[34px] h-[34px] rounded-[10px] flex items-center justify-center text-white shrink-0" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))', boxShadow: '0 2px 10px var(--accent-glow)' }}><Sparkles size={18} /></div>
                <div className="text-[14px] leading-relaxed text-ink pt-1" style={{ whiteSpace: 'pre-line' }}>{x.text}</div>
              </div>
            ) : (
              <div key={i} className="self-end max-w-[80%] text-[14px] leading-relaxed text-ink px-[15px] py-[11px] rounded-[15px]" style={{ background: 'var(--accent-soft)', border: '1px solid var(--accent-line)' }}>{x.text}</div>
            )
          )}
        </div>
      </div>
      <div className="shrink-0 py-4" style={{ borderTop: '1px solid var(--border)' }}>
        <div className="max-w-[740px] mx-auto px-6">
          {msgs.length <= 1 && (
            <div className="flex gap-2 mb-3 flex-wrap">
              {sugg.map((sq) => (
                <button key={sq} onClick={() => send(sq)} className="text-[12.5px] font-medium px-3.5 py-2 rounded-full text-ink-soft hover:text-accent transition-colors" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>{sq}</button>
              ))}
            </div>
          )}
          <div className="flex gap-2.5 items-center">
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && send()}
              placeholder={tr('Posez une question à votre conseiller sécurité…', 'Ask your security advisor…')}
              className="flex-1 h-12 px-[18px] rounded-[14px] text-[14px] text-ink outline-none"
              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
            />
            <button onClick={() => send()} className="w-12 h-12 rounded-[14px] flex items-center justify-center text-white shrink-0" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}><Send size={20} /></button>
          </div>
          <div className="text-center text-[11px] text-ink-muted mt-2.5">{tr('L’IA peut se tromper. Vérifiez les décisions critiques.', 'AI can make mistakes. Verify critical decisions.')}</div>
        </div>
      </div>
    </div>
  );
}
