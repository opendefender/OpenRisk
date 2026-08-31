// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: LicenseRef-OpenRisk-Commercial
//
// The dry-run trace.
//
// The old "Test" button ran the rule for real. This one answers the same
// question — "would this work?" — while touching nothing, and shows the answer
// the way an operator actually needs it: step by step, with the payload as it
// stood entering each step, and the failure point named rather than left to be
// inferred from a wall of green and grey.

import { useRef, useState } from 'react';
import { toast } from 'sonner';
import {
  FlaskConical, X, CheckCircle2, MinusCircle, XCircle, CircleSlash, Ban,
  ShieldCheck, Loader2, ChevronDown, ChevronRight, AlertTriangle, Database,
} from 'lucide-react';
import { Btn, Card } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAutomationMutations } from './useAutomation';
import { ACTION_META, pick } from './automationMeta';
import type {
  AutomationRule, DryRunReport, DryRunStep, DryRunVerdict, DryRunInput,
} from './automationService';

const VERDICT: Record<
  DryRunVerdict,
  { icon: typeof CheckCircle2; color: string; fr: string; en: string }
> = {
  would_run: { icon: CheckCircle2, color: 'var(--low)', fr: 'S’exécuterait', en: 'Would run' },
  would_skip: { icon: MinusCircle, color: 'var(--fg-secondary)', fr: 'Serait ignorée', en: 'Would skip' },
  would_fail: { icon: XCircle, color: 'var(--critical)', fr: 'Échouerait', en: 'Would fail' },
  not_reached: { icon: CircleSlash, color: 'var(--fg-secondary)', fr: 'Jamais atteinte', en: 'Never reached' },
  not_matched: { icon: Ban, color: 'var(--medium)', fr: 'Conditions non remplies', en: 'Conditions not met' },
};

function PayloadTable({ payload }: { payload: Record<string, unknown> }) {
  const entries = Object.entries(payload ?? {});
  if (entries.length === 0) {
    return <p className="text-[12px]" style={{ color: 'var(--fg-secondary)' }}>—</p>;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-[12px]">
        <tbody>
          {entries.map(([k, v]) => (
            <tr key={k}>
              <td className="pr-3 py-0.5 align-top mono whitespace-nowrap" style={{ color: 'var(--fg-secondary)' }}>
                {k}
              </td>
              <td className="py-0.5 mono break-all" style={{ color: 'var(--fg-primary)' }}>
                {typeof v === 'object' ? JSON.stringify(v) : String(v)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function StepRow({ step, isFailure }: { step: DryRunStep; isFailure: boolean }) {
  const lang = useUIStore((s) => s.lang);
  const [open, setOpen] = useState(isFailure); // the failing step opens itself
  const meta = VERDICT[step.verdict];
  const Icon = meta.icon;
  const action = ACTION_META[step.action];

  return (
    <div
      className="rounded-[10px] border"
      style={{
        borderColor: isFailure ? 'var(--critical)' : 'var(--border)',
        background: isFailure ? 'color-mix(in srgb, var(--critical) 6%, transparent)' : 'transparent',
      }}
    >
      <button
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-start gap-2.5 p-3 text-left"
      >
        <span className="mono text-[11px] pt-0.5 w-5 shrink-0" style={{ color: 'var(--fg-secondary)' }}>
          {step.index + 1}
        </span>
        <Icon size={16} style={{ color: meta.color }} className="shrink-0 mt-0.5" />
        <span className="flex-1 min-w-0">
          <span className="flex items-center gap-2 flex-wrap">
            <span className="text-[13px] font-semibold" style={{ color: 'var(--fg-primary)' }}>
              {action ? pick(action.label, lang) : step.action}
            </span>
            <span
              className="text-[10.5px] font-bold px-1.5 py-0.5 rounded"
              style={{ background: `color-mix(in srgb, ${meta.color} 15%, transparent)`, color: meta.color }}
            >
              {pick({ fr: meta.fr, en: meta.en }, lang)}
            </span>
            {step.capability && !step.wired && (
              <span className="text-[10.5px]" style={{ color: 'var(--fg-secondary)' }}>
                · {step.capability} {lang === 'fr' ? 'non configuré' : 'not configured'}
              </span>
            )}
          </span>
          <span className="block text-[12px] mt-0.5" style={{ color: 'var(--fg-secondary)' }}>
            {step.detail}
          </span>
        </span>
        {open ? <ChevronDown size={14} className="shrink-0 mt-1" /> : <ChevronRight size={14} className="shrink-0 mt-1" />}
      </button>

      {open && (
        <div className="px-3 pb-3 pl-10 grid gap-3 md:grid-cols-2">
          <div>
            <p className="text-[11px] font-bold uppercase tracking-wide mb-1" style={{ color: 'var(--fg-secondary)' }}>
              {lang === 'fr' ? 'Données à cet instant' : 'Payload at this point'}
            </p>
            <PayloadTable payload={step.payload} />
          </div>
          <div>
            {step.params && Object.keys(step.params).length > 0 && (
              <>
                <p className="text-[11px] font-bold uppercase tracking-wide mb-1" style={{ color: 'var(--fg-secondary)' }}>
                  {lang === 'fr' ? 'Paramètres de l’action' : 'Action parameters'}
                </p>
                <PayloadTable payload={step.params} />
              </>
            )}
            {step.produces && Object.keys(step.produces).length > 0 && (
              <>
                <p className="text-[11px] font-bold uppercase tracking-wide mb-1 mt-2" style={{ color: 'var(--fg-secondary)' }}>
                  {lang === 'fr' ? 'Ajouterait au contexte' : 'Would add to the context'}
                </p>
                <PayloadTable payload={step.produces} />
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export function DryRunPanel({ rule, onClose }: { rule: AutomationRule; onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { dryRun } = useAutomationMutations();

  const [report, setReport] = useState<DryRunReport | null>(null);
  const [severity, setSeverity] = useState('');
  const abortRef = useRef<AbortController | null>(null);

  const run = async () => {
    const controller = new AbortController();
    abortRef.current = controller;
    const input: DryRunInput = severity ? { overrides: { severity } } : {};
    try {
      const res = await dryRun.mutateAsync({ id: rule.id, input, signal: controller.signal });
      setReport(res);
    } catch (e) {
      // A cancelled test is the operator's own doing, not an error to apologise for.
      if (controller.signal.aborted) {
        toast.info(tr('Test annulé', 'Test cancelled'));
        return;
      }
      toast.error(tr('Le test n’a pas pu s’exécuter', 'The test could not run'));
    } finally {
      abortRef.current = null;
    }
  };

  const cancel = () => {
    abortRef.current?.abort();
    if (report?.id) void import('./automationService').then((m) => m.automationService.cancelDryRun(report.id));
  };

  const running = dryRun.isPending;

  return (
    <div className="fixed inset-0 z-50 flex justify-end" style={{ background: 'rgba(0,0,0,.35)' }}>
      <div
        className="w-full max-w-[760px] h-full overflow-y-auto or-slidein"
        style={{ background: 'var(--bg-elevated)', borderLeft: '1px solid var(--border-strong)' }}
      >
        <div
          className="sticky top-0 z-10 flex items-center justify-between px-5 py-4"
          style={{ background: 'var(--bg-elevated)', borderBottom: '1px solid var(--border)' }}
        >
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <FlaskConical size={16} style={{ color: 'var(--accent-500)' }} />
              <h2 className="text-[15px] font-bold truncate" style={{ color: 'var(--fg-primary)' }}>
                {tr('Tester', 'Test')} — {rule.name}
              </h2>
            </div>
            <p className="text-[12px] mt-0.5" style={{ color: 'var(--fg-secondary)' }}>
              {tr(
                'Simulation sur vos données réelles. Aucune action n’est exécutée.',
                'Simulated against your real data. No action is executed.',
              )}
            </p>
          </div>
          <button onClick={onClose} aria-label={tr('Fermer', 'Close')} className="p-1.5 rounded-lg">
            <X size={18} />
          </button>
        </div>

        <div className="p-5 space-y-4">
          {/* The guarantee, stated where the user is about to rely on it. */}
          <div
            className="rounded-[10px] p-3 flex items-start gap-2.5"
            style={{ background: 'color-mix(in srgb, var(--low) 8%, transparent)', border: '1px solid color-mix(in srgb, var(--low) 30%, transparent)' }}
          >
            <ShieldCheck size={16} style={{ color: 'var(--low)' }} className="shrink-0 mt-0.5" />
            <p className="text-[12.5px]" style={{ color: 'var(--fg-primary)' }}>
              {tr(
                'Ce test ne crée aucun risque, n’ouvre aucun ticket et n’envoie aucune alerte. Il montre ce qui se passerait.',
                'This test creates no risk, opens no ticket and sends no alert. It shows what would happen.',
              )}
            </p>
          </div>

          <Card>
            <div className="p-4 flex flex-wrap items-end gap-3">
              <label className="flex flex-col gap-1">
                <span className="text-[11.5px] font-semibold" style={{ color: 'var(--fg-secondary)' }}>
                  {tr('Et si la criticité était…', 'What if the severity were…')}
                </span>
                <select
                  value={severity}
                  onChange={(e) => setSeverity(e.target.value)}
                  className="h-9 px-2 rounded-[9px] text-[13px]"
                  style={{ background: 'var(--bg)', border: '1px solid var(--border-strong)', color: 'var(--fg-primary)' }}
                >
                  <option value="">{tr('(garder la valeur réelle)', '(keep the real value)')}</option>
                  <option value="critical">critical</option>
                  <option value="high">high</option>
                  <option value="medium">medium</option>
                  <option value="low">low</option>
                </select>
              </label>
              <Btn
                label={running ? tr('Test en cours…', 'Testing…') : tr('Lancer le test', 'Run the test')}
                icon={running ? Loader2 : FlaskConical}
                primary
                onClick={run}
                disabled={running}
              />
              {running && <Btn label={tr('Annuler le test', 'Cancel test')} icon={X} onClick={cancel} />}
            </div>
          </Card>

          {report && (
            <>
              {/* Where the trace's subject came from — a green run on invented
                  data must never read as a green run on the tenant's data. */}
              <div className="flex items-start gap-2 text-[12px]" style={{ color: 'var(--fg-secondary)' }}>
                <Database size={14} className="shrink-0 mt-0.5" />
                <span>
                  {report.real_subject
                    ? tr('Données réelles — ', 'Real data — ')
                    : tr('⚠ Données simulées — ', '⚠ Simulated data — ')}
                  {report.subject_source}
                </span>
              </div>

              <div className="flex flex-wrap gap-2">
                {[
                  { n: report.would_run, label: tr('s’exécuteraient', 'would run'), color: 'var(--low)' },
                  { n: report.would_skip, label: tr('ignorées', 'skipped'), color: 'var(--fg-secondary)' },
                  { n: report.would_fail, label: tr('échoueraient', 'would fail'), color: 'var(--critical)' },
                ].map((s) => (
                  <span
                    key={s.label}
                    className="text-[12px] px-2.5 py-1 rounded-[8px] font-semibold"
                    style={{ background: `color-mix(in srgb, ${s.color} 12%, transparent)`, color: s.color }}
                  >
                    {s.n} {s.label}
                  </span>
                ))}
                <span className="text-[12px] px-2.5 py-1 rounded-[8px] mono" style={{ color: 'var(--fg-secondary)' }}>
                  {report.duration_ms} ms
                </span>
              </div>

              {/* Conditions first: a rule that never matches is the most common
                  "it does nothing" report, and no step explains it. */}
              <Card>
                <div className="p-4">
                  <p className="text-[11px] font-bold uppercase tracking-wide mb-1" style={{ color: 'var(--fg-secondary)' }}>
                    {tr('Conditions', 'Conditions')}
                  </p>
                  <p
                    className="text-[13px]"
                    style={{ color: report.conditions_matched ? 'var(--low)' : 'var(--medium)' }}
                  >
                    {report.conditions_matched
                      ? tr('✓ ', '✓ ')
                      : tr('✗ ', '✗ ')}
                    {report.conditions_detail}
                  </p>
                </div>
              </Card>

              {report.failure_reason && (
                <div
                  className="rounded-[10px] p-3.5"
                  style={{ background: 'color-mix(in srgb, var(--critical) 8%, transparent)', border: '1px solid var(--critical)' }}
                >
                  <div className="flex items-start gap-2.5">
                    <AlertTriangle size={16} style={{ color: 'var(--critical)' }} className="shrink-0 mt-0.5" />
                    <div className="min-w-0">
                      <p className="text-[13px] font-bold" style={{ color: 'var(--critical)' }}>
                        {tr('Point d’échec : étape ', 'Failure point: step ')}
                        {(report.failed_at_index ?? 0) + 1}
                        {report.failed_action ? ` — ${report.failed_action}` : ''}
                      </p>
                      <p className="text-[12.5px] mt-1" style={{ color: 'var(--fg-primary)' }}>
                        {report.failure_reason}
                      </p>
                      {report.payload_at_failure && (
                        <div className="mt-2">
                          <p className="text-[11px] font-bold uppercase tracking-wide mb-1" style={{ color: 'var(--fg-secondary)' }}>
                            {tr('Données à cet instant', 'Payload at that moment')}
                          </p>
                          <PayloadTable payload={report.payload_at_failure} />
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}

              <div className="space-y-2">
                {report.steps.map((s) => (
                  <StepRow key={s.index} step={s} isFailure={report.failed_at_index === s.index} />
                ))}
              </div>
            </>
          )}

          {!report && !running && (
            <p className="text-[12.5px]" style={{ color: 'var(--fg-secondary)' }}>
              {tr(
                'Lancez le test pour voir, étape par étape, ce que cette règle ferait de votre dernière vulnérabilité réelle.',
                'Run the test to see, step by step, what this rule would do with your latest real vulnerability.',
              )}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
