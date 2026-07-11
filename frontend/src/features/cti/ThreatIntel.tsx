// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Threat Intel (OpenRisk.dc.html §6.11): 4 headline stats + a CVE feed list
// (CVSS score chip, title/asset, date, status pill).

import { Globe } from 'lucide-react';
import { PageFrame, PageHeader, Btn, Card, PreviewBadge } from '../../shared/ui';
import { critColor, type Criticality } from '../../shared/riskColors';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

export function ThreatIntel() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const cves: [string, string, number, Criticality, string, string, string][] = [
    ['CVE-2024-4032', tr('Contournement d’authentification RDP', 'RDP authentication bypass'), 9.8, 'critical', 'srv-paie-01', tr('il y a 2h', '2h ago'), 'open'],
    ['CVE-2024-3812', tr('Élévation de privilèges IAM', 'IAM privilege escalation'), 8.6, 'high', 'aws-prod', tr('il y a 6h', '6h ago'), 'investigating'],
    ['CVE-2024-3567', tr('Déni de service TLS', 'TLS denial of service'), 7.5, 'high', 'gw-bank-02', tr('hier', 'yesterday'), 'open'],
    ['CVE-2024-2991', tr('Injection SQL portail', 'Portal SQL injection'), 6.4, 'medium', 'portail-client', tr('il y a 2j', '2d ago'), 'patched'],
    ['CVE-2024-2210', tr('Fuite mémoire Redis', 'Redis memory leak'), 5.1, 'medium', 'redis-cache', tr('il y a 3j', '3d ago'), 'patched'],
    ['CVE-2024-1877', tr('Lecture de fichier arbitraire', 'Arbitrary file read'), 4.3, 'low', 'www-public', tr('il y a 5j', '5d ago'), 'accepted'],
  ];
  const stMap: Record<string, [string, string]> = {
    open: ['var(--critical)', tr('Ouvert', 'Open')], investigating: ['var(--high)', tr('En analyse', 'Investigating')],
    patched: ['var(--low)', tr('Corrigé', 'Patched')], accepted: ['var(--text-muted)', tr('Accepté', 'Accepted')],
  };
  const stats: [string, string, string][] = [
    ['3', tr('Nouvelles · 24h', 'New · 24h'), 'var(--accent)'], ['2', tr('Critiques ouvertes', 'Open critical'), 'var(--critical)'],
    ['8', tr('CVE actives', 'Active CVEs'), 'var(--high)'], ['12', tr('Correctifs dispo.', 'Patches available'), 'var(--low)'],
  ];

  return (
    <PageFrame>
      <PageHeader title={L.n_cti} badge={<PreviewBadge label={tr('Aperçu', 'Preview')} />} actions={<Btn label={tr('Synchroniser', 'Sync feed')} icon={Globe} />} />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-4">
        {stats.map(([v, lbl, col]) => (
          <Card key={lbl} style={{ padding: '16px 18px' }}>
            <div className="disp mono text-[28px] font-bold" style={{ color: col }}>{v}</div>
            <div className="text-[12.5px] text-ink-soft mt-1">{lbl}</div>
          </Card>
        ))}
      </div>
      <Card style={{ padding: '8px 14px' }}>
        {cves.map(([id, title, cvss, crit, asset, date, st], i) => (
          <div key={id} className="flex items-center gap-4 py-3.5 px-2 rounded-lg cursor-pointer hover:bg-hover transition-colors" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
            <div className="w-[52px] shrink-0 text-center">
              <div className="mono text-[17px] font-bold" style={{ color: critColor[crit] }}>{cvss.toFixed(1)}</div>
              <div className="text-[9px] text-ink-muted tracking-[.04em]">CVSS</div>
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-[13.5px] font-medium text-ink">{title}</div>
              <div className="mono text-[11.5px] text-ink-muted mt-0.5">{id} · {asset}</div>
            </div>
            <span className="text-[11.5px] text-ink-soft whitespace-nowrap hidden sm:inline">{date}</span>
            <span className="inline-flex items-center gap-1.5 text-[12px] font-semibold w-[110px]" style={{ color: stMap[st][0] }}>
              <span className="w-[7px] h-[7px] rounded-full" style={{ background: stMap[st][0] }} />{stMap[st][1]}
            </span>
          </div>
        ))}
      </Card>
    </PageFrame>
  );
}
