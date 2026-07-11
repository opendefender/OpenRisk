// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: BUSL-1.1
//
// Compliance (OpenRisk.dc.html §6.10): posture hero (overall radial gauge + copy
// + CTAs) and a responsive grid of framework cards each with a mini radial gauge.

import { FileText, AlertTriangle } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { PageFrame, PageHeader, Btn, Card, RingGauge } from '../../shared/ui';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

export function ComplianceScreen() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const fws: [string, number, number, number, string][] = [
    ['ISO 27001', 82, 94, 114, '#7c6cff'], ['SOC 2', 76, 61, 80, '#64d2ff'], ['NIST CSF', 68, 72, 106, '#0a84ff'],
    ['DORA', 54, 41, 76, '#ff2d92'], ['BCEAO', 88, 53, 60, '#30d158'], ['ANSSI', 71, 48, 68, '#ff9f0a'],
  ];
  const overall = Math.round(fws.reduce((a, f) => a + f[1], 0) / fws.length);

  return (
    <PageFrame>
      <PageHeader title={L.n_compliance} />
      <Card style={{ padding: '22px 24px', marginBottom: 16 }}>
        <div className="flex items-center gap-6 flex-wrap">
          <RingGauge value={overall} size={128} color="var(--low)">
            <span className="disp mono text-[32px] font-bold text-ink">{overall}%</span>
            <span className="text-[11px] text-ink-muted">{tr('conforme', 'compliant')}</span>
          </RingGauge>
          <div className="flex-1 min-w-[280px]">
            <div className="disp text-[19px] font-bold text-ink mb-1.5">{tr('Posture de conformité', 'Compliance posture')}</div>
            <div className="text-[13.5px] text-ink-soft leading-relaxed mb-3.5 max-w-[520px]">
              {tr('369 contrôles suivis sur 6 référentiels. 42 contrôles requièrent une action avant le prochain audit (18 sept. 2026).', '369 controls tracked across 6 frameworks. 42 controls need action before the next audit (Sep 18, 2026).')}
            </div>
            <div className="flex gap-2.5 flex-wrap">
              <Btn label={L.genReport} icon={FileText} primary onClick={() => navigate('/reports')} />
              <Btn label={tr('Voir les écarts', 'View gaps')} icon={AlertTriangle} />
            </div>
          </div>
        </div>
      </Card>
      <div className="grid gap-4" style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(240px,1fr))' }}>
        {fws.map(([name, pct, passed, total, col]) => (
          <Card key={name} style={{ padding: 18, cursor: 'pointer' }}>
            <div className="flex items-center gap-3.5">
              <RingGauge value={pct} size={56} color={col} thickness={6}>
                <span className="mono text-[13px] font-bold text-ink">{pct}</span>
              </RingGauge>
              <div className="flex-1">
                <div className="text-[14px] font-semibold text-ink">{name}</div>
                <div className="text-[12px] text-ink-soft mt-0.5">{passed} / {total} {tr('contrôles', 'controls')}</div>
              </div>
            </div>
          </Card>
        ))}
      </div>
    </PageFrame>
  );
}
