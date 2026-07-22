// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Emerging-risk detection (spec §12.2): paste a threat-intel report, a news
// snippet or logs, and the AI extracts candidate new risks (title, severity,
// suggested probability/impact). Wired to POST /ai/emerging-risks; the tenant's
// existing risk titles are de-duplicated server-side. Replaces the old fully
// mocked AIRiskInsights page.

import { useState } from 'react';
import { Sparkles, Loader2, AlertTriangle } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useDetectEmergingRisks } from './useAi';
import type { EmergingRisk } from './aiService';

const SEV_COLOR: Record<string, string> = {
  critical: 'var(--critical)',
  high: 'var(--high)',
  medium: 'var(--medium, #d19a24)',
  low: 'var(--accent)',
};

export function EmergingRisksPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const locale = lang === 'fr' ? 'fr' : 'en';

  const [source, setSource] = useState('threat-intel');
  const [text, setText] = useState('');
  const detect = useDetectEmergingRisks();
  const res = detect.data?.result;

  const sources = [
    ['threat-intel', tr('Threat intelligence', 'Threat intelligence')],
    ['news', tr('Actualité', 'News')],
    ['logs', tr('Logs', 'Logs')],
  ];

  const analyze = () => {
    if (!text.trim() || detect.isPending) return;
    detect.mutate({ source, text, locale });
  };

  return (
    <div className="max-w-[900px] mx-auto px-6 py-7">
      <div className="flex items-center gap-3 mb-1.5">
        <div className="w-9 h-9 rounded-[11px] flex items-center justify-center text-white" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))' }}>
          <Sparkles size={19} />
        </div>
        <h1 className="disp text-[22px] font-bold text-ink">{tr('Détection de risques émergents', 'Emerging risk detection')}</h1>
      </div>
      <p className="text-[13px] text-ink-soft mb-6 leading-relaxed">
        {tr(
          "Collez un rapport de threat intelligence, une actualité ou des logs. L'IA identifie les risques émergents et propose de nouveaux risques à ajouter à votre registre (les risques déjà suivis sont ignorés).",
          'Paste a threat-intelligence report, a news item or logs. The AI identifies emerging risks and proposes new risks to add to your register (already-tracked risks are skipped).',
        )}
      </p>

      <div className="p-4 rounded-[14px] mb-5" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}>
        <div className="flex gap-2 mb-3 flex-wrap">
          {sources.map(([k, lbl]) => (
            <button
              key={k}
              onClick={() => setSource(k)}
              className="text-[12.5px] font-medium px-3 py-1.5 rounded-full transition-colors"
              style={{
                border: `1px solid ${source === k ? 'var(--accent-line)' : 'var(--border-strong)'}`,
                background: source === k ? 'var(--accent-soft)' : 'var(--bg-elevated)',
                color: source === k ? 'var(--accent)' : 'var(--ink-soft)',
              }}
            >
              {lbl}
            </button>
          ))}
        </div>
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={7}
          placeholder={tr(
            'Ex : Une nouvelle souche de rançongiciel se propage via des emails de phishing exploitant CVE-2024-3400…',
            'E.g. A new ransomware strain is spreading via phishing emails exploiting CVE-2024-3400…',
          )}
          className="w-full p-3 rounded-[11px] text-[13.5px] text-ink outline-none resize-y"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
        />
        <div className="flex items-center justify-between mt-3">
          <span className="text-[11.5px] text-ink-muted">{text.length} {tr('caractères', 'characters')}</span>
          <button
            onClick={analyze}
            disabled={detect.isPending || !text.trim()}
            className="h-10 px-5 rounded-[11px] flex items-center gap-2 text-white text-[13px] font-semibold disabled:opacity-60"
            style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-hover))' }}
          >
            {detect.isPending ? <Loader2 size={16} className="animate-spin" /> : <Sparkles size={16} />}
            {detect.isPending ? tr('Analyse…', 'Analysing…') : tr('Analyser avec l’IA', 'Analyse with AI')}
          </button>
        </div>
      </div>

      {detect.isError && (
        <div className="text-[13px]" style={{ color: 'var(--critical)' }}>{tr('L’analyse a échoué. Réessayez.', 'Analysis failed. Please try again.')}</div>
      )}

      {res && (
        <div style={{ animation: 'or-fadeup .25s ease' }}>
          <div className="flex items-center justify-between mb-3">
            <div className="text-[13.5px] font-semibold text-ink">{res.summary}</div>
            <span className="text-[11px] text-ink-muted">{tr('Généré par', 'Generated by')} {detect.data?.generated_by}</span>
          </div>
          {res.risks.length === 0 ? (
            <div className="text-center py-10 text-[13px] text-ink-soft">{tr('Aucun risque émergent détecté.', 'No emerging risk detected.')}</div>
          ) : (
            <div className="grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))' }}>
              {res.risks.map((rk: EmergingRisk, i: number) => (
                <div key={i} className="p-4 rounded-[13px]" style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}>
                  <div className="flex items-start gap-2 mb-2">
                    <AlertTriangle size={16} style={{ color: SEV_COLOR[rk.severity] ?? 'var(--accent)' }} className="shrink-0 mt-0.5" />
                    <div className="text-[14px] font-semibold text-ink leading-snug flex-1">{rk.title}</div>
                  </div>
                  <div className="text-[12.5px] text-ink-soft leading-relaxed mb-3">{rk.description}</div>
                  <div className="flex items-center gap-2 flex-wrap text-[11px]">
                    <span className="px-2 py-0.5 rounded-full font-semibold" style={{ background: 'var(--bg-elevated)', color: SEV_COLOR[rk.severity] ?? 'var(--accent)', border: '1px solid var(--border)' }}>{rk.severity}</span>
                    <span className="px-2 py-0.5 rounded-full text-ink-soft" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}>{rk.category}</span>
                    <span className="text-ink-muted ml-auto">P {(rk.suggested_probability * 100).toFixed(0)}% · I {rk.suggested_impact.toFixed(1)}</span>
                  </div>
                  {rk.rationale && <div className="text-[11.5px] text-ink-muted italic mt-2 leading-relaxed">{rk.rationale}</div>}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default EmergingRisksPage;
