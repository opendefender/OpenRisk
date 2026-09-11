// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Steps 4 and 5 of the guided tunnel (#438) — the two screens that RETURN
// something computed.
//
// The first three steps ask. These two show, and that asymmetry is the whole
// point of the issue: five screens that only collect facts and then drop the
// user on a checklist is the activation cliff. Here the matrix lights as the
// user moves a slider, and the residual falls on screen when they accept a
// control.
//
// Both honour prefers-reduced-motion by rendering the FINAL state directly
// (criterion 13) — not a shorter animation, no animation.

import { useMemo, useState } from 'react';
import { ShieldCheck } from 'lucide-react';

import { useUIStore } from '../../../store/uiStore';
import { useAuthStore } from '../../../hooks/useAuthStore';
import { StepShell } from './stepPrimitives';
import { num, useStepNav, useStoredAnswers } from './stepNav';
import { useOnboardingState, usePosture, usePrefersReducedMotion } from '../useActivation';
import type { PostureRiskView } from '../../../services/activationService';

// ---------------------------------------------------------------------------
// The matrix
//
// Five bands on each axis, on the Score Engine's own scales: probability [0,1]
// and impact [0,10]. The cell the user lands on is `P × I`, which is the same
// arithmetic the engine runs (asset criticality is its third factor and is not
// known at this point — the screen says so rather than inventing one).
// ---------------------------------------------------------------------------

const PROBABILITY_BANDS = [0.1, 0.3, 0.5, 0.7, 0.9];
const IMPACT_BANDS = [2, 4, 6, 8, 10];

/** Score Engine bands, on the P×I×AC scale with AC unknown (so 0–10 here). */
function bandOf(score: number): 'low' | 'medium' | 'high' | 'critical' {
  if (score >= 7) return 'critical';
  if (score >= 4) return 'high';
  if (score >= 2) return 'medium';
  return 'low';
}

function bandColor(band: string): string {
  switch (band) {
    case 'critical':
      return 'var(--critical)';
    case 'high':
      return 'var(--high)';
    case 'medium':
      return 'var(--medium)';
    default:
      return 'var(--low)';
  }
}

// ---------------------------------------------------------------------------
// 4. Score — likelihood and impact for one of the adopted risks
// ---------------------------------------------------------------------------

export function ScoreStep() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const stored = useStoredAnswers('score');
  const { go, busy, error, retry } = useStepNav('score');

  // WHICH risk this step scores, resolved by the server (#643). The step used to
  // render "Évaluez ce risque" over two sliders and no risk at all, so the user
  // was scoring something the screen never named — and the score was stored as
  // an opaque answer and applied to nothing.
  const { data: state } = useOnboardingState();
  const target = state?.score_target;

  // Derived, not copied into state by an effect. The sliders have three possible
  // sources in priority order — what the user is dragging right now, what they
  // stored here last time, and the risk's own values — and an effect that copied
  // the latter two into state would both cascade a render and race the query:
  // the server's answer arrives after the first paint, so the effect had to fire
  // a second time to correct what it had already shown.
  //
  // `edited` is null until the user touches a slider, which is exactly the
  // "uncontrolled until interacted with" semantics this step wants.
  const [edited, setEdited] = useState<{ probability: number; impact: number } | null>(null);
  const probability = edited?.probability ?? num(stored, 'probability', target?.probability ?? 0.5);
  const impact = edited?.impact ?? num(stored, 'impact', target?.impact ?? 6);
  const setProbability = (next: number) => setEdited({ probability: next, impact });
  const setImpact = (next: number) => setEdited({ probability, impact: next });

  const score = Math.round(probability * impact * 1000) / 1000;
  const band = bandOf(score);

  // The nearest band cell, so the grid highlights exactly one square.
  const pIndex = PROBABILITY_BANDS.reduce(
    (best, value, i) =>
      Math.abs(value - probability) < Math.abs(PROBABILITY_BANDS[best] - probability) ? i : best,
    0,
  );
  const iIndex = IMPACT_BANDS.reduce(
    (best, value, i) => (Math.abs(value - impact) < Math.abs(IMPACT_BANDS[best] - impact) ? i : best),
    0,
  );

  return (
    <StepShell
      title={tr('Évaluez ce risque', 'Score this risk')}
      subtitle={
        target
          ? tr(
              'Probabilité et impact. La matrice se met à jour pendant que vous bougez les curseurs — c’est le score qui sera enregistré sur ce risque.',
              'Likelihood and impact. The matrix updates as you move the sliders — this is the score that will be saved on this risk.',
            )
          : tr(
              'Probabilité et impact. La matrice se met à jour pendant que vous bougez les curseurs — c’est le score que le moteur calculera.',
              'Likelihood and impact. The matrix updates as you move the sliders — this is the score the engine will compute.',
            )
      }
      onBack={() => go({ probability, impact }, -1)}
      onNext={() => go({ probability, impact }, 1)}
      nextLabel={tr('Continuer', 'Continue')}
      busy={busy}
      error={error}
      onRetry={retry}
      errorLabel={tr("Impossible d'enregistrer cette étape.", 'This step could not be saved.')}
      errorHint={tr(
        'Vos réponses sont conservées — réessayez, rien n’est perdu.',
        'Your answers are kept — try again, nothing is lost.',
      )}
      retryLabel={tr('Réessayer', 'Try again')}
    >
      {/* The risk being scored, named. Criterion 1 of #643: a step headed "this
          risk" has to say which. */}
      {target ? (
        <div
          className="mb-5 rounded-xl p-4"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
          data-testid="score-target"
        >
          <div className="text-[11px] uppercase tracking-wide text-ink-muted">
            {tr('Risque évalué', 'Risk being scored')}
          </div>
          <div className="text-[14px] font-semibold text-ink mt-0.5">{target.title}</div>
        </div>
      ) : (
        // Adoption sits on step 2 and is skippable, so a tenant can legitimately
        // arrive here with nothing to score. Say so and stay passable — nobody
        // is trapped on a step they cannot complete (#643 criterion 5).
        <div
          className="mb-5 rounded-xl p-4 text-[13px] text-ink-soft"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
          data-testid="score-no-target"
        >
          {tr(
            'Aucun risque n’a encore été adopté, donc cette évaluation ne sera rattachée à aucun risque. Vous pourrez évaluer vos risques depuis le registre.',
            'No risk has been adopted yet, so this scoring will not be attached to a risk. You can score your risks from the register.',
          )}
        </div>
      )}

      <div className="grid gap-5 sm:grid-cols-2">
        <div>
          <Slider
            id="score-probability"
            testId="score-probability"
            label={tr('Probabilité', 'Likelihood')}
            min={0}
            max={1}
            step={0.05}
            value={probability}
            display={`${Math.round(probability * 100)} %`}
            onChange={setProbability}
          />
          <Slider
            id="score-impact"
            testId="score-impact"
            label={tr('Impact', 'Impact')}
            min={0}
            max={10}
            step={0.5}
            value={impact}
            display={impact.toFixed(1)}
            onChange={setImpact}
          />

          {/* The live announcement criterion 12 asks for. `polite` rather than
              `assertive`: a slider fires many changes, and an assertive region
              would interrupt the screen reader on every one of them. */}
          <div
            className="mt-4 rounded-lg p-3"
            style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
            role="status"
            aria-live="polite"
            data-testid="score-readout"
          >
            <div className="text-[11px] uppercase tracking-wide text-ink-muted">
              {tr('Score', 'Score')}
            </div>
            <div className="text-[24px] font-bold tabular-nums" style={{ color: bandColor(band) }}>
              {score.toFixed(2)}
            </div>
            <div className="text-[12px] text-ink-soft mt-0.5">
              {tr('Niveau', 'Level')} : {band}
            </div>
          </div>

          <p className="text-[11.5px] text-ink-muted mt-3 m-0">
            {tr(
              'La criticité de l’actif est le troisième facteur du moteur ; elle sera appliquée quand vous rattacherez un actif.',
              'Asset criticality is the engine’s third factor; it is applied once you link an asset.',
            )}
          </p>
        </div>

        <Matrix pIndex={pIndex} iIndex={iIndex} tr={tr} />
      </div>
    </StepShell>
  );
}

function Slider({
  id,
  testId,
  label,
  min,
  max,
  step,
  value,
  display,
  onChange,
}: {
  id: string;
  testId: string;
  label: string;
  min: number;
  max: number;
  step: number;
  value: number;
  display: string;
  onChange: (v: number) => void;
}) {
  return (
    <div className="mb-4">
      <div className="flex items-baseline justify-between mb-1.5">
        <label htmlFor={id} className="text-[12.5px] font-semibold text-ink">
          {label}
        </label>
        <span className="text-[12.5px] font-semibold text-ink-soft tabular-nums">{display}</span>
      </div>
      <input
        id={id}
        data-testid={testId}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full"
        aria-valuetext={display}
      />
    </div>
  );
}

/**
 * The 5×5 grid. A `<table>` rather than a div soup: it IS tabular data, and a
 * screen reader reading "likelihood 70%, impact 8, high" beats it reading
 * twenty-five unlabelled cells.
 */
function Matrix({
  pIndex,
  iIndex,
  tr,
}: {
  pIndex: number;
  iIndex: number;
  tr: (fr: string, en: string) => string;
}) {
  const reduced = usePrefersReducedMotion();

  return (
    <table className="w-full border-separate" style={{ borderSpacing: 3 }}>
      <caption className="sr-only">
        {tr(
          'Matrice de risque : probabilité en lignes, impact en colonnes',
          'Risk matrix: likelihood by row, impact by column',
        )}
      </caption>
      <tbody>
        {[...PROBABILITY_BANDS].reverse().map((p, row) => {
          const realRow = PROBABILITY_BANDS.length - 1 - row;
          return (
            <tr key={p}>
              {IMPACT_BANDS.map((impact, col) => {
                const cellScore = p * impact;
                const band = bandOf(cellScore);
                const active = realRow === pIndex && col === iIndex;
                return (
                  <td
                    key={impact}
                    data-testid={active ? 'matrix-cell-active' : undefined}
                    aria-current={active ? 'true' : undefined}
                    className="h-9 rounded text-center align-middle text-[10.5px] font-semibold"
                    style={{
                      background: active
                        ? bandColor(band)
                        : `color-mix(in srgb, ${bandColor(band)} 14%, transparent)`,
                      color: active ? 'var(--fg-on-solid)' : 'var(--fg-muted)',
                      outline: active ? '2px solid var(--fg-primary)' : 'none',
                      // Criterion 13: no transition at all under reduced motion,
                      // so the lit cell simply IS lit on the next paint.
                      transition: reduced ? 'none' : 'background .18s ease, outline .18s ease',
                    }}
                  >
                    {active ? (p * impact).toFixed(1) : ''}
                  </td>
                );
              })}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// ---------------------------------------------------------------------------
// 5. Cover — accept the proposed control and watch the residual fall
// ---------------------------------------------------------------------------

export function CoverStep() {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { go, busy, error, retry } = useStepNav('cover');
  const user = useAuthStore((s) => s.user);

  // The posture is read here so the residual shown is the REAL one, computed by
  // the server from this tenant's own control mappings (ADR 0003). A number
  // invented client-side would be the placeholder criterion 7 forbids, on the
  // screen the whole tunnel builds towards.
  const { data: posture, isLoading } = usePosture();

  // The SAME risk step 4 scored (#643 criterion 4). This used to be
  // `top_risks[0]` — ordered by score — so the user's own scoring could change
  // which risk step 5 named, and the two screens could disagree precisely
  // because the tunnel worked. Falls back to the top risk when the tenant
  // adopted nothing, which is the state that produced the old behaviour anyway.
  const { data: state } = useOnboardingState();
  const targetId = state?.score_target?.id;
  const risk: PostureRiskView | undefined = useMemo(() => {
    const top = posture?.top_risks;
    if (!top?.length) return undefined;
    return (targetId && top.find((r) => r.id === targetId)) || top[0];
  }, [posture, targetId]);

  return (
    <StepShell
      title={tr('Ce que vos contrôles couvrent déjà', 'What your controls already cover')}
      subtitle={tr(
        'Le référentiel que vous venez d’importer couvre déjà une partie de ce risque. Voici l’écart entre son score inhérent et son score résiduel, calculé par le serveur à partir de vos propres contrôles.',
        'The framework you just imported already covers part of this risk. Here is the gap between its inherent score and its residual score, computed by the server from your own controls.',
      )}
      onBack={() => go({}, -1)}
      onNext={() => go({ by: user?.id ?? '' }, 1)}
      nextLabel={tr('Voir ma posture', 'See my posture')}
      busy={busy}
      error={error}
      onRetry={retry}
      errorLabel={tr("Impossible d'enregistrer cette étape.", 'This step could not be saved.')}
      errorHint={tr(
        'Vos réponses sont conservées — réessayez, rien n’est perdu.',
        'Your answers are kept — try again, nothing is lost.',
      )}
      retryLabel={tr('Réessayer', 'Try again')}
    >
      {isLoading && <div className="h-28 rounded-xl or-skeleton" />}

      {/* The degraded case the DoD names: the posture is not computable yet
          (no framework imported, or the import failed). The step still works —
          it stores the acceptance — and says plainly that the number is not
          available rather than showing a zero that would read as "no risk". */}
      {!isLoading && !risk && (
        <div
          className="rounded-xl p-4 text-[13px] text-ink-soft"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-strong)' }}
          data-testid="cover-unavailable"
        >
          {tr(
            'Le résiduel sera calculé dès qu’un référentiel sera importé — nous vous le montrerons à l’écran suivant.',
            'The residual is computed as soon as a framework is imported — we will show it on the next screen.',
          )}
        </div>
      )}

      {!isLoading && risk && <ResidualCard risk={risk} tr={tr} />}

      {/* #639: there used to be a checkbox here reading "J'accepte le contrôle
          proposé pour ce risque." Ticking it created nothing — no control, no
          mapping, and no backend path read the answer. It also drove which
          number the card labelled "Résiduel", so leaving it unticked displayed
          the INHERENT score under the residual label. Both are gone: the step
          now shows what the imported framework has actually earned, which is
          true and is the payoff the tunnel was built for. Proposing a real
          control is still open on #639 and is a product decision. */}
    </StepShell>
  );
}

function ResidualCard({
  risk,
  tr,
}: {
  risk: PostureRiskView;
  tr: (fr: string, en: string) => string;
}) {
  const reduced = usePrefersReducedMotion();

  // Both numbers come from the server and neither is conditional. A checkbox
  // used to choose which of the two sat under the "Résiduel" label, so an
  // unticked box showed the INHERENT score labelled as the residual (#639).
  const shown = risk.residual.value;
  const band = bandOf(shown);

  return (
    <div
      className="rounded-xl p-5"
      style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)' }}
      data-testid="cover-residual"
    >
      <div className="text-[13.5px] font-semibold text-ink mb-3 truncate">{risk.title}</div>

      <div className="flex items-end gap-5">
        <Figure label={tr('Inhérent', 'Inherent')} value={risk.residual.inherent} muted />
        <ShieldCheck
          size={18}
          aria-hidden="true"
          style={{
            color: risk.residual.coverage.measured ? 'var(--low)' : 'var(--fg-muted)',
            marginBottom: 6,
          }}
        />
        <Figure
          label={tr('Résiduel', 'Residual')}
          value={shown}
          color={bandColor(band)}
          testId="cover-residual-value"
          reduced={reduced}
        />
      </div>

      {risk.residual.coverage.measured ? (
        <div className="text-[12px] text-ink-soft mt-3" role="status" aria-live="polite">
          {tr(
            `${risk.residual.coverage.applicable} contrôle(s) rattaché(s) — réduction de ${risk.residual.reduction}`,
            `${risk.residual.coverage.applicable} control(s) mapped — down by ${risk.residual.reduction}`,
          )}
        </div>
      ) : (
        <div className="text-[12px] text-ink-muted mt-3">
          {tr(
            'Aucun contrôle rattaché pour l’instant — le résiduel égale l’inhérent.',
            'No control mapped yet — the residual equals the inherent score.',
          )}
        </div>
      )}
    </div>
  );
}

function Figure({
  label,
  value,
  color,
  muted,
  testId,
  reduced,
}: {
  label: string;
  value: number;
  color?: string;
  muted?: boolean;
  testId?: string;
  reduced?: boolean;
}) {
  return (
    <div data-testid={testId}>
      <div className="text-[10.5px] uppercase tracking-wide text-ink-muted">{label}</div>
      <div
        className="text-[28px] font-bold tabular-nums leading-none"
        style={{
          color: muted ? 'var(--fg-secondary)' : (color ?? 'var(--fg-primary)'),
          transition: reduced ? 'none' : 'color .2s ease',
        }}
      >
        {value}
      </div>
    </div>
  );
}
