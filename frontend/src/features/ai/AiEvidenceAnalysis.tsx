// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Inline AI evidence analysis (spec §12.5): a compact "Analyser (IA)" control
// under an uploaded evidence that checks, via POST /ai/evidence/:id/analyze,
// whether the proof meets the intent of its control. Renders a coloured verdict
// badge (documentary-compliance status) + rationale/gaps/suggestions — the
// "indicateur visuel du statut d'analyse IA" the spec asks for.

import { Sparkles, Loader2 } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useAnalyzeEvidence } from './useAi';

const VERDICT: Record<string, { fr: string; en: string; color: string }> = {
  satisfies: { fr: 'Satisfait', en: 'Satisfies', color: 'var(--success, #1f9d55)' },
  partial: { fr: 'Partiel', en: 'Partial', color: 'var(--high)' },
  insufficient: { fr: 'Insuffisant', en: 'Insufficient', color: 'var(--critical)' },
  unrelated: { fr: 'Hors sujet', en: 'Unrelated', color: 'var(--ink-muted)' },
};

export function AiEvidenceAnalysis({ evidenceId }: { evidenceId: string }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const locale = lang === 'fr' ? 'fr' : 'en';
  const analyze = useAnalyzeEvidence();
  const res = analyze.data?.assessment;
  const v = res ? VERDICT[res.verdict] : undefined;

  return (
    <div className="pl-12 pr-1 pb-1">
      <button
        onClick={() => analyze.mutate({ evidenceId, locale })}
        disabled={analyze.isPending}
        className="inline-flex items-center gap-1.5 text-[11.5px] font-medium px-2.5 py-1 rounded-full disabled:opacity-60"
        style={{ border: '1px solid var(--accent-line)', background: 'var(--accent-soft)', color: 'var(--accent)' }}
      >
        {analyze.isPending ? <Loader2 size={13} className="animate-spin" /> : <Sparkles size={13} />}
        {analyze.isPending ? tr('Analyse…', 'Analysing…') : res ? tr('Ré-analyser (IA)', 'Re-analyse (AI)') : tr('Analyser (IA)', 'Analyse (AI)')}
      </button>

      {res && v && (
        <div className="mt-2 p-3 rounded-[11px] text-[12px]" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)', animation: 'or-fadeup .2s ease' }}>
          <div className="flex items-center gap-2 mb-1.5">
            <span className="px-2 py-0.5 rounded-full text-[11px] font-semibold text-white" style={{ background: v.color }}>
              {tr(v.fr, v.en)}
            </span>
            <span className="text-ink-muted text-[11px]">{tr('Confiance', 'Confidence')} {(res.confidence * 100).toFixed(0)}%</span>
            <span className="text-ink-muted text-[10.5px] ml-auto">{analyze.data?.generated_by}</span>
          </div>
          <div className="text-ink-soft leading-relaxed">{res.rationale}</div>
          {res.gaps.length > 0 && (
            <ul className="mt-1.5 list-disc pl-4 text-ink-soft">
              {res.gaps.map((g, i) => <li key={i}>{g}</li>)}
            </ul>
          )}
          {res.suggestions.length > 0 && (
            <div className="mt-1.5 text-ink-muted">
              {tr('Suggestions', 'Suggestions')}: {res.suggestions.join(' ')}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
