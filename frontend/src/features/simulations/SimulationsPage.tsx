// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Simulations — Risk Digital Twin (OpenRisk.dc.html §6.7).
//
// There is no simulation engine behind this screen: no endpoint runs a scenario,
// and nothing persists a run. The page previously showed an 8.4/10 impact gauge
// for a "last run 2 days ago" that never happened, a blast radius across four
// named assets, three AI-suggested scenarios and a three-entry run history — all
// literals, identical in every tenant. Its "Run" button started a 1.9s spinner
// and then did nothing.
//
// Until the engine exists, the page explains what a simulation will do and sends
// the user to the analyses that are real today (asset dependencies, financial
// quantification). No gauge, no history, no invented blast radius.

import { Cpu, Atom, Coins } from 'lucide-react';
import { useNavigate } from 'react-router';
import { PageFrame, PageHeader, Card, PreviewBadge } from '../../shared/ui';
import { EmptyState } from '../../shared/EmptyState';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

export function SimulationsPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  // Real analyses that answer part of the same question today.
  const alternatives: [typeof Atom, string, string, string][] = [
    [
      Atom,
      tr('Cartographie des dépendances', 'Dependency map'),
      tr(
        'Voir quels actifs tombent avec un actif compromis.',
        'See which assets fall with a compromised one.',
      ),
      '/assets/topology',
    ],
    [
      Coins,
      tr('Quantification financière', 'Financial quantification'),
      tr(
        'Chiffrer la perte annualisée d’un risque (ALE, pire cas).',
        'Quantify a risk’s annualised loss (ALE, worst case).',
      ),
      '/analytics/financial',
    ],
  ];

  return (
    <PageFrame>
      <PageHeader
        title={L.simTitle}
        badge={<PreviewBadge label={tr('Bientôt', 'Coming soon')} />}
      />
      <div className="text-[13.5px] text-ink-soft -mt-2 mb-5">{L.simSub}</div>

      <Card style={{ padding: '8px 4px 24px' }}>
        <EmptyState
          variant="first-use"
          icon={Cpu}
          title={tr('Le moteur de simulation arrive', 'The simulation engine is on its way')}
          description={tr(
            'Les simulations rejoueront un scénario d’attaque sur votre patrimoine réel pour estimer sa portée et son coût avant qu’il ne se produise. Le moteur n’est pas encore déployé — aucun résultat n’est affiché tant qu’il n’est pas calculé sur vos données.',
            'Simulations will replay an attack scenario against your real estate to estimate its reach and cost before it happens. The engine is not deployed yet — no result is shown until it is computed from your own data.',
          )}
        />

        <div className="px-6">
          <div className="text-[13px] font-semibold text-ink mb-3">
            {tr('En attendant', 'In the meantime')}
          </div>
          <div
            className="grid gap-3"
            style={{ gridTemplateColumns: 'repeat(auto-fill,minmax(260px,1fr))' }}
          >
            {alternatives.map(([Icon, title, desc, href]) => (
              <button
                key={href}
                onClick={() => navigate(href)}
                className="text-left rounded-[12px] p-4 hover:bg-hover transition-colors"
                style={{ border: '1px solid var(--border)' }}
              >
                <div
                  className="w-9 h-9 rounded-[10px] flex items-center justify-center mb-3"
                  style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}
                >
                  <Icon size={17} />
                </div>
                <div className="text-[13.5px] font-semibold text-ink mb-1">{title}</div>
                <div className="text-[12px] text-ink-soft leading-relaxed">{desc}</div>
              </button>
            ))}
          </div>
        </div>
      </Card>
    </PageFrame>
  );
}
