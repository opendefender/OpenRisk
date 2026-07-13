// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Framework detail (opened from the Compliance grid): the framework's real controls
// with reference code, source citation and an editable status. Header carries the
// progress gauge + a one-click PDF export. Status filter chips for the long lists.

import { useMemo, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { ArrowLeft, Download, ClipboardCheck } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Chip, Card, RingGauge, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useControls, useComplianceReport } from './useCompliance';
import { useComplianceOverview, frameworkColorFor } from './complianceOverview';
import { CONTROL_STATUSES, type ControlStatus, type ComplianceControl } from '../../types/compliance';

const STATUS_META: Record<string, { color: string; fr: string; en: string }> = {
  implemented: { color: 'var(--low)', fr: 'Implémenté', en: 'Implemented' },
  in_progress: { color: 'var(--high)', fr: 'En cours', en: 'In progress' },
  partially_implemented: { color: 'var(--medium)', fr: 'Partiel', en: 'Partial' },
  not_implemented: { color: 'var(--critical)', fr: 'Non implémenté', en: 'Not implemented' },
  not_applicable: { color: 'var(--text-muted)', fr: 'Non applicable', en: 'Not applicable' },
};

export function FrameworkDetail() {
  const { frameworkId } = useParams<{ frameworkId: string }>();
  const navigate = useNavigate();
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const { data: fws = [] } = useComplianceOverview();
  const fwIndex = fws.findIndex((f) => f.id === frameworkId);
  const fw = fwIndex >= 0 ? fws[fwIndex] : undefined;
  const { controls, isLoading, updateControl } = useControls(frameworkId);
  const report = useComplianceReport();
  const [filter, setFilter] = useState<'all' | ControlStatus>('all');

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const ct of controls) c[ct.status ?? 'not_implemented'] = (c[ct.status ?? 'not_implemented'] ?? 0) + 1;
    return c;
  }, [controls]);
  const filtered = filter === 'all' ? controls : controls.filter((c) => c.status === filter);
  const col = fw ? frameworkColorFor(fw.name, fwIndex) : 'var(--accent)';

  const setStatus = (c: ComplianceControl, status: ControlStatus) => {
    if (status === c.status) return;
    updateControl.mutate(
      { id: c.id as string, payload: { status } },
      { onError: () => toast.error(tr('Mise à jour échouée', 'Update failed')) }
    );
  };

  const downloadReport = () => {
    if (!frameworkId) return;
    toast.promise(report.mutateAsync({ frameworkId, locale: lang }), {
      loading: tr('Génération du rapport…', 'Generating report…'),
      success: tr('Rapport téléchargé', 'Report downloaded'),
      error: tr('Échec de la génération', 'Report generation failed'),
    });
  };

  const meta = (s?: string) => STATUS_META[s ?? 'not_implemented'] ?? STATUS_META.not_implemented;

  return (
    <PageFrame wide>
      <button onClick={() => navigate('/compliance')} className="inline-flex items-center gap-1.5 text-[13px] font-medium text-ink-soft hover:text-ink transition-colors mb-3">
        <ArrowLeft size={15} /> {L.n_compliance}
      </button>

      <PageHeader
        title={fw?.name ?? tr('Référentiel', 'Framework')}
        count={fw ? `${fw.passed}/${fw.total} ${tr('contrôles', 'controls')}` : undefined}
        actions={<Btn label={L.exportPdf} icon={Download} primary onClick={downloadReport} />}
      />

      {fw && (
        <Card style={{ padding: '18px 22px', marginBottom: 16 }}>
          <div className="flex items-center gap-5 flex-wrap">
            <RingGauge value={fw.pct} size={84} color={col} thickness={8}>
              <span className="mono text-[18px] font-bold text-ink">{fw.pct}%</span>
            </RingGauge>
            <div className="flex-1 min-w-[200px]">
              <div className="text-[13px] text-ink-soft mb-2">{fw.description || `${fw.name} · ${fw.version ?? ''}`}</div>
              <div className="flex gap-4 flex-wrap">
                {CONTROL_STATUSES.map((s) => (
                  <span key={s} className="inline-flex items-center gap-1.5 text-[12px] text-ink-soft">
                    <span className="w-2 h-2 rounded-full" style={{ background: meta(s).color }} />
                    {tr(meta(s).fr, meta(s).en)} · <span className="mono font-semibold text-ink">{counts[s] ?? 0}</span>
                  </span>
                ))}
              </div>
            </div>
          </div>
        </Card>
      )}

      <div className="flex gap-2 mb-4 flex-wrap">
        <Chip label={tr('Tous', 'All')} active={filter === 'all'} onClick={() => setFilter('all')} />
        {CONTROL_STATUSES.map((s) => (
          <Chip key={s} label={`${tr(meta(s).fr, meta(s).en)} · ${counts[s] ?? 0}`} active={filter === s} onClick={() => setFilter(s)} color={meta(s).color} />
        ))}
      </div>

      <Card style={{ padding: '8px 8px 4px', overflow: 'hidden' }}>
        {isLoading ? (
          <SkeletonRows rows={8} />
        ) : filtered.length === 0 ? (
          <EmptyState icon={ClipboardCheck} title={tr('Aucun contrôle', 'No controls')} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse" style={{ minWidth: 760 }}>
              <thead style={{ borderBottom: '1px solid var(--border)' }}>
                <tr>
                  {[tr('Réf.', 'Ref.'), tr('Contrôle', 'Control'), tr('Source', 'Source'), tr('Statut', 'Status')].map((t) => (
                    <th key={t} className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{t}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {filtered.map((c) => (
                  <tr key={c.id} style={{ borderBottom: '1px solid var(--border)' }}>
                    <td className="px-3 py-3 align-top"><span className="mono text-[12px] font-semibold text-ink whitespace-nowrap">{c.reference_code}</span></td>
                    <td className="px-3 py-3">
                      <div className="text-[13.5px] font-medium text-ink">{c.name}</div>
                      {c.description && <div className="text-[12px] text-ink-muted mt-0.5 max-w-[520px] leading-snug">{c.description}</div>}
                    </td>
                    <td className="px-3 py-3 align-top"><span className="text-[11.5px] text-ink-muted">{c.source_reference || '—'}</span></td>
                    <td className="px-3 py-3 align-top">
                      <div className="relative inline-flex items-center">
                        <span className="w-2 h-2 rounded-full absolute left-2.5 pointer-events-none" style={{ background: meta(c.status).color }} />
                        <select
                          value={c.status}
                          onChange={(e) => setStatus(c, e.target.value as ControlStatus)}
                          className="appearance-none text-[12px] font-semibold rounded-full pl-6 pr-6 py-1.5 cursor-pointer outline-none"
                          style={{ color: meta(c.status).color, background: `color-mix(in srgb,${meta(c.status).color} 12%,transparent)`, border: `1px solid color-mix(in srgb,${meta(c.status).color} 30%,transparent)` }}
                        >
                          {CONTROL_STATUSES.map((s) => (
                            <option key={s} value={s} style={{ color: 'var(--text-primary)', background: 'var(--bg-elevated)' }}>{tr(meta(s).fr, meta(s).en)}</option>
                          ))}
                        </select>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </PageFrame>
  );
}
