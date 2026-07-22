// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
// This file is part of the OpenRisk Enterprise Edition and is NOT covered by the
// AGPL; it is licensed under the OpenRisk Commercial License (see LICENSE.commercial).

import { useEffect, useMemo, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Sparkles,
  FileText,
  Download,
  CheckCircle2,
  Trash2,
  Loader2,
  AlertTriangle,
  ShieldAlert,
  ScrollText,
} from 'lucide-react';
import { Button, cn } from '../../components/ui/Button';
import { useBoardReports, useBoardReport } from './useBoardReports';
import type { BoardReport, BoardLocale } from '../../types/board';

// formatFCFA groups thousands with a thin space and appends the currency, matching
// the backend's FormatFCFA so the on-screen figure equals the one in the PDF.
function formatFCFA(n: number): string {
  const sign = n < 0 ? '-' : '';
  const grouped = Math.abs(Math.trunc(n))
    .toString()
    .replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  return `${sign}${grouped} FCFA`;
}

function provenanceLabel(model: string): string {
  if (!model || model === 'template') return 'Modèle déterministe (sans IA)';
  return `Rédigé par l'IA — ${model}`;
}

function StatusBadge({ status }: { status: BoardReport['status'] }) {
  const approved = status === 'approved';
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 text-xs font-semibold px-2 py-0.5 rounded-full border',
        approved
          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'
          : 'bg-amber-500/10 text-amber-400 border-amber-500/30'
      )}
    >
      {approved ? <CheckCircle2 size={12} /> : <ScrollText size={12} />}
      {approved ? 'Approuvé' : 'Brouillon'}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export const BoardReportPage = () => {
  const { reports, isLoading, error, generate, remove } = useBoardReports();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [period, setPeriod] = useState('');
  const [locale, setLocale] = useState<BoardLocale>('fr');

  // Auto-select the most recent report once the list loads.
  useEffect(() => {
    if (!selectedId && reports.length > 0) setSelectedId(reports[0].id);
  }, [reports, selectedId]);

  const onGenerate = () => {
    generate.mutate(
      { period_label: period.trim() || undefined, locale },
      { onSuccess: (r) => setSelectedId(r.id) }
    );
    setPeriod('');
  };

  return (
    <div className="max-w-7xl mx-auto p-6">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-end md:justify-between gap-4 mb-6">
        <div>
          <h2 className="text-2xl font-bold mb-1 flex items-center gap-2">
            <Sparkles size={22} className="text-primary" />
            Rapport du conseil
          </h2>
          <p className="text-zinc-400 text-sm max-w-2xl">
            Une synthèse mensuelle, non technique, pour le conseil d'administration : posture de
            risque, conformité réglementaire et exposition financière estimée en FCFA. Généré en
            brouillon, relu, puis approuvé avant diffusion.
          </p>
        </div>

        {/* Generate control */}
        <div className="flex items-center gap-2 shrink-0">
          <input
            value={period}
            onChange={(e) => setPeriod(e.target.value)}
            placeholder={locale === 'fr' ? 'Période (ex. Juillet 2026)' : 'Period (e.g. July 2026)'}
            className="bg-surface border border-border rounded-lg px-3 py-2 text-sm w-48 focus:outline-none focus:ring-2 focus:ring-primary/40"
          />
          <select
            value={locale}
            onChange={(e) => setLocale(e.target.value as BoardLocale)}
            className="bg-surface border border-border rounded-lg px-2 py-2 text-sm focus:outline-none"
            aria-label="Langue"
          >
            <option value="fr">FR</option>
            <option value="en">EN</option>
          </select>
          <Button onClick={onGenerate} isLoading={generate.isPending} className="shadow-lg shadow-primary/20">
            <Sparkles size={16} className="mr-2" />
            Générer
          </Button>
        </div>
      </div>

      {generate.isError && (
        <div className="mb-4 flex items-center gap-2 text-sm text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg px-4 py-2">
          <AlertTriangle size={16} /> La génération a échoué. Réessayez.
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* List */}
        <div className="lg:col-span-4 space-y-3">
          {isLoading ? (
            <ListSkeleton />
          ) : error ? (
            <div className="text-sm text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg p-4">
              Erreur de chargement des rapports.
            </div>
          ) : reports.length === 0 ? (
            <EmptyList generating={generate.isPending} onGenerate={onGenerate} />
          ) : (
            reports.map((r) => (
              <ReportCard
                key={r.id}
                report={r}
                active={r.id === selectedId}
                onSelect={() => setSelectedId(r.id)}
              />
            ))
          )}
        </div>

        {/* Detail */}
        <div className="lg:col-span-8">
          {selectedId ? (
            <ReportDetail
              key={selectedId}
              id={selectedId}
              onDeleted={() => {
                remove.mutate(selectedId);
                setSelectedId(null);
              }}
              deleting={remove.isPending}
            />
          ) : (
            !isLoading &&
            reports.length > 0 && (
              <div className="text-zinc-500 text-sm p-8 text-center">
                Sélectionnez un rapport pour le consulter.
              </div>
            )
          )}
        </div>
      </div>
    </div>
  );
};

// ---------------------------------------------------------------------------
// List pieces
// ---------------------------------------------------------------------------

function ReportCard({
  report,
  active,
  onSelect,
}: {
  report: BoardReport;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      onClick={onSelect}
      className={cn(
        'w-full text-left bg-surface border rounded-xl p-4 transition-all',
        active ? 'border-primary/60 ring-1 ring-primary/40' : 'border-border hover:border-zinc-600'
      )}
    >
      <div className="flex items-center justify-between mb-2">
        <span className="font-semibold text-sm text-white truncate">{report.period_label}</span>
        <StatusBadge status={report.status} />
      </div>
      <div className="flex items-center gap-4 text-xs text-zinc-400">
        <span>
          Conformité <span className="text-zinc-200 font-medium">{Math.round(report.overall_compliance_percent)}%</span>
        </span>
        <span>
          Risques <span className="text-zinc-200 font-medium">{report.risks_total}</span>
          {report.risks_critical > 0 && (
            <span className="text-red-400"> ({report.risks_critical} crit.)</span>
          )}
        </span>
      </div>
      <div className="mt-1 text-xs text-zinc-500">{formatFCFA(report.financial_exposure_fcfa)}</div>
    </button>
  );
}

function ListSkeleton() {
  return (
    <>
      {[0, 1, 2].map((i) => (
        <div key={i} className="bg-surface border border-border rounded-xl p-4 animate-pulse">
          <div className="h-4 w-24 bg-white/10 rounded mb-3" />
          <div className="h-3 w-40 bg-white/5 rounded" />
        </div>
      ))}
    </>
  );
}

function EmptyList({ generating, onGenerate }: { generating: boolean; onGenerate: () => void }) {
  return (
    <div className="bg-surface border border-dashed border-border rounded-xl p-8 text-center">
      <FileText size={40} className="mx-auto mb-3 text-zinc-600" />
      <p className="text-sm text-zinc-400 mb-4">
        Aucun rapport pour l'instant. Générez le premier rapport du conseil en un clic.
      </p>
      <Button onClick={onGenerate} isLoading={generating}>
        <Sparkles size={16} className="mr-2" /> Générer le premier rapport
      </Button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Detail (edit + approve + download)
// ---------------------------------------------------------------------------

interface Draft {
  executive_summary: string;
  risk_commentary: string;
  compliance_commentary: string;
  financial_commentary: string;
  recommendations: string; // one per line for editing
}

function toDraft(r: BoardReport): Draft {
  return {
    executive_summary: r.executive_summary,
    risk_commentary: r.risk_commentary,
    compliance_commentary: r.compliance_commentary,
    financial_commentary: r.financial_commentary,
    recommendations: (r.recommendations ?? []).join('\n'),
  };
}

function ReportDetail({
  id,
  onDeleted,
  deleting,
}: {
  id: string;
  onDeleted: () => void;
  deleting: boolean;
}) {
  const { report, isLoading, error, update, approve, download } = useBoardReport(id);
  const [draft, setDraft] = useState<Draft | null>(null);

  useEffect(() => {
    if (report) setDraft(toDraft(report));
  }, [report]);

  const dirty = useMemo(() => {
    if (!report || !draft) return false;
    const original = toDraft(report);
    return (Object.keys(original) as (keyof Draft)[]).some((k) => original[k] !== draft[k]);
  }, [report, draft]);

  if (isLoading || !report || !draft) {
    return <div className="bg-surface border border-border rounded-xl p-8 animate-pulse h-96" />;
  }
  if (error) {
    return (
      <div className="text-sm text-red-400 bg-red-500/10 border border-red-500/20 rounded-lg p-4">
        Erreur de chargement du rapport.
      </div>
    );
  }

  const readOnly = report.status === 'approved';

  const onSave = () => {
    update.mutate({
      executive_summary: draft.executive_summary,
      risk_commentary: draft.risk_commentary,
      compliance_commentary: draft.compliance_commentary,
      financial_commentary: draft.financial_commentary,
      recommendations: draft.recommendations
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean),
    });
  };

  const set = (k: keyof Draft) => (e: React.ChangeEvent<HTMLTextAreaElement>) =>
    setDraft((d) => (d ? { ...d, [k]: e.target.value } : d));

  return (
    <AnimatePresence mode="wait">
      <motion.div
        key={report.id}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        className="bg-surface border border-border rounded-xl p-6"
      >
        {/* Header */}
        <div className="flex items-start justify-between gap-4 mb-1">
          <div>
            <h3 className="text-lg font-bold text-white">{report.title}</h3>
            <p className="text-xs text-zinc-500 mt-0.5">{provenanceLabel(report.generated_by_model)}</p>
          </div>
          <StatusBadge status={report.status} />
        </div>

        {/* Posture KPIs */}
        <div className="grid grid-cols-3 gap-3 my-5">
          <KpiTile label="Conformité" value={`${Math.round(report.overall_compliance_percent)}%`} />
          <KpiTile
            label="Risques actifs"
            value={`${report.risks_total}`}
            sub={report.risks_critical > 0 ? `${report.risks_critical} critiques` : 'aucun critique'}
            danger={report.risks_critical > 0}
          />
          <KpiTile label="Exposition estimée" value={formatFCFA(report.financial_exposure_fcfa)} small />
        </div>

        <RiskChips report={report} />

        {/* Narrative */}
        <div className="mt-5 space-y-4">
          <Section
            title="Synthèse exécutive"
            value={draft.executive_summary}
            onChange={set('executive_summary')}
            readOnly={readOnly}
            rows={5}
          />
          <Section
            title="Posture de risque"
            value={draft.risk_commentary}
            onChange={set('risk_commentary')}
            readOnly={readOnly}
          />
          <Section
            title="Conformité réglementaire"
            value={draft.compliance_commentary}
            onChange={set('compliance_commentary')}
            readOnly={readOnly}
          />
          <Section
            title="Exposition financière"
            value={draft.financial_commentary}
            onChange={set('financial_commentary')}
            readOnly={readOnly}
          />
          <Section
            title="Recommandations (une par ligne)"
            value={draft.recommendations}
            onChange={set('recommendations')}
            readOnly={readOnly}
            rows={4}
          />

          {report.frameworks_snapshot && report.frameworks_snapshot.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold text-zinc-300 mb-2">Conformité par référentiel</h4>
              <div className="space-y-2">
                {report.frameworks_snapshot.map((f) => (
                  <div key={f.name + f.version} className="flex items-center gap-3">
                    <span className="text-xs text-zinc-400 w-40 truncate">
                      {f.name}
                      {f.version ? ` (${f.version})` : ''}
                    </span>
                    <div className="flex-1 h-2 bg-white/5 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-primary rounded-full"
                        style={{ width: `${Math.min(100, Math.max(0, f.percent_complete))}%` }}
                      />
                    </div>
                    <span className="text-xs text-zinc-400 w-24 text-right">
                      {Math.round(f.percent_complete)}% ({f.implemented}/{f.applicable})
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex flex-wrap items-center gap-2 mt-6 pt-4 border-t border-border">
          {!readOnly && (
            <>
              <Button onClick={onSave} disabled={!dirty} isLoading={update.isPending}>
                Enregistrer les modifications
              </Button>
              <Button
                variant="secondary"
                onClick={() => approve.mutate()}
                isLoading={approve.isPending}
              >
                <CheckCircle2 size={16} className="mr-2" /> Approuver
              </Button>
            </>
          )}
          <Button variant="ghost" onClick={() => download.mutate()} isLoading={download.isPending}>
            <Download size={16} className="mr-2" /> Télécharger le PDF
          </Button>
          <div className="flex-1" />
          <Button variant="danger" onClick={onDeleted} isLoading={deleting} title="Supprimer">
            <Trash2 size={16} />
          </Button>
        </div>

        {readOnly && (
          <p className="text-xs text-zinc-500 mt-3">
            Ce rapport est approuvé et ne peut plus être modifié.
            {report.approved_at && ` Approuvé le ${new Date(report.approved_at).toLocaleString()}.`}
          </p>
        )}
      </motion.div>
    </AnimatePresence>
  );
}

function KpiTile({
  label,
  value,
  sub,
  danger,
  small,
}: {
  label: string;
  value: string;
  sub?: string;
  danger?: boolean;
  small?: boolean;
}) {
  return (
    <div className="bg-white/5 border border-border rounded-lg p-3">
      <div className="text-[11px] uppercase tracking-wide text-zinc-500 mb-1">{label}</div>
      <div className={cn('font-bold', small ? 'text-base' : 'text-2xl', danger && 'text-red-400')}>
        {value}
      </div>
      {sub && <div className="text-[11px] text-zinc-500 mt-0.5">{sub}</div>}
    </div>
  );
}

function RiskChips({ report }: { report: BoardReport }) {
  const chips = [
    { label: 'Critiques', count: report.risks_critical, color: 'bg-red-500' },
    { label: 'Élevés', count: report.risks_high, color: 'bg-orange-500' },
    { label: 'Moyens', count: report.risks_medium, color: 'bg-amber-500' },
    { label: 'Faibles', count: report.risks_low, color: 'bg-emerald-500' },
  ];
  return (
    <div className="flex flex-wrap gap-3">
      {chips.map((c) => (
        <span key={c.label} className="inline-flex items-center gap-1.5 text-xs text-zinc-300">
          <span className={cn('w-2.5 h-2.5 rounded-sm', c.color)} />
          {c.label}: <span className="font-semibold">{c.count}</span>
        </span>
      ))}
    </div>
  );
}

function Section({
  title,
  value,
  onChange,
  readOnly,
  rows = 3,
}: {
  title: string;
  value: string;
  onChange: (e: React.ChangeEvent<HTMLTextAreaElement>) => void;
  readOnly: boolean;
  rows?: number;
}) {
  return (
    <div>
      <div className="flex items-center gap-2 mb-1.5">
        {title === 'Posture de risque' ? (
          <ShieldAlert size={14} className="text-zinc-500" />
        ) : (
          <FileText size={14} className="text-zinc-500" />
        )}
        <h4 className="text-sm font-semibold text-zinc-300">{title}</h4>
      </div>
      {readOnly ? (
        <p className="text-sm text-zinc-300 whitespace-pre-wrap leading-relaxed">{value || '—'}</p>
      ) : (
        <textarea
          value={value}
          onChange={onChange}
          rows={rows}
          className="w-full bg-white/5 border border-border rounded-lg p-3 text-sm text-zinc-200 leading-relaxed resize-y focus:outline-none focus:ring-2 focus:ring-primary/40"
        />
      )}
    </div>
  );
}

export default BoardReportPage;
