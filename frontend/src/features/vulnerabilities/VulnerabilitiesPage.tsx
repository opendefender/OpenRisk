// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Vulnerability Management (Module 3). A tenant-scoped register of findings
// ingested from Nessus/OpenVAS/Qualys/Defender/Inspector/Azure Defender/
// CrowdStrike, risk-based prioritised (CVSS + exploitability + business
// criticality + affected assets) into P1..P4. List sorted by priority, KPI
// stats, filters, a detail drawer (status lifecycle + prioritisation breakdown),
// an ingest modal and a connectors panel.

import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import {
  Bug, Search, X, Upload, Plug, Flame, ShieldAlert, Zap, Trash2, ChevronRight,
  Ticket, ExternalLink,
} from 'lucide-react';
import { PageFrame, PageHeader, Btn, Chip, Card, SkeletonRows, EmptyState } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useVulnerabilities, useVulnStats, useVulnMutations } from './useVulnerabilities';
import type { Vulnerability, VulnStatus, VulnQueryParams } from './vulnerabilityService';
import { SEVERITY_META, STATUS_META, STATUS_ORDER, TIER_META, SOURCE_LABEL, pick } from './vulnMeta';
import { IngestModal } from './IngestModal';
import { IntegrationsPanel } from './IntegrationsPanel';

const cvssColor = (s: number) =>
  s >= 9 ? 'var(--critical)' : s >= 7 ? 'var(--high)' : s >= 4 ? 'var(--medium)' : 'var(--low)';

export function VulnerabilitiesPage() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const [tierFilter, setTierFilter] = useState<string | null>(null);
  const [sevFilter, setSevFilter] = useState<string | null>(null);
  const [kevOnly, setKevOnly] = useState(false);
  const [query, setQuery] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const [drawerId, setDrawerId] = useState<string | null>(null);
  const [ingestOpen, setIngestOpen] = useState(false);
  const [connectorsOpen, setConnectorsOpen] = useState(false);

  const params: VulnQueryParams = useMemo(() => {
    const p: VulnQueryParams = { limit: 100, sort_by: 'priority_score', sort_dir: 'desc' };
    if (tierFilter) p.tier = tierFilter;
    if (sevFilter) p.severity = sevFilter;
    if (kevOnly) p.kev = true;
    if (query.trim()) p.q = query.trim();
    return p;
  }, [tierFilter, sevFilter, kevOnly, query]);

  const { data, isLoading } = useVulnerabilities(params);
  const { data: stats } = useVulnStats();
  const items = data?.items ?? [];
  const drawer = drawerId ? items.find((v) => v.id === drawerId) ?? null : null;

  const kpi = (label: string, value: number | string, color: string, Icon: typeof Flame) => (
    <Card style={{ padding: '14px 16px', flex: 1, minWidth: 130 }}>
      <div className="flex items-center gap-2 text-ink-muted text-[11px] font-semibold uppercase tracking-[.04em]">
        <Icon size={13} style={{ color }} /> {label}
      </div>
      <div className="mono text-[24px] font-bold text-ink mt-1" style={{ color }}>{value}</div>
    </Card>
  );

  return (
    <PageFrame wide>
      <PageHeader
        title={tr('Vulnérabilités', 'Vulnerabilities')}
        count={`${stats?.total ?? 0} ${tr('vulnérabilités', 'vulnerabilities')}`}
        actions={
          <>
            <Btn label={tr('Intégrations', 'Integrations')} icon={Plug} onClick={() => setConnectorsOpen(true)} />
            <Btn label={tr('Importer', 'Import')} icon={Upload} primary onClick={() => setIngestOpen(true)} />
          </>
        }
      />

      {/* KPI row */}
      <div className="flex gap-3 mb-4 flex-wrap">
        {kpi(tr('Total', 'Total'), stats?.total ?? 0, 'var(--accent)', Bug)}
        {kpi(tr('Ouvertes', 'Open'), stats?.open ?? 0, 'var(--high)', ShieldAlert)}
        {kpi('P1', stats?.by_tier?.P1 ?? 0, 'var(--critical)', Flame)}
        {kpi('CISA-KEV', stats?.kev_count ?? 0, 'var(--critical)', Flame)}
        {kpi(tr('Exploitables', 'Exploitable'), stats?.exploit_count ?? 0, 'var(--high)', Zap)}
      </div>

      {/* filters */}
      <div className="flex gap-2 mb-3 flex-wrap items-center">
        <Chip label={tr('Toutes', 'All')} active={!tierFilter && !sevFilter && !kevOnly} onClick={() => { setTierFilter(null); setSevFilter(null); setKevOnly(false); }} />
        {(['P1', 'P2', 'P3', 'P4'] as const).map((t) => (
          <Chip key={t} label={t} active={tierFilter === t} onClick={() => setTierFilter(tierFilter === t ? null : t)} color={TIER_META[t].color} />
        ))}
        <span className="w-px h-5 mx-1" style={{ background: 'var(--border-strong)' }} />
        {(['critical', 'high', 'medium', 'low'] as const).map((s) => (
          <Chip key={s} label={pick(SEVERITY_META[s].label, lang)} active={sevFilter === s} onClick={() => setSevFilter(sevFilter === s ? null : s)} color={SEVERITY_META[s].color} />
        ))}
        <Chip label="CISA-KEV" active={kevOnly} onClick={() => setKevOnly((v) => !v)} color="var(--critical)" />
        <button onClick={() => { setShowSearch((v) => !v); if (showSearch) setQuery(''); }} className="ml-auto h-8 px-2.5 rounded-[8px] text-[12.5px] inline-flex items-center gap-1.5" style={{ border: '1px solid var(--border-strong)', color: 'var(--text-secondary)' }}>
          {showSearch ? <X size={14} /> : <Search size={14} />} {tr('Rechercher', 'Search')}
        </button>
      </div>
      {showSearch && (
        <div className="mb-3 flex items-center gap-2.5 h-11 px-3.5 rounded-[12px]" style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}>
          <Search size={16} className="text-ink-muted shrink-0" />
          <input autoFocus value={query} onChange={(e) => setQuery(e.target.value)} placeholder={tr('CVE ou titre…', 'CVE or title…')} className="flex-1 bg-transparent text-[13.5px] text-ink outline-none" />
        </div>
      )}

      <Card style={{ padding: '8px 8px 4px', overflow: 'hidden' }}>
        {isLoading && items.length === 0 ? (
          <SkeletonRows rows={6} />
        ) : items.length === 0 ? (
          <EmptyState
            icon={Bug}
            title={tr('Aucune vulnérabilité', 'No vulnerabilities')}
            sub={tr('Importez des findings depuis Nessus, Qualys, Defender, Inspector, CrowdStrike…', 'Import findings from Nessus, Qualys, Defender, Inspector, CrowdStrike…')}
            cta={<Btn label={tr('Importer', 'Import')} icon={Upload} primary onClick={() => setIngestOpen(true)} />}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse" style={{ minWidth: 900 }}>
              <thead style={{ borderBottom: '1px solid var(--border)' }}>
                <tr>
                  {[tr('Priorité', 'Priority'), 'CVE', tr('Titre', 'Title'), tr('Sévérité', 'Severity'), 'CVSS', tr('Actif', 'Asset'), tr('Source', 'Source'), tr('Statut', 'Status')].map((h) => (
                    <th key={h} className="text-left text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted px-3 pb-[11px]">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {items.map((v) => (
                  <tr key={v.id} onClick={() => setDrawerId(v.id)} className="cursor-pointer transition-colors hover:bg-hover">
                    <td className="px-3 py-[13px]">
                      <div className="inline-flex items-center gap-2">
                        <span className="inline-flex items-center justify-center h-[22px] px-2 rounded-[6px] text-[11.5px] font-bold text-white" style={{ background: TIER_META[v.priority_tier]?.color ?? 'var(--low)' }}>{v.priority_tier}</span>
                        <span className="mono text-[13px] font-bold text-ink">{v.priority_score.toFixed(0)}</span>
                        {v.kev && <Flame size={13} style={{ color: 'var(--critical)' }} />}
                      </div>
                    </td>
                    <td className="px-3 py-[13px]"><span className="mono text-[12.5px] text-ink">{v.cve_id || '—'}</span></td>
                    <td className="px-3 py-[13px]"><div className="text-[13px] text-ink max-w-[280px] truncate">{v.title}</div></td>
                    <td className="px-3 py-[13px]"><SevBadge sev={v.severity} lang={lang} /></td>
                    <td className="px-3 py-[13px]"><span className="mono text-[13px] font-semibold" style={{ color: cvssColor(v.cvss_score) }}>{v.cvss_score ? v.cvss_score.toFixed(1) : '—'}</span></td>
                    <td className="px-3 py-[13px] text-[12.5px] text-ink-soft">{v.asset_name || '—'}</td>
                    <td className="px-3 py-[13px] text-[12px] text-ink-muted">{SOURCE_LABEL[v.source] ?? v.source}</td>
                    <td className="px-3 py-[13px]"><StatusChip status={v.status} lang={lang} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {drawer && <VulnDrawer v={drawer} onClose={() => setDrawerId(null)} />}
      <IngestModal isOpen={ingestOpen} onClose={() => setIngestOpen(false)} />
      <IntegrationsPanel isOpen={connectorsOpen} onClose={() => setConnectorsOpen(false)} onImport={() => { setConnectorsOpen(false); setIngestOpen(true); }} />
    </PageFrame>
  );
}

function SevBadge({ sev, lang }: { sev: Vulnerability['severity']; lang: 'fr' | 'en' }) {
  const m = SEVERITY_META[sev] ?? SEVERITY_META.info;
  return (
    <span className="inline-flex items-center h-[20px] px-2 rounded-full text-[11px] font-semibold" style={{ color: m.color, background: `color-mix(in srgb, ${m.color} 14%, transparent)` }}>
      {pick(m.label, lang)}
    </span>
  );
}

function StatusChip({ status, lang }: { status: VulnStatus; lang: 'fr' | 'en' }) {
  const m = STATUS_META[status];
  return (
    <span className="inline-flex items-center gap-1.5 text-[12px] font-medium" style={{ color: m.color }}>
      <span className="w-1.5 h-1.5 rounded-full" style={{ background: m.color }} /> {pick(m.label, lang)}
    </span>
  );
}

/* ---------------- drawer ---------------- */
function VulnDrawer({ v, onClose }: { v: Vulnerability; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const canWrite = useAuthStore((s) => s.hasPermission('vulnerabilities:update'));
  const canDelete = useAuthStore((s) => s.hasPermission('vulnerabilities:delete'));
  const { updateStatus, createTicket, remove } = useVulnMutations();

  const openTicket = async () => {
    try {
      await createTicket.mutateAsync(v.id);
      toast.success(tr('Ticket ouvert', 'Ticket opened'));
    } catch (e) {
      const msg = e instanceof Error ? e.message : tr('Échec — ITSM configuré ?', 'Failed — is ITSM configured?');
      toast.error(msg);
    }
  };

  const setStatus = async (status: VulnStatus) => {
    try {
      await updateStatus.mutateAsync({ id: v.id, status });
      toast.success(tr('Statut mis à jour', 'Status updated'));
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };
  const del = async () => {
    if (!window.confirm(tr('Supprimer cette vulnérabilité ?', 'Delete this vulnerability?'))) return;
    try {
      await remove.mutateAsync(v.id);
      toast.success(tr('Supprimée', 'Deleted'));
      onClose();
    } catch {
      toast.error(tr('Échec', 'Failed'));
    }
  };

  const field = (lbl: string, val: React.ReactNode) => (
    <div className="mb-3.5">
      <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1">{lbl}</div>
      <div className="text-[13.5px] text-ink">{val}</div>
    </div>
  );
  const signal = (on: boolean, label: string) => (
    <span className="inline-flex items-center gap-1.5 h-[24px] px-2.5 rounded-full text-[12px] font-semibold" style={{ color: on ? 'var(--critical)' : 'var(--text-muted)', background: on ? 'color-mix(in srgb,var(--critical) 12%,transparent)' : 'var(--bg-hover)' }}>
      {label}
    </span>
  );

  return (
    <div className="fixed inset-0 z-[70] flex justify-end" style={{ background: 'rgba(0,0,0,.45)', backdropFilter: 'blur(3px)' }} onClick={onClose}>
      <div onClick={(e) => e.stopPropagation()} className="h-full flex flex-col" style={{ width: 'min(94vw,560px)', background: 'var(--bg-secondary)', borderLeft: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', animation: 'or-slidein .3s cubic-bezier(.2,.8,.2,1)' }}>
        <div className="px-[22px] pt-5 pb-3.5" style={{ borderBottom: '1px solid var(--border)' }}>
          <div className="flex items-start gap-3 mb-3">
            <div className="flex-1">
              <div className="mono text-[12px] text-ink-muted mb-1">{v.cve_id || v.external_id || '—'}</div>
              <div className="disp text-[17px] font-bold text-ink leading-snug">{v.title}</div>
            </div>
            <button onClick={onClose} className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0 text-ink-soft" style={{ background: 'var(--bg-hover)' }}><X size={18} /></button>
          </div>
          <div className="flex items-center gap-2.5 flex-wrap">
            <span className="inline-flex items-center h-[24px] px-2.5 rounded-[7px] text-[12px] font-bold text-white" style={{ background: TIER_META[v.priority_tier]?.color }}>{v.priority_tier} · {v.priority_score.toFixed(0)}</span>
            <SevBadge sev={v.severity} lang={lang} />
            <StatusChip status={v.status} lang={lang} />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-[22px] py-5">
          {/* Prioritisation breakdown */}
          <div className="rounded-[12px] p-4 mb-4" style={{ border: '1px solid color-mix(in srgb,var(--accent) 30%,transparent)', background: 'color-mix(in srgb,var(--accent) 6%,transparent)' }}>
            <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">{tr('Priorisation (risk-based)', 'Prioritisation (risk-based)')}</div>
            <div className="text-[13.5px] text-ink">{v.priority_explanation || tr('Score dérivé du CVSS, de l’exploitabilité, de la criticité métier et des actifs concernés.', 'Score from CVSS, exploitability, business criticality and affected assets.')}</div>
          </div>

          <div className="grid grid-cols-2 gap-x-5">
            {field('CVSS', <span className="mono font-semibold" style={{ color: cvssColor(v.cvss_score) }}>{v.cvss_score ? v.cvss_score.toFixed(1) : '—'}{v.cvss_vector ? ` · ${v.cvss_vector}` : ''}</span>)}
            {field('EPSS', v.epss ? `${(v.epss * 100).toFixed(1)}%` : '—')}
            {field(tr('Actif concerné', 'Affected asset'), v.asset_name ? `${v.asset_name}${v.asset_criticality ? ` · ${v.asset_criticality}` : ''}` : '—')}
            {field(tr('Actifs concernés', 'Affected assets'), String(v.affected_assets_count))}
            {field(tr('Source', 'Source'), SOURCE_LABEL[v.source] ?? v.source)}
            {field(tr('Vu pour la dernière fois', 'Last seen'), v.last_seen ? new Date(v.last_seen).toLocaleDateString() : '—')}
          </div>

          <div className="flex gap-2 flex-wrap mb-4">
            {signal(v.kev, 'CISA-KEV')}
            {signal(v.exploit_available, tr('Exploit public', 'Public exploit'))}
            {v.exploit_maturity ? signal(true, `Maturité: ${v.exploit_maturity}`) : null}
          </div>

          {v.description ? field('Description', <span className="text-ink-soft">{v.description}</span>) : null}
          {v.remediation_hint ? field(tr('Remédiation', 'Remediation'), <span className="text-ink-soft">{v.remediation_hint}</span>) : null}

          {/* Cross-module linkage: ITSM ticket + auto-created risk */}
          <div className="rounded-[12px] p-3.5 mb-4" style={{ border: '1px solid var(--border)' }}>
            <div className="flex items-center gap-2 mb-2">
              <Ticket size={14} className="text-ink-muted" />
              <span className="text-[12.5px] font-semibold text-ink">{tr('Ticket ITSM', 'ITSM ticket')}</span>
            </div>
            {v.ticket_key ? (
              <a href={v.ticket_url || '#'} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 text-[13px] font-semibold" style={{ color: 'var(--accent)' }}>
                {v.ticket_key} <ExternalLink size={13} />
              </a>
            ) : canWrite ? (
              <button onClick={openTicket} disabled={createTicket.isPending} className="h-8 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: 'var(--accent)' }}>
                <Ticket size={13} /> {tr('Ouvrir un ticket', 'Open a ticket')}
              </button>
            ) : (
              <span className="text-[12.5px] text-ink-muted">{tr('Aucun ticket', 'No ticket')}</span>
            )}
            {v.risk_id && (
              <div className="mt-2.5 pt-2.5 flex items-center gap-1.5 text-[12px] text-ink-soft" style={{ borderTop: '1px solid var(--border)' }}>
                <ShieldAlert size={13} style={{ color: 'var(--high)' }} />
                {tr('Risque auto-créé lié', 'Linked auto-created risk')}
              </div>
            )}
          </div>

          {/* Status lifecycle */}
          {canWrite && (
            <div className="mt-2">
              <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">{tr('Changer le statut', 'Change status')}</div>
              <div className="flex flex-wrap gap-2">
                {STATUS_ORDER.filter((s) => s !== v.status).map((s) => (
                  <button key={s} disabled={updateStatus.isPending} onClick={() => setStatus(s)} className="h-8 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5 disabled:opacity-60" style={{ border: '1px solid var(--border-strong)', color: STATUS_META[s].color }}>
                    <ChevronRight size={13} /> {pick(STATUS_META[s].label, lang)}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>

        {canDelete && (
          <div className="px-[22px] py-3.5" style={{ borderTop: '1px solid var(--border)' }}>
            <button onClick={del} className="h-9 px-3 rounded-[9px] text-[12.5px] font-semibold inline-flex items-center gap-1.5" style={{ color: 'var(--critical)', background: 'color-mix(in srgb,var(--critical) 12%,transparent)' }}>
              <Trash2 size={14} /> {tr('Supprimer', 'Delete')}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
