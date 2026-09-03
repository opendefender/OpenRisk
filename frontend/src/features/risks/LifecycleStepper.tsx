// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// <LifecycleStepper> — where this risk stands, what comes next, and EXACTLY
// what is in the way.
//
// The last part is the point. The previous stepper offered whichever buttons it
// felt like and the server accepted them, so a risk could be marked "traité"
// with two sub-actions still open. Now the server decides, and this renders its
// answer verbatim: a blocked step is SHOWN, greyed, with its reason
// ("2 sous-actions restantes sur la mitigation MIT-14") and a way out — never
// hidden, because a user who cannot see the next step has no way to learn what
// to do about it.

import { useState } from 'react';
import {
  AlertTriangle,
  ArrowRight,
  Check,
  Loader2,
  Lock,
  RotateCcw,
  ShieldCheck,
} from 'lucide-react';
import { toast } from 'sonner';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  useRiskTransitions,
  useTransitionRisk,
  type RiskState,
  type TransitionGuard,
  type TransitionOption,
} from './useLifecycle';

/** Icon for the way out of each guard, so a blocked step is never a dead end. */
const GUARD_ACTION: Record<TransitionGuard, { icon: typeof ShieldCheck }> = {
  active_mitigation: { icon: ShieldCheck },
  subactions_complete: { icon: ShieldCheck },
  governance_approval: { icon: Lock },
};

interface Props {
  riskId: string;
  /** Opens the mitigation plan — the way out of both treatment guards. */
  onOpenMitigations?: () => void;
  /** Opens Governance — the way out of the residual-acceptance guard. */
  onOpenGovernance?: () => void;
}

export function LifecycleStepper({ riskId, onOpenMitigations, onOpenGovernance }: Props) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));

  const { data, isLoading, isError, refetch } = useRiskTransitions(riskId);
  const transition = useTransitionRisk(riskId);
  const [comment, setComment] = useState('');
  const [pending, setPending] = useState<RiskState | null>(null);

  if (isLoading) {
    return (
      <div className="space-y-2" aria-busy>
        <div className="h-3 w-40 animate-pulse rounded-full bg-app" />
        <div className="h-16 animate-pulse rounded-3xl bg-app" />
      </div>
    );
  }

  if (isError || !data) {
    return (
      <div className="rounded-3xl border border-border bg-app p-4 text-[13px] text-ink-muted">
        <p>{tr('Impossible de charger le cycle de vie.', 'Could not load the lifecycle.')}</p>
        <button type="button" onClick={() => refetch()} className="mt-2 text-accent underline">
          {tr('Réessayer', 'Retry')}
        </button>
      </div>
    );
  }

  const run = async (to: RiskState) => {
    setPending(to);
    try {
      await transition.mutateAsync({ to, comment: comment.trim() || undefined });
      setComment('');
      toast.success(tr('Cycle de vie mis à jour', 'Lifecycle updated'));
    } catch (err) {
      // The server's refusal IS the explanation — surfacing our own generic
      // message here would throw away the only actionable part.
      const message =
        (err as { response?: { data?: { error?: string } } })?.response?.data?.error ??
        tr('La transition a été refusée.', 'The transition was refused.');
      toast.error(message);
    } finally {
      setPending(null);
    }
  };

  const wayOut = (guard: TransitionGuard) => {
    if (guard === 'governance_approval') return onOpenGovernance;
    return onOpenMitigations;
  };

  const wayOutLabel = (guard: TransitionGuard) =>
    guard === 'governance_approval'
      ? tr('Ouvrir la Gouvernance', 'Open Governance')
      : tr('Ouvrir le plan de mitigation', 'Open the mitigation plan');

  const forward = data.options.filter((o) => o.is_forward);
  const backward = data.options.filter((o) => !o.is_forward);

  return (
    <div className="space-y-4">
      {/* Progress spine ------------------------------------------------- */}
      <div>
        <div className="mb-1.5 flex items-baseline justify-between">
          <span className="text-[11px] font-semibold uppercase tracking-[0.16em] text-ink-muted">
            {tr('Étape actuelle', 'Current step')}
          </span>
          <span className="text-[11px] text-ink-muted">
            {data.step_index + 1} / {data.step_count}
          </span>
        </div>
        <p className="text-lg font-semibold text-ink">{data.current_label}</p>
        <div
          className="mt-2 flex gap-1"
          role="img"
          aria-label={`${data.step_index + 1}/${data.step_count}`}
        >
          {Array.from({ length: data.step_count }).map((_, i) => (
            <span
              key={i}
              className="h-1.5 flex-1 rounded-full transition"
              style={{ background: i <= data.step_index ? 'var(--accent)' : 'var(--border)' }}
            />
          ))}
        </div>
      </div>

      {/* The blocker, stated once and prominently ----------------------- */}
      {data.blocked_reason ? (
        <div
          className="flex items-start gap-2.5 rounded-3xl border p-3"
          style={{
            borderColor: 'color-mix(in srgb, var(--high) 40%, transparent)',
            background: 'color-mix(in srgb, var(--high) 10%, transparent)',
          }}
        >
          <AlertTriangle size={16} className="mt-0.5 shrink-0" style={{ color: 'var(--high)' }} />
          <div className="min-w-0">
            <p className="text-[13px] font-semibold text-ink">
              {tr('Étape suivante bloquée', 'Next step blocked')} — {data.next_label}
            </p>
            <p className="mt-0.5 text-[13px] text-ink-soft">{data.blocked_reason}</p>
          </div>
        </div>
      ) : null}

      {/* Comment — the "why", carried into the audit trail -------------- */}
      {canUpdate ? (
        <input
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          maxLength={1000}
          placeholder={tr(
            'Commentaire (optionnel, conservé dans la piste d’audit)',
            'Comment (optional, kept in the audit trail)',
          )}
          className="w-full rounded-2xl border border-border bg-elevated px-3 py-2 text-[13px] text-ink outline-none focus:border-accent"
        />
      ) : null}

      {/* Options — blocked ones included, greyed, with their reason ----- */}
      <div className="space-y-2">
        {[...forward, ...backward].map((opt) => (
          <TransitionRow
            key={opt.to}
            opt={opt}
            disabled={!canUpdate || transition.isPending}
            pending={pending === opt.to}
            onRun={() => run(opt.to)}
            wayOut={opt.guard ? wayOut(opt.guard) : undefined}
            wayOutLabel={opt.guard ? wayOutLabel(opt.guard) : ''}
          />
        ))}
        {data.options.length === 0 ? (
          <p className="text-[13px] text-ink-muted">
            {tr(
              'Aucune transition possible depuis cet état.',
              'No transition is available from this state.',
            )}
          </p>
        ) : null}
      </div>

      {!canUpdate ? (
        <p className="text-[11px] text-ink-muted">
          {tr(
            'Lecture seule : la permission risks:update est requise pour faire évoluer un risque.',
            'Read-only: the risks:update permission is required to move a risk along.',
          )}
        </p>
      ) : null}
    </div>
  );
}

function TransitionRow({
  opt,
  disabled,
  pending,
  onRun,
  wayOut,
  wayOutLabel,
}: {
  opt: TransitionOption;
  disabled: boolean;
  pending: boolean;
  onRun: () => void;
  wayOut?: () => void;
  wayOutLabel: string;
}) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const GuardIcon = opt.guard ? (GUARD_ACTION[opt.guard]?.icon ?? Lock) : Lock;

  return (
    <div
      className="rounded-3xl border p-3 transition"
      style={{
        borderColor: opt.allowed
          ? 'var(--border)'
          : 'color-mix(in srgb, var(--border) 70%, transparent)',
        opacity: opt.allowed ? 1 : 0.78,
      }}
    >
      <div className="flex items-center gap-2.5">
        <span
          className="inline-flex h-7 w-7 items-center justify-center rounded-full shrink-0"
          style={{
            background: opt.allowed
              ? 'color-mix(in srgb, var(--accent) 14%, transparent)'
              : 'color-mix(in srgb, var(--ink-muted) 12%, transparent)',
            color: opt.allowed ? 'var(--accent)' : 'var(--ink-muted)',
          }}
        >
          {opt.allowed ? (
            opt.is_forward ? (
              <ArrowRight size={14} />
            ) : (
              <RotateCcw size={14} />
            )
          ) : (
            <GuardIcon size={14} />
          )}
        </span>

        <span className="min-w-0 flex-1">
          <span className="block text-[13px] font-semibold text-ink">{opt.label}</span>
          {!opt.allowed && opt.reason ? (
            <span className="mt-0.5 block text-[12px] text-ink-soft">{opt.reason}</span>
          ) : null}
        </span>

        <button
          type="button"
          onClick={onRun}
          disabled={disabled || !opt.allowed || pending}
          className="shrink-0 rounded-full px-3 py-1.5 text-[12px] font-semibold transition disabled:cursor-not-allowed"
          style={{
            background: opt.allowed ? 'var(--accent)' : 'transparent',
            color: opt.allowed ? 'var(--on-accent, var(--fg-primary))' : 'var(--ink-muted)',
            border: opt.allowed ? 'none' : '1px solid var(--border)',
          }}
        >
          {pending ? (
            <Loader2 size={13} className="animate-spin" />
          ) : opt.allowed ? (
            tr('Passer', 'Move')
          ) : (
            tr('Bloqué', 'Blocked')
          )}
        </button>
      </div>

      {/* A blocked step offers the way OUT, not just the bad news. */}
      {!opt.allowed && wayOut ? (
        <button
          type="button"
          onClick={wayOut}
          className="mt-2 ml-9 inline-flex items-center gap-1 text-[12px] font-semibold text-accent hover:underline"
        >
          {wayOutLabel}
          <ArrowRight size={12} />
        </button>
      ) : null}
    </div>
  );
}

/** Compact state pill for the register table and the drawer header. */
export function StatePill({ state, label }: { state: RiskState; label?: string }) {
  const tone: Record<RiskState, string> = {
    draft: 'var(--ink-muted)',
    identified: 'var(--low)',
    assessed: 'var(--low)',
    treatment_planned: 'var(--medium)',
    in_treatment: 'var(--accent)',
    residual_accepted: 'var(--medium)',
    mitigated: 'var(--low)',
    closed: 'var(--ink-muted)',
    reopened: 'var(--high)',
  };
  const color = tone[state] ?? 'var(--ink-muted)';
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold whitespace-nowrap"
      style={{ background: `color-mix(in srgb, ${color} 14%, transparent)`, color }}
    >
      {state === 'closed' || state === 'mitigated' ? <Check size={10} /> : null}
      {label ?? state}
    </span>
  );
}
