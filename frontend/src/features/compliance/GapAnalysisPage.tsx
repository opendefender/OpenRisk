// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Gap analysis ("analyse d'écarts") — every unsatisfied control across the
// tenant's frameworks, grouped by framework, with per-framework roll-ups. Wired
// to GET /compliance/gap-analysis. The "Voir les écarts" CTA on ComplianceScreen
// lands here.

import { useMemo, useState } from 'react';
import { AlertTriangle, ChevronRight, ShieldCheck, FileText, Filter, Wrench } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { PageFrame, PageHeader, Btn, Card, RingGauge, SkeletonRows, EmptyState, ErrorState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useGapAnalysis } from './useCompliance';
import { CreateRemediationDialog } from './AuditRemediationModals';
import type { ControlStatus, GapControl } from '../../types/compliance';

const STATUS_META: Record<string, { color: string; fr: string; en: string }> = {
  in_progress: { color: 'var(--high)', fr: 'En cours', en: 'In progress' },
  not_implemented: { color: 'var(--critical)', fr: 'Non implémenté', en: 'Not implemented' },
};

export function GapAnalysisPage() {
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data, isLoading, error, refetch } = useGapAnalysis();
  const hasPermission = useAuthStore((s) => s.hasPermission);
  const canRemediate = hasPermission('compliance:remediations:write');
  const [fwFilter, setFwFilter] = useState<string>('all');
  const [remediate, setRemediate] = useState<GapControl | null>(null);

  const meta = (s?: string) => STATUS_META[s ?? 'not_implemented'] ?? STATUS_META.not_implemented;

  const gaps = useMemo(() => {
    const all = data?.gaps ?? [];
    return fwFilter === 'all' ? all : all.filter((g) => g.framework_id === fwFilter);
  }, [data, fwFilter]);

  // Group gaps by framework for a readable, sectioned list.
  const grouped = useMemo(() => {
    const m = new Map<string, { name: string; items: GapControl[] }>();
    for (const g of gaps) {
      const entry = m.get(g.framework_id) ?? { name: g.framework_name, items: [] };
      entry.items.push(g);
      m.set(g.framework_id, entry);
    }
    return Array.from(m.entries());
  }, [gaps]);

  const totalGaps = data?.total_gaps ?? 0;
  const totalControls = data?.total_controls ?? 0;
  const coverage = totalControls > 0 ? Math.round(((totalControls - totalGaps) / totalControls) * 100) : 100;
  const gaugeColor = coverage >= 70 ? 'var(--low)' : coverage >= 40 ? 'var(--high)' : 'var(--critical)';

  return (
    <PageFrame>
      <PageHeader
        title={tr("Analyse d'écarts", 'Gap analysis')}
        actions={<Btn label={tr('Voir la conformité', 'View compliance')} icon={FileText} onClick={() => navigate('/compliance')} />}
      />

      {isLoading ? (
        <Card style={{ padding: 8 }}><SkeletonRows rows={5} height={56} /></Card>
      ) : error ? (
        <Card><ErrorState title={tr('Chargement impossible', 'Failed to load')} onRetry={() => refetch()} retryLabel={tr('Réessayer', 'Retry')} /></Card>
      ) : totalControls === 0 ? (
        <Card>
          <EmptyState
            icon={ShieldCheck}
            title={tr('Aucun référentiel suivi', 'No frameworks tracked')}
            sub={tr('Importez un référentiel pour lancer une analyse d’écarts.', 'Import a framework to run a gap analysis.')}
            cta={<Btn label={tr('Aller à la conformité', 'Go to compliance')} primary onClick={() => navigate('/compliance')} />}
          />
        </Card>
      ) : totalGaps === 0 ? (
        <Card>
          <EmptyState
            icon={ShieldCheck}
            title={tr('Aucun écart — 100 % couvert', 'No gaps — 100% covered')}
            sub={tr('Tous les contrôles applicables sont implémentés sur vos référentiels.', 'Every applicable control is implemented across your frameworks.')}
          />
        </Card>
      ) : (
        <>
          {/* Hero */}
          <Card style={{ padding: '22px 24px', marginBottom: 16 }}>
            <div className="flex items-center gap-6 flex-wrap">
              <RingGauge value={coverage} size={128} color={gaugeColor}>
                <span className="disp mono text-[32px] font-bold text-ink">{coverage}%</span>
                <span className="text-[11px] text-ink-muted">{tr('couvert', 'covered')}</span>
              </RingGauge>
              <div className="flex-1 min-w-[280px]">
                <div className="disp text-[19px] font-bold text-ink mb-1.5 flex items-center gap-2">
                  <AlertTriangle size={18} style={{ color: 'var(--critical)' }} />
                  {totalGaps} {tr('écart', 'gap')}{totalGaps > 1 ? 's' : ''}
                </div>
                <div className="text-[13.5px] text-ink-soft leading-relaxed max-w-[560px]">
                  {tr(
                    `Sur ${totalControls} contrôles suivis, ${totalGaps} ne sont pas satisfaits (non implémentés ou en cours). Traitez-les via un plan de remédiation.`,
                    `Of ${totalControls} tracked controls, ${totalGaps} are unsatisfied (not implemented or in progress). Address them with a remediation plan.`
                  )}
                </div>
              </div>
            </div>
          </Card>

          {/* Per-framework roll-up + filter chips */}
          <div className="flex items-center gap-2 flex-wrap mb-4">
            <span className="inline-flex items-center gap-1.5 text-[12px] text-ink-muted mr-1"><Filter size={13} /> {tr('Filtrer', 'Filter')}</span>
            <FilterChip label={tr('Tous', 'All')} active={fwFilter === 'all'} onClick={() => setFwFilter('all')} count={data?.total_gaps ?? 0} />
            {(data?.frameworks ?? []).filter((f) => f.gaps > 0).map((f) => (
              <FilterChip key={f.framework_id} label={f.framework_name} active={fwFilter === f.framework_id} onClick={() => setFwFilter(f.framework_id)} count={f.gaps} />
            ))}
          </div>

          {/* Grouped gap list */}
          <div className="flex flex-col gap-4">
            {grouped.map(([fwId, group], gi) => (
              <Card key={fwId} style={{ padding: 0, overflow: 'hidden', animation: 'or-fadeup .4s ease both', animationDelay: `${Math.min(gi * 0.04, 0.3)}s` }}>
                <button
                  onClick={() => navigate(`/compliance/${fwId}`)}
                  className="w-full flex items-center justify-between gap-3 px-5 py-3.5 text-left group hover:bg-hover transition-colors"
                  style={{ borderBottom: '1px solid var(--border)' }}
                >
                  <div className="flex items-center gap-2.5">
                    <span className="text-[14px] font-semibold text-ink group-hover:text-accent transition-colors">{group.name}</span>
                    <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full" style={{ background: 'color-mix(in srgb,var(--critical) 12%,transparent)', color: 'var(--critical)' }}>
                      {group.items.length} {tr('écart', 'gap')}{group.items.length > 1 ? 's' : ''}
                    </span>
                  </div>
                  <ChevronRight size={16} className="text-ink-muted group-hover:text-accent transition-colors" />
                </button>
                <div>
                  {group.items.map((g) => {
                    const m = meta(g.status);
                    return (
                      <div key={g.control_id} className="flex items-start gap-3 px-5 py-3" style={{ borderTop: '1px solid var(--border)' }}>
                        <span className="mono text-[12px] font-semibold text-ink-soft shrink-0 mt-0.5 w-[92px] truncate" title={g.reference_code}>{g.reference_code}</span>
                        <div className="flex-1 min-w-0">
                          <div className="text-[13.5px] font-medium text-ink">{g.name}</div>
                          {g.source_reference && <div className="text-[11.5px] text-ink-muted mt-0.5 truncate" title={g.source_reference}>{g.source_reference}</div>}
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          {g.evidence_count > 0 && (
                            <span className="text-[11px] text-ink-muted">{g.evidence_count} {tr('preuve', 'evidence')}{g.evidence_count > 1 ? 's' : ''}</span>
                          )}
                          <span className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold px-2.5 py-1 rounded-full" style={{ background: `color-mix(in srgb,${m.color} 12%,transparent)`, color: m.color }}>
                            <span className="h-1.5 w-1.5 rounded-full" style={{ background: m.color }} />
                            {lang === 'fr' ? m.fr : m.en}
                          </span>
                          {canRemediate && (
                            <button
                              onClick={() => setRemediate(g)}
                              className="inline-flex items-center gap-1.5 h-7 px-2.5 rounded-[8px] text-[11.5px] font-semibold text-ink-soft hover:text-ink transition-colors"
                              style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
                              title={tr('Créer un plan de remédiation', 'Create a remediation plan')}
                            >
                              <Wrench size={12} /> {tr('Remédier', 'Remediate')}
                            </button>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </Card>
            ))}
          </div>
        </>
      )}

      {remediate && (
        <CreateRemediationDialog
          onClose={() => setRemediate(null)}
          controlId={remediate.control_id}
          controlLabel={`${remediate.reference_code} — ${remediate.name}`}
        />
      )}
    </PageFrame>
  );
}

function FilterChip({ label, active, onClick, count }: { label: string; active?: boolean; onClick?: () => void; count?: number }) {
  return (
    <button
      onClick={onClick}
      className="inline-flex items-center gap-1.5 h-7 px-3 rounded-full text-[12px] font-semibold transition-colors"
      style={{
        border: '1px solid var(--border-strong)',
        background: active ? 'var(--accent)' : 'transparent',
        color: active ? '#fff' : 'var(--text-soft)',
      }}
    >
      {label}
      {typeof count === 'number' && (
        <span className="text-[10.5px] font-bold px-1.5 rounded-full" style={{ background: active ? 'rgba(255,255,255,.25)' : 'var(--hover)' }}>{count}</span>
      )}
    </button>
  );
}
