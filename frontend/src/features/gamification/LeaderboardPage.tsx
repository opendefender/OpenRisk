// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Risk Champions Leaderboard (OpenRisk.dc.html §6.8).
//
// There is no ranking backend: the only gamification endpoint is
// /gamification/me, which returns the caller's own profile and has no notion of
// a tenant-wide standing. This page therefore shows what the leaderboard will
// rank and how points are earned — it does not invent a standing.
//
// It previously rendered seven named colleagues (Fatou Sy, Amir Diallo, ...)
// with points, badges and streaks, plus a "your position — #2, Amir Diallo"
// card, on every tenant. Blurring invented people behind an upsell does not
// make them less invented, so the fixtures are gone rather than obscured.

import { Trophy, Sparkles } from 'lucide-react';
import { toast } from 'sonner';
import { useNavigate } from 'react-router';
import { PageFrame, PageHeader, Card, Btn, PreviewBadge } from '../../shared/ui';
import { EmptyState } from '../../shared/EmptyState';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';

/** How a champion earns points once ranking ships. Describes the scoring rules,
 *  which are real product intent — not standings, which would be fabricated. */
const SCORING: [string, string][] = [
  ['risks_treated', 'mitigations'],
  ['evidence_filed', 'compliance'],
  ['incidents_resolved', 'incidents'],
];

export function LeaderboardPage() {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);

  const rules: [string, string, string][] = [
    [tr('Risques traités', 'Risks treated'), tr('Faire passer un risque en « traité » avec un plan documenté.', 'Move a risk to "treated" with a documented plan.'), '/risks'],
    [tr('Preuves déposées', 'Evidence filed'), tr('Attacher une preuve valide à un contrôle de conformité.', 'Attach valid evidence to a compliance control.'), '/compliance'],
    [tr('Incidents résolus', 'Incidents resolved'), tr('Clore un incident dans les délais de son SLA.', 'Close an incident within its SLA.'), '/incidents'],
  ];

  return (
    <PageFrame wide>
      <PageHeader title={L.lbTitle} badge={<PreviewBadge label={tr('Bientôt', 'Coming soon')} />} />
      <div className="text-[13.5px] text-ink-soft -mt-2 mb-5">{L.lbSub}</div>

      <Card style={{ padding: '8px 4px 24px' }}>
        <EmptyState
          variant="first-use"
          icon={Trophy}
          title={tr('Le classement n’est pas encore actif', 'Rankings are not live yet')}
          description={tr(
            'Le classement des Champions du risque récompensera les membres qui réduisent réellement l’exposition de l’organisation. Il s’activera dès que le calcul des points sera déployé — aucun classement n’est affiché tant qu’il n’est pas réel.',
            'The Risk Champions leaderboard will reward the members who genuinely reduce the organisation’s exposure. It switches on once point scoring ships — no standing is shown until it is a real one.',
          )}
          primaryAction={
            <Btn
              label={tr('M’avertir au lancement', 'Notify me at launch')}
              icon={Sparkles}
              primary
              onClick={() => toast(tr('Nous vous préviendrons dès que le classement sera actif.', 'We will let you know as soon as rankings go live.'), { icon: '🏆' })}
            />
          }
        />

        <div className="px-6">
          <div className="text-[13px] font-semibold text-ink mb-3">{tr('Comment les points seront gagnés', 'How points will be earned')}</div>
          <div className="flex flex-col gap-2">
            {rules.map(([name, how, href], i) => (
              <button
                key={SCORING[i][0]}
                onClick={() => navigate(href)}
                className="flex items-center gap-3 rounded-[10px] px-3 py-2.5 text-left hover:bg-hover transition-colors"
                style={{ border: '1px solid var(--border)' }}
              >
                <div className="w-8 h-8 rounded-[9px] flex items-center justify-center shrink-0" style={{ background: 'var(--accent-soft)', color: 'var(--accent-500)' }}>
                  <Trophy size={15} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-[13px] font-medium text-ink">{name}</div>
                  <div className="text-[11.5px] text-ink-muted">{how}</div>
                </div>
              </button>
            ))}
          </div>
        </div>
      </Card>
    </PageFrame>
  );
}
