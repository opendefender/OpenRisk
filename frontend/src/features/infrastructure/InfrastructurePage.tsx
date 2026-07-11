// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Infrastructure (OpenRisk.dc.html §6.13): 3 environment cards (count + health
// bar) and a services availability list with operational/degraded/incident dots.

import { Atom } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

export function InfrastructurePage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const envs: [string, number, number, string][] = [
    [tr('Production', 'Production'), 142, 96, 'var(--low)'],
    ['Staging', 38, 84, 'var(--high)'],
    [tr('Développement', 'Development'), 22, 71, 'var(--medium)'],
  ];
  const services: [string, string, string][] = [
    [tr('Passerelle bancaire', 'Banking gateway'), 'operational', '99.98%'],
    [tr('API Portail client', 'Client portal API'), 'operational', '99.94%'],
    [tr('Base de données core', 'Core database'), 'degraded', '99.2%'],
    [tr('Service de paie', 'Payroll service'), 'incident', '—'],
    [tr('Authentification SSO', 'SSO authentication'), 'operational', '100%'],
    [tr('Sauvegardes NAS', 'NAS backups'), 'degraded', '—'],
  ];
  const stMap: Record<string, [string, string]> = {
    operational: ['var(--low)', tr('Opérationnel', 'Operational')],
    degraded: ['var(--high)', tr('Dégradé', 'Degraded')],
    incident: ['var(--critical)', tr('Incident', 'Incident')],
  };

  return (
    <PageFrame>
      <PageHeader title={L.n_infra} actions={<Btn label={tr('Vue Univers', 'Universe view')} icon={Atom} onClick={() => navigate('/assets/universe')} />} />
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-4">
        {envs.map(([name, count, health, col]) => (
          <Card key={name} style={{ padding: '18px 20px' }}>
            <div className="flex items-center justify-between mb-4">
              <span className="text-[14px] font-semibold text-ink">{name}</span>
              <span className="mono text-[12px] font-semibold" style={{ color: col }}>{health}%</span>
            </div>
            <div className="flex items-baseline gap-1.5 mb-3.5">
              <span className="disp mono text-[30px] font-bold text-ink">{count}</span>
              <span className="text-[12.5px] text-ink-soft">{L.uniAssets}</span>
            </div>
            <div className="h-1.5 rounded-md overflow-hidden" style={{ background: 'var(--bg-hover)' }}>
              <div className="h-full rounded-md" style={{ width: `${health}%`, background: col, transition: 'width .8s ease' }} />
            </div>
          </Card>
        ))}
      </div>
      <Card style={{ padding: '18px 22px' }}>
        <div className="text-[14px] font-semibold text-ink mb-3.5">{tr('Services & disponibilité', 'Services & uptime')}</div>
        {services.map(([name, st, up], i) => (
          <div key={name} className="flex items-center gap-3 py-3 px-1" style={{ borderTop: i ? '1px solid var(--border)' : 'none' }}>
            <span className="w-[9px] h-[9px] rounded-full shrink-0" style={{ background: stMap[st][0], boxShadow: st === 'incident' ? '0 0 7px var(--critical)' : 'none' }} />
            <span className="flex-1 text-[13.5px] font-medium text-ink">{name}</span>
            <span className="text-[12px] font-semibold" style={{ color: stMap[st][0] }}>{stMap[st][1]}</span>
            <span className="mono text-[12.5px] text-ink-soft w-16 text-right">{up}</span>
          </div>
        ))}
      </Card>
    </PageFrame>
  );
}
