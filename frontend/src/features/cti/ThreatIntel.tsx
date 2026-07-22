// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Threat Intel (OpenRisk.dc.html §6.11): headline stats + a LIVE CVE feed from
// NVD + CISA KEV (enriched with MITRE ATT&CK). "Sync feed" pulls the sources;
// "Match assets" intersects CPEs and auto-creates risks.

import { useMemo, useState } from 'react';
import { Globe, Crosshair, ShieldAlert, Search } from 'lucide-react';
import { toast } from 'sonner';
import { PageFrame, PageHeader, Btn, Card, SkeletonRows, EmptyState } from '../../shared/ui';
import { critColor, type Criticality } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useCTIStats, useCTIVulnerabilities, useCTISync, useCTIMatch } from './useCTI';
import type { CTIVulnerability } from './ctiService';

const sevToCrit = (s: string): Criticality => {
  switch ((s || '').toUpperCase()) {
    case 'CRITICAL': return 'critical';
    case 'HIGH': return 'high';
    case 'MEDIUM': return 'medium';
    default: return 'low';
  }
};

function relTime(iso: string, tr: (fr: string, en: string) => string): string {
  const t = new Date(iso).getTime();
  if (!t) return '—';
  const mins = Math.max(0, Math.floor((Date.now() - t) / 60000));
  if (mins < 60) return tr(`il y a ${mins} min`, `${mins}m ago`);
  const h = Math.floor(mins / 60);
  if (h < 24) return tr(`il y a ${h} h`, `${h}h ago`);
  const d = Math.floor(h / 24);
  return tr(`il y a ${d} j`, `${d}d ago`);
}

export function ThreatIntel() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const [q, setQ] = useState('');
  const [sev, setSev] = useState('');
  const { data: stats } = useCTIStats();
  const { data: vulns, isLoading, isError, refetch } = useCTIVulnerabilities({
    query: q || undefined,
    severity: sev || undefined,
    limit: 60,
  });
  const sync = useCTISync();
  const match = useCTIMatch();

  const onSync = () => {
    sync.mutate(undefined, {
      onSuccess: (r) => toast.success(tr(`Flux synchronisé — ${r.total_vulnerabilities} CVE`, `Feed synced — ${r.total_vulnerabilities} CVEs`)),
      onError: () => toast.error(tr('Échec de la synchronisation', 'Sync failed')),
    });
  };
  const onMatch = () => {
    match.mutate(undefined, {
      onSuccess: (r) => toast.success(
        r.risks_created > 0
          ? tr(`${r.risks_created} risque(s) créé(s) depuis les CVE`, `${r.risks_created} risk(s) created from CVEs`)
          : tr('Aucune nouvelle exposition détectée', 'No new exposure detected'),
      ),
      onError: () => toast.error(tr('Échec du matching', 'Matching failed')),
    });
  };

  const statCards: [string, string, string][] = useMemo(() => ([
    [String(stats?.total ?? 0), tr('CVE actives', 'Active CVEs'), 'var(--accent)'],
    [String(stats?.critical ?? 0), tr('Critiques', 'Critical'), 'var(--critical)'],
    [String(stats?.cisa_known ?? 0), tr('CISA KEV (exploitées)', 'CISA KEV (exploited)'), 'var(--high)'],
    [String(stats?.cti_risks ?? 0), tr('Risques auto-créés', 'Auto-created risks'), 'var(--low)'],
  ]), [stats, lang]); // eslint-disable-line react-hooks/exhaustive-deps

  const list: CTIVulnerability[] = vulns ?? [];

  return (
    <PageFrame>
      <PageHeader
        title={L.n_cti}
        count={stats ? String(stats.total) : null}
        actions={
          <div className="flex items-center gap-2">
            <Btn label={tr('Matcher les actifs', 'Match assets')} icon={Crosshair} onClick={onMatch} />
            <Btn label={tr('Synchroniser', 'Sync feed')} icon={Globe} primary onClick={onSync} />
          </div>
        }
      />

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
        {statCards.map(([v, lbl, col]) => (
          <Card key={lbl} style={{ padding: '16px 18px' }}>
            <div className="disp mono text-[28px] font-bold" style={{ color: col }}>{v}</div>
            <div className="text-[12.5px] text-ink-soft mt-1">{lbl}</div>
          </Card>
        ))}
      </div>

      {/* Search / filter */}
      <div className="flex flex-wrap items-center gap-2 mb-3">
        <div className="relative flex-1 min-w-[220px]">
          <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-ink-muted" />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={tr('Rechercher CVE / description…', 'Search CVE / description…')}
            className="w-full h-9 pl-9 pr-3 rounded-[10px] text-[13px] bg-surface border border-[var(--border)] text-ink focus:outline-none focus:ring-2 focus:ring-[var(--accent)]/40"
          />
        </div>
        {['', 'CRITICAL', 'HIGH', 'MEDIUM'].map((s) => (
          <button
            key={s || 'all'}
            onClick={() => setSev(s)}
            className="h-9 px-3 rounded-[10px] text-[12.5px] font-semibold transition-colors"
            style={sev === s
              ? { background: 'var(--accent)', color: '#fff' }
              : { background: 'var(--bg-hover)', color: 'var(--text-soft)' }}
          >
            {s === '' ? tr('Toutes', 'All') : s}
          </button>
        ))}
      </div>

      {/* Feed */}
      <Card style={{ padding: '8px 14px' }}>
        {isLoading ? (
          <SkeletonRows rows={6} />
        ) : isError ? (
          <EmptyState
            icon={ShieldAlert}
            title={tr('Impossible de charger le flux', 'Could not load the feed')}
            sub={tr('Réessayez ou synchronisez le flux.', 'Retry or sync the feed.')}
            cta={<Btn label={tr('Réessayer', 'Retry')} onClick={() => refetch()} />}
          />
        ) : list.length === 0 ? (
          <EmptyState
            icon={Globe}
            title={tr('Aucune CVE', 'No CVEs yet')}
            sub={tr('Synchronisez NVD + CISA KEV pour peupler le flux.', 'Sync NVD + CISA KEV to populate the feed.')}
            cta={<Btn label={tr('Synchroniser', 'Sync feed')} icon={Globe} primary onClick={onSync} />}
          />
        ) : (
          list.map((v, i) => {
            const crit = sevToCrit(v.severity);
            const techniques = (v.mitre_techniques ?? []).slice(0, 3);
            return (
              <div key={v.cve_id} className="flex items-center gap-4 py-3.5 px-2 rounded-lg hover:bg-hover transition-colors" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
                <div className="w-[52px] shrink-0 text-center">
                  <div className="mono text-[17px] font-bold" style={{ color: critColor[crit] }}>{v.cvss_v3 > 0 ? v.cvss_v3.toFixed(1) : '—'}</div>
                  <div className="text-[9px] text-ink-muted tracking-[.04em]">CVSS</div>
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[13.5px] font-medium text-ink truncate">{v.description || v.cve_id}</div>
                  <div className="flex items-center gap-2 mt-1 flex-wrap">
                    <span className="mono text-[11.5px] text-ink-muted">{v.cve_id}</span>
                    {v.cisa_known && (
                      <span className="inline-flex items-center gap-1 text-[10px] font-bold px-1.5 py-0.5 rounded" style={{ background: 'color-mix(in srgb, var(--critical) 16%, transparent)', color: 'var(--critical)' }}>
                        <ShieldAlert size={10} /> CISA KEV
                      </span>
                    )}
                    {techniques.map((t) => (
                      <span key={t} className="mono text-[10px] font-semibold px-1.5 py-0.5 rounded" style={{ background: 'var(--bg-hover)', color: 'var(--text-soft)' }}>{t}</span>
                    ))}
                  </div>
                </div>
                <span className="text-[11.5px] text-ink-soft whitespace-nowrap hidden sm:inline">{relTime(v.published_at, tr)}</span>
                <span className="inline-flex items-center gap-1.5 text-[12px] font-semibold w-[92px]" style={{ color: critColor[crit] }}>
                  <span className="w-[7px] h-[7px] rounded-full" style={{ background: critColor[crit] }} />
                  {v.severity ? v.severity.charAt(0) + v.severity.slice(1).toLowerCase() : '—'}
                </span>
              </div>
            );
          })
        )}
      </Card>
    </PageFrame>
  );
}
