// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// Scan Preview — the human-in-the-loop gate. A scan's results live only in a
// Redis preview (48h TTL); nothing is in the DB until the user imports here.
// Tabs: Assets | Findings | Auto-detected mitigations. Assets are selectable
// with an editable criticality, then "Import selection" promotes them to the
// real inventory.

import { useMemo, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  ArrowLeft, Boxes, Bug, Wrench, Check, Server, Cloud, Download, Trash2, ShieldCheck,
} from 'lucide-react';
import toast from 'react-hot-toast';
import { PageFrame, PageHeader, Btn, Card, Skeleton, EmptyState, ErrorState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useScanPreview } from './useScanner';
import { PROVIDERS, severityColor, criticalityFromFactor } from './scannerMeta';
import type { AssetCriticality, ImportSelection } from './scannerService';

const CRITS: AssetCriticality[] = ['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'];
const CRIT_COLOR: Record<AssetCriticality, string> = {
  LOW: 'var(--low)', MEDIUM: 'var(--medium)', HIGH: 'var(--high)', CRITICAL: 'var(--critical)',
};

type Tab = 'assets' | 'findings' | 'mitigations';

export function ScanPreviewPage() {
  const { jobId } = useParams<{ jobId: string }>();
  const navigate = useNavigate();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const canImport = useAuthStore((s) => s.hasPermission)('scanner:import');

  const { preview, isLoading, error, importPreview, ignorePreview } = useScanPreview(jobId);
  const [tab, setTab] = useState<Tab>('assets');
  // selection: external_id -> criticality override (default = inferred)
  const [selected, setSelected] = useState<Record<string, AssetCriticality>>({});

  const assets = preview?.assets ?? [];
  const findings = preview?.findings ?? [];
  const mitigations = preview?.mitigations ?? [];

  const allSelected = assets.length > 0 && assets.every((a) => selected[a.external_id]);
  const selCount = Object.keys(selected).length;

  const toggle = (extId: string, inferred: AssetCriticality) =>
    setSelected((s) => {
      const next = { ...s };
      if (next[extId]) delete next[extId];
      else next[extId] = inferred;
      return next;
    });
  const toggleAll = () =>
    setSelected(() => {
      if (allSelected) return {};
      const next: Record<string, AssetCriticality> = {};
      for (const a of assets) next[a.external_id] = criticalityFromFactor(a.criticality);
      return next;
    });
  const setCrit = (extId: string, c: AssetCriticality) => setSelected((s) => ({ ...s, [extId]: c }));

  const doImport = () => {
    const selections: ImportSelection[] = Object.entries(selected).map(([external_id, criticality]) => ({ external_id, criticality }));
    if (!selections.length) return toast.error(tr('Sélectionnez au moins un actif', 'Select at least one asset'));
    importPreview.mutate(selections, {
      onSuccess: (r) => {
        toast.success(tr(`${r.assets_imported} actif(s) importé(s)`, `Imported ${r.assets_imported} asset(s)`));
        setSelected({});
        navigate('/assets');
      },
      onError: (e) => toast.error((e as { response?: { data?: { error?: string } } })?.response?.data?.error ?? tr('Échec de l’import', 'Import failed')),
    });
  };

  const doIgnore = () => {
    if (!confirm(tr('Ignorer et supprimer ce preview ?', 'Ignore and discard this preview?'))) return;
    ignorePreview.mutate(undefined, {
      onSuccess: () => { toast.success(tr('Preview ignoré', 'Preview discarded')); navigate('/infrastructure'); },
    });
  };

  if (isLoading) {
    return <PageFrame><div className="flex flex-col gap-3">{[0, 1, 2, 3].map((i) => <Skeleton key={i} style={{ height: 60 }} />)}</div></PageFrame>;
  }
  if (error || !preview) {
    return (
      <PageFrame>
        <ErrorState
          title={tr('Preview indisponible', 'Preview unavailable')}
          sub={tr('Il a peut-être expiré (48h) ou a déjà été importé.', 'It may have expired (48h) or already been imported.')}
          onRetry={() => navigate('/infrastructure')}
          retryLabel={tr('Retour', 'Back')}
        />
      </PageFrame>
    );
  }

  const m = PROVIDERS[preview.provider];
  const sourceLabel = preview.agent_name
    ? tr(`Scan exécuté par l'Agent ${preview.agent_name} (sur site)`, `Scan run by Agent ${preview.agent_name} (on-prem)`)
    : tr(`Scan ${m.short} (cloud)`, `${m.short} scan (cloud)`);

  const tabs: { key: Tab; label: string; icon: typeof Boxes; n: number }[] = [
    { key: 'assets', label: tr('Actifs', 'Assets'), icon: Boxes, n: assets.length },
    { key: 'findings', label: tr('Vulnérabilités', 'Findings'), icon: Bug, n: findings.length },
    { key: 'mitigations', label: tr('Mitigations auto', 'Auto-mitigations'), icon: Wrench, n: mitigations.length },
  ];

  return (
    <PageFrame wide>
      <button onClick={() => navigate('/infrastructure')} className="inline-flex items-center gap-1.5 text-[12.5px] font-semibold text-ink-soft hover:text-ink mb-3">
        <ArrowLeft size={15} /> {tr('Infrastructure', 'Infrastructure')}
      </button>
      <PageHeader
        title={tr('Aperçu du scan', 'Scan preview')}
        actions={
          canImport ? (
            <>
              <Btn label={tr('Ignorer', 'Ignore')} icon={Trash2} onClick={doIgnore} />
              <Btn label={importPreview.isPending ? tr('Import…', 'Importing…') : tr(`Importer (${selCount})`, `Import selection (${selCount})`)} icon={Download} primary onClick={doImport} />
            </>
          ) : undefined
        }
      />

      {/* source banner */}
      <div className="flex items-center gap-2.5 rounded-xl px-4 py-3 mb-4" style={{ background: `color-mix(in srgb,${m.color} 8%,transparent)`, border: `1px solid color-mix(in srgb,${m.color} 22%,transparent)` }}>
        {m.cloud ? <Cloud size={16} style={{ color: m.color }} /> : <Server size={16} style={{ color: m.color }} />}
        <span className="text-[13px] font-medium text-ink">{sourceLabel}</span>
        <span className="text-[12px] text-ink-soft ml-auto inline-flex items-center gap-1.5"><ShieldCheck size={14} style={{ color: 'var(--low)' }} /> {tr('Rien n’est écrit tant que vous n’importez pas', 'Nothing is saved until you import')}</span>
      </div>

      {(preview.errors?.length ?? 0) > 0 && (
        <div className="rounded-xl px-4 py-3 mb-4 text-[12.5px]" style={{ background: 'color-mix(in srgb,var(--critical) 8%,transparent)', border: '1px solid color-mix(in srgb,var(--critical) 25%,transparent)', color: 'var(--critical)' }}>
          {preview.errors?.join(' · ')}
        </div>
      )}

      {/* tabs */}
      <div className="flex items-center gap-1.5 mb-4">
        {tabs.map((t) => (
          <button key={t.key} onClick={() => setTab(t.key)} className="h-9 px-3.5 rounded-[10px] text-[13px] font-semibold inline-flex items-center gap-2 transition-all" style={{ background: tab === t.key ? 'var(--accent-soft)' : 'transparent', color: tab === t.key ? 'var(--accent)' : 'var(--text-secondary)', border: `1px solid ${tab === t.key ? 'transparent' : 'var(--border)'}` }}>
            <t.icon size={15} /> {t.label}
            <span className="text-[11px] font-bold px-1.5 py-[1px] rounded-full" style={{ background: 'var(--bg-hover)' }}>{t.n}</span>
          </button>
        ))}
      </div>

      <Card style={{ padding: 0 }}>
        {tab === 'assets' && (
          assets.length === 0 ? <EmptyState icon={Boxes} title={tr('Aucun actif découvert', 'No assets discovered')} /> : (
            <div>
              <div className="flex items-center gap-3 px-4 py-2.5 text-[11.5px] font-semibold text-ink-soft uppercase tracking-wide" style={{ borderBottom: '1px solid var(--border)' }}>
                {canImport && <input type="checkbox" checked={allSelected} onChange={toggleAll} className="accent-[var(--accent)]" />}
                <span className="flex-1">{tr('Actif', 'Asset')}</span>
                <span className="w-24 hidden sm:block">{tr('Type', 'Type')}</span>
                <span className="w-28">{tr('Criticité', 'Criticality')}</span>
              </div>
              {assets.map((a) => {
                const inferred = criticalityFromFactor(a.criticality);
                const sel = selected[a.external_id];
                const crit = sel ?? inferred;
                return (
                  <div key={a.external_id} className="flex items-center gap-3 px-4 py-3" style={{ borderBottom: '1px solid var(--border)' }}>
                    {canImport && <input type="checkbox" checked={!!sel} onChange={() => toggle(a.external_id, inferred)} className="accent-[var(--accent)]" />}
                    <div className="min-w-0 flex-1">
                      <div className="text-[13px] font-semibold text-ink truncate">{a.name || a.external_id}</div>
                      <div className="text-[11.5px] text-ink-soft truncate">{[a.ip, a.os, a.environment].filter(Boolean).join(' · ') || a.external_id}{a.cpe?.length ? ` · ${a.cpe.length} CPE` : ''}</div>
                    </div>
                    <span className="w-24 hidden sm:block text-[12px] text-ink-soft truncate">{a.type}</span>
                    <div className="w-28">
                      {canImport && sel ? (
                        <select value={crit} onChange={(e) => setCrit(a.external_id, e.target.value as AssetCriticality)} className="text-[12px] font-semibold rounded-lg px-2 py-1.5 w-full" style={{ background: 'var(--bg-elevated)', border: `1px solid ${CRIT_COLOR[crit]}`, color: CRIT_COLOR[crit] }}>
                          {CRITS.map((c) => <option key={c} value={c}>{c}</option>)}
                        </select>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-[11.5px] font-semibold px-2 py-1 rounded-full" style={{ color: CRIT_COLOR[inferred], background: `color-mix(in srgb,${CRIT_COLOR[inferred]} 14%,transparent)` }}>
                          <span className="w-1.5 h-1.5 rounded-full" style={{ background: CRIT_COLOR[inferred] }} />{inferred}
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )
        )}

        {tab === 'findings' && (
          findings.length === 0 ? <EmptyState icon={Bug} title={tr('Aucune vulnérabilité', 'No findings')} /> : (
            <div>
              {findings.map((f, i) => (
                <div key={`${f.asset_external_id}-${f.cve ?? f.title}-${i}`} className="flex items-start gap-3 px-4 py-3.5" style={{ borderBottom: '1px solid var(--border)' }}>
                  <span className="mt-1.5 w-2 h-2 rounded-full shrink-0" style={{ background: severityColor(f.severity) }} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-[13px] font-semibold text-ink">{f.title}</span>
                      {f.cve && <span className="mono text-[11px] font-semibold px-1.5 py-[1px] rounded" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>{f.cve}</span>}
                    </div>
                    <div className="text-[11.5px] text-ink-soft mt-0.5">{f.asset_external_id} · {f.evidence}</div>
                  </div>
                  <span className="text-[11px] font-semibold uppercase shrink-0" style={{ color: severityColor(f.severity) }}>{f.severity}</span>
                </div>
              ))}
            </div>
          )
        )}

        {tab === 'mitigations' && (
          mitigations.length === 0 ? <EmptyState icon={Wrench} title={tr('Aucune mitigation détectée', 'No auto-mitigations')} sub={tr('Comparé au scan précédent de cette config.', 'Compared against the previous scan of this config.')} /> : (
            <div>
              {mitigations.map((mit, i) => (
                <div key={`${mit.asset_external_id}-${mit.cve ?? mit.title}-${i}`} className="flex items-start gap-3 px-4 py-3.5" style={{ borderBottom: '1px solid var(--border)' }}>
                  <div className="mt-0.5 w-6 h-6 rounded-lg flex items-center justify-center shrink-0" style={{ background: 'color-mix(in srgb,var(--low) 15%,transparent)', color: 'var(--low)' }}><Check size={14} /></div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="text-[13px] font-semibold text-ink">{mit.title}</span>
                      {mit.cve && <span className="mono text-[11px] font-semibold px-1.5 py-[1px] rounded" style={{ background: 'var(--bg-hover)', color: 'var(--text-secondary)' }}>{mit.cve}</span>}
                    </div>
                    <div className="text-[11.5px] text-ink-soft mt-0.5">{mit.evidence}</div>
                  </div>
                  <span className="text-[11px] font-semibold shrink-0" style={{ color: 'var(--low)' }}>{tr('Résolu', 'Resolved')}</span>
                </div>
              ))}
            </div>
          )
        )}
      </Card>
    </PageFrame>
  );
}
