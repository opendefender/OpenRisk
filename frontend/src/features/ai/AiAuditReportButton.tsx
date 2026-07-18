// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// "Rapport IA" button + modal (spec §12.4): generates an executive audit report
// from an audit campaign via POST /ai/audits/:id/report. Composes the audit, a
// live gap analysis and the open remediation count into an executive summary,
// findings, recommendations and a conclusion — ready to export.

import { useState } from 'react';
import { Sparkles, Loader2, X, Copy } from 'lucide-react';
import { useUIStore } from '../../store/uiStore';
import { useAuditReport } from './useAi';

export function AiAuditReportButton({ auditId, title }: { auditId: string; title: string }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const locale = lang === 'fr' ? 'fr' : 'en';
  const [open, setOpen] = useState(false);
  const report = useAuditReport();
  const r = report.data?.report;

  const start = () => {
    setOpen(true);
    if (!report.data) report.mutate({ auditId, locale });
  };

  const copyText = () => {
    if (!r) return;
    const txt = [
      title,
      '',
      tr('Synthèse exécutive', 'Executive summary'),
      r.executive_summary,
      '',
      tr('Constats', 'Findings'),
      r.findings,
      '',
      tr('Recommandations', 'Recommendations'),
      ...r.recommendations.map((x) => `- ${x}`),
      '',
      tr('Conclusion', 'Conclusion'),
      r.conclusion,
    ].join('\n');
    void navigator.clipboard?.writeText(txt);
  };

  return (
    <>
      <button
        onClick={start}
        className="h-8 px-2.5 rounded-[8px] inline-flex items-center gap-1.5 text-[12px] font-semibold transition-colors"
        style={{ border: '1px solid var(--accent-line)', background: 'var(--accent-soft)', color: 'var(--accent)' }}
        title={tr('Générer un rapport exécutif avec l’IA', 'Generate an executive report with AI')}
      >
        <Sparkles size={13} /> {tr('Rapport IA', 'AI report')}
      </button>

      {open && (
        <div className="fixed inset-0 z-[80] flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,.5)', backdropFilter: 'blur(3px)' }} onClick={() => setOpen(false)}>
          <div
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-[640px] max-h-[88vh] flex flex-col rounded-[16px]"
            style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-scalein .2s ease' }}
          >
            <div className="flex items-center gap-3 px-5 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <div className="w-8 h-8 rounded-[9px] flex items-center justify-center text-white shrink-0" style={{ background: 'linear-gradient(135deg,var(--accent),var(--accent-2))' }}><Sparkles size={16} /></div>
              <div className="flex-1 min-w-0">
                <div className="text-[14px] font-semibold text-ink truncate">{tr('Rapport d’audit — IA', 'Audit report — AI')}</div>
                <div className="text-[11.5px] text-ink-muted truncate">{title}</div>
              </div>
              {r && <button onClick={copyText} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-soft hover:text-ink" title={tr('Copier', 'Copy')}><Copy size={15} /></button>}
              <button onClick={() => setOpen(false)} className="w-8 h-8 rounded-lg flex items-center justify-center text-ink-soft hover:text-ink"><X size={17} /></button>
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-4">
              {report.isPending && (
                <div className="flex items-center gap-2 justify-center py-14 text-[13px] text-ink-soft">
                  <Loader2 size={16} className="animate-spin" /> {tr('Génération du rapport…', 'Generating report…')}
                </div>
              )}
              {report.isError && (
                <div className="py-10 text-center text-[13px]" style={{ color: 'var(--critical)' }}>{tr('La génération a échoué.', 'Generation failed.')}</div>
              )}
              {r && (
                <div className="space-y-4">
                  <Section title={tr('Synthèse exécutive', 'Executive summary')} body={r.executive_summary} />
                  <Section title={tr('Constats', 'Findings')} body={r.findings} />
                  <div>
                    <div className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide mb-1.5">{tr('Recommandations', 'Recommendations')}</div>
                    <ul className="list-disc pl-5 space-y-1 text-[13px] text-ink leading-relaxed">
                      {r.recommendations.map((x, i) => <li key={i}>{x}</li>)}
                    </ul>
                  </div>
                  <Section title={tr('Conclusion', 'Conclusion')} body={r.conclusion} />
                  <div className="text-[11px] text-ink-muted pt-2" style={{ borderTop: '1px solid var(--border)' }}>
                    {tr('Généré par', 'Generated by')} : <span className="font-semibold">{report.data?.generated_by}</span>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function Section({ title, body }: { title: string; body: string }) {
  if (!body) return null;
  return (
    <div>
      <div className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide mb-1.5">{title}</div>
      <div className="text-[13px] text-ink leading-relaxed" style={{ whiteSpace: 'pre-line' }}>{body}</div>
    </div>
  );
}
