// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// <ScoreWorking> (#486) — a risk score with its working shown.
//
// Everything here is rendered from GET /risks/:id/score-working; the component
// computes nothing. It shows:
//
//   • the terms of the frozen formula and what the Score Engine makes of them;
//   • the stored score, and says so plainly when it does not equal the working
//     rather than showing a number the reader cannot re-derive;
//   • for each term, the audit entry that last set it — who, when, its position
//     in the chain and its hash — and whether that entry still verifies;
//   • when no entry exists, that none exists; when the reader may not see the
//     trail, that it was withheld. Neither is dressed up as the other.

import { Link } from 'react-router';
import { ShieldAlert, ShieldCheck, AlertTriangle } from 'lucide-react';

import { SkeletonRows, ErrorState } from '../../../shared/ui';
import { critColor } from '../../../shared/riskColors';
import type { Criticality } from '../../../shared/riskColors';
import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { relTime } from '../riskMap';
import { useScoreWorking } from '../useScoreWorking';
import type {
  ScoreTermKey,
  ScoreWorkingAsset,
  ScoreWorkingSource,
  ScoreWorkingTerm,
} from '../scoreWorkingService';

const AUDIT_READ = 'governance:audit:read';

function fmt(n: number, digits: number): string {
  return n.toFixed(digits);
}

function shortHash(h: string): string {
  return h.length > 12 ? `${h.slice(0, 12)}…` : h;
}

function asCrit(c: string): Criticality | undefined {
  const k = c.toLowerCase();
  return k === 'low' || k === 'medium' || k === 'high' || k === 'critical'
    ? (k as Criticality)
    : undefined;
}

export function ScoreWorking({ riskId, storedScore }: { riskId: string; storedScore: number }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const canVerify = useAuthStore((s) => s.hasPermission(AUDIT_READ));
  const { data, isLoading, error, refetch, isFetching } = useScoreWorking(riskId, storedScore);

  if (isLoading) {
    return (
      <div data-testid="score-working-loading">
        <SkeletonRows rows={4} />
      </div>
    );
  }
  if (error || !data) {
    return (
      <ErrorState
        title={tr(
          'Impossible d’afficher le détail du calcul.',
          'Could not load the score working.',
        )}
        onRetry={() => void refetch()}
        retrying={isFetching}
        retryLabel={tr('Réessayer', 'Retry')}
      />
    );
  }

  const termLabel: Record<ScoreTermKey, string> = {
    probability: tr('Probabilité', 'Probability'),
    impact: tr('Impact', 'Impact'),
    asset_criticality: tr('Criticité de l’actif', 'Asset criticality'),
  };
  const digits: Record<ScoreTermKey, number> = {
    probability: 3,
    impact: 1,
    asset_criticality: 2,
  };
  const band = asCrit(data.criticality);

  const actorOf = (s: ScoreWorkingSource): string => {
    if (s.actor_type === 'job')
      return tr(`tâche ${s.actor_label ?? ''}`, `job ${s.actor_label ?? ''}`);
    if (s.actor_type === 'unattributed')
      return tr('écriture sans identité transmise', 'write that carried no identity');
    const who = s.actor_email || (s.actor_id ? s.actor_id.slice(0, 8) : '');
    if (s.actor_type === 'service_token') {
      const tok = s.actor_label ? s.actor_label.slice(0, 8) : '';
      return tr(`jeton ${tok} de ${who}`, `token ${tok} of ${who}`);
    }
    return who || tr('acteur non enregistré', 'actor not recorded');
  };

  const sourceLine = (source: ScoreWorkingSource | null, testid: string) => {
    if (!data.sources_visible) return null;
    if (!source) {
      return (
        <div className="text-[12px] text-ink-muted mt-1" data-testid={`${testid}-none`}>
          {tr(
            'Aucune entrée du journal ne porte cette valeur : elle précède la journalisation de ce champ.',
            'No journal entry carries this value: it predates journalling of this field.',
          )}
        </div>
      );
    }
    return (
      <div
        className="text-[12px] text-ink-soft mt-1 flex items-center gap-1.5 flex-wrap"
        data-testid={testid}
      >
        {source.hash_valid ? (
          <ShieldCheck size={13} aria-hidden style={{ color: 'var(--low)' }} />
        ) : (
          <ShieldAlert size={13} aria-hidden style={{ color: 'var(--critical)' }} />
        )}
        <span>
          {tr('Fixé par', 'Set by')} <strong className="text-ink">{actorOf(source)}</strong>
        </span>
        <span className="text-ink-muted">·</span>
        <time dateTime={source.at} title={new Date(source.at).toISOString()}>
          {relTime(source.at, lang)}
        </time>
        <span className="text-ink-muted">·</span>
        <span className="mono">
          {tr('entrée', 'entry')} #{source.sequence} {shortHash(source.hash)}
        </span>
        {!source.hash_valid && (
          <strong style={{ color: 'var(--critical)' }}>
            {tr('— entrée altérée depuis son scellement', '— entry altered since it was sealed')}
          </strong>
        )}
      </div>
    );
  };

  const termRow = (term: ScoreWorkingTerm) => {
    const recorded = term.source?.value;
    const drift =
      typeof recorded === 'number' &&
      Math.abs(recorded - term.value) > 0.0005 &&
      term.key !== 'asset_criticality';
    return (
      <li key={term.key} className="py-3" style={{ borderTop: '1px solid var(--border)' }}>
        <div className="flex items-baseline justify-between gap-3">
          <span className="text-[13px] font-medium text-ink">{termLabel[term.key]}</span>
          <span className="mono text-[13px] text-ink">
            {fmt(term.value, digits[term.key])}
            <span className="text-ink-muted text-[11px] ml-1.5">
              [{term.min}–{term.max}]
            </span>
          </span>
        </div>
        {term.key === 'asset_criticality'
          ? assetSources()
          : sourceLine(term.source, `score-working-source-${term.key}`)}
        {drift && (
          <div className="text-[12px] mt-1" style={{ color: 'var(--high)' }}>
            {tr(
              `La dernière entrée du journal fixait ${String(recorded)} : la valeur actuelle a changé sans entrée.`,
              `The last journal entry set ${String(recorded)}: the current value changed without an entry.`,
            )}
          </div>
        )}
      </li>
    );
  };

  const assetSources = () => {
    if (data.asset_criticality_defaulted) {
      return (
        <div className="text-[12px] text-ink-muted mt-1">
          {tr(
            'Aucun actif lié : le moteur applique sa valeur par défaut (criticité moyenne).',
            'No linked asset: the engine applies its default (medium criticality).',
          )}
        </div>
      );
    }
    return (
      <ul className="mt-1.5 space-y-1.5">
        {data.assets.map((a: ScoreWorkingAsset) => (
          <li key={a.id} className="text-[12px]">
            <div className="flex justify-between gap-3 text-ink-soft">
              <span>{a.name}</span>
              <span className="mono">
                {a.criticality} → {fmt(a.factor, 1)}
              </span>
            </div>
            {sourceLine(a.source, `score-working-asset-${a.id}`)}
          </li>
        ))}
        {data.assets.length > 1 && (
          <li className="text-[11.5px] text-ink-muted">
            {tr('Moyenne des facteurs des actifs liés.', 'Average of the linked assets’ factors.')}
          </li>
        )}
      </ul>
    );
  };

  return (
    <section aria-labelledby="score-working-title" data-testid="score-working">
      <h3 id="score-working-title" className="text-[13px] font-semibold text-ink mb-2">
        {tr('Le calcul', 'The working')}
      </h3>

      <div
        className="text-center p-[18px] rounded-[14px] mono text-[15px] text-ink-soft"
        style={{ background: 'var(--bg-hover)' }}
        data-testid="score-working-equation"
      >
        {data.terms.map((t, i) => (
          <span key={t.key}>
            {i > 0 && <span className="mx-2 text-ink-muted">×</span>}
            <span>{fmt(t.value, digits[t.key])}</span>
          </span>
        ))}
        <span className="mx-2.5 text-ink-muted">=</span>
        <span
          className="text-[22px] font-bold"
          style={{ color: band ? critColor[band] : 'var(--fg-primary)' }}
        >
          {fmt(data.computed, 3)}
        </span>
        <div className="text-[12px] text-ink-muted mt-2 font-sans">
          {tr(
            'Probabilité × Impact × Criticité de l’actif — seuils : critique ≥ 7 · élevé ≥ 4 · moyen ≥ 2',
            'Probability × Impact × Asset criticality — bands: critical ≥ 7 · high ≥ 4 · medium ≥ 2',
          )}
        </div>
      </div>

      {!data.consistent && (
        <div
          role="status"
          className="flex gap-2 items-start text-[12.5px] mt-3 p-3 rounded-[10px]"
          style={{ background: 'var(--bg-hover)', color: 'var(--high)' }}
          data-testid="score-working-inconsistent"
        >
          <AlertTriangle size={15} aria-hidden className="shrink-0 mt-px" />
          <span>
            {tr(
              `Le score enregistré (${fmt(data.stored, 3)}) ne correspond pas au calcul à partir des termes actuels (${fmt(data.computed, 3)}). Le moteur de score ne l’a pas encore recalculé, ou il a été écrit par un autre chemin.`,
              `The stored score (${fmt(data.stored, 3)}) does not match the working from the current terms (${fmt(data.computed, 3)}). The score engine has not recomputed it yet, or another path wrote it.`,
            )}
          </span>
        </div>
      )}

      <h3 className="text-[13px] font-semibold text-ink mt-5 mb-1">
        {tr('D’où vient chaque terme', 'Where each term comes from')}
      </h3>
      {!data.sources_visible && (
        <p className="text-[12px] text-ink-muted mb-2" data-testid="score-working-withheld">
          {tr(
            'L’origine des termes est réservée aux rôles autorisés à lire le journal d’audit.',
            'Term origins are shown only to roles allowed to read the audit trail.',
          )}
        </p>
      )}
      <ul>
        {data.terms.map((t) => termRow(t))}
        <li className="py-3" style={{ borderTop: '1px solid var(--border)' }}>
          <div className="flex items-baseline justify-between gap-3">
            <span className="text-[13px] font-medium text-ink">
              {tr('Score enregistré', 'Stored score')}
            </span>
            <span className="mono text-[13px] text-ink">{fmt(data.stored, 3)}</span>
          </div>
          {sourceLine(data.score_source, 'score-working-source-score')}
        </li>
      </ul>

      {canVerify && data.sources_visible && (
        <Link
          to="/governance/audit-trail"
          className="inline-block text-[12.5px] mt-3 underline"
          style={{ color: 'var(--accent)' }}
        >
          {tr(
            'Vérifier l’intégrité du journal complet',
            'Verify the integrity of the whole journal',
          )}
        </Link>
      )}
    </section>
  );
}
