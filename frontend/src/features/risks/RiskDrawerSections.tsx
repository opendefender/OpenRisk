// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The risk drawer's sections that the redesign (#913) keeps but does not draw:
// responsibilities and control mappings, lifecycle, score working, SmartScore,
// financial quantification, CTI, AI treatment plan and the audit timeline.
// Moved here unchanged from RiskRegisterPage.tsx; they are reached from the
// drawer's "Plus" tab. The redesigned tabs (Aperçu, Mitigations, Historique)
// live in RiskDrawer.tsx.

import { useFormat, usePreferredDate } from '../../hooks/useI18n';
import { useState } from 'react';
import { useNavigate } from 'react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  X,
  ShieldCheck,
  Route as RouteIcon,
  Coins,
  SlidersHorizontal,
  Sparkles,
  Loader2,
  Bug,
} from 'lucide-react';
import { Btn, SkeletonRows, EmptyState, ErrorState } from '../../shared/ui';
import { ProgressState } from '../../shared/ProgressState';
import { HistoryTimeline } from '../../shared/HistoryTimeline';
import { useEntityTimeline } from '../entity-drawer/useEntityDrawer';
import { toHistoryEntries } from './riskTimelineEntries';
import { useUIStrings } from '../../shared/uiStrings';
import { useUIStore } from '../../store/uiStore';
import { useRiskStore, type RiskPhase } from '../../hooks/useRiskStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { relTime, type UiRisk } from './riskMap';
import { ScoreWorking } from './components/ScoreWorking';
import { useRiskFinancial } from '../financial/useFinancial';
import { useRiskSmartScore } from './useSmartScore';
import { SmartRiskRadar } from './components/SmartRiskRadar';
import { useTreatmentPlan } from '../ai/useAi';
import { LifecycleStepper } from './LifecycleStepper';
import { useRiskMappings, useCreateMapping, useDeleteMapping } from './useTaxonomy';
import { ComplianceMappingField, type MappingDraft } from './ComplianceMappingField';
import { OwnershipFields } from '../../shared/UserPicker';
import { ownershipPatch, type OwnershipRole } from '../../services/ownershipService';
import { mappingHref, mappingLabel } from '../../services/taxonomyService';
import { ctiService } from '../cti/ctiService';
import { useFeature } from '../billing/useEntitlements';
import type { LocaleCode } from '../../i18n/locales';

// DrawerTimeline — the "Timeline" tab: who changed this risk, and when, from
// the merged entity timeline (GET /entities/risk/:id/timeline), rendered with the
// shared HistoryTimeline primitive (UX-25).
//
// Every entry comes from the audit trail with its author (#793): a person, a
// token's owner, or a job by name such as the Score Engine. That is the same
// answer the score working gives. It used to read GET /risks/:id/timeline,
// whose changed_by named the risk's creator for every change.
export function DrawerTimeline({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const { data, isLoading, error, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useEntityTimeline('risk', r.id, true);

  const entries = toHistoryEntries(
    (data?.pages ?? []).flatMap((p) => p.events),
    tr,
  );

  return (
    <div className="py-5 px-[22px]">
      <HistoryTimeline
        entries={entries}
        isLoading={isLoading}
        error={!!error}
        emptyLabel={tr(
          'Aucun changement enregistré pour ce risque.',
          'No changes recorded for this risk yet.',
        )}
        errorLabel={tr('Chargement de l’historique impossible.', 'Could not load the history.')}
        formatDate={(iso) => relTime(iso, lang)}
      />
      {hasNextPage && (
        <div className="mt-3">
          <Btn
            label={
              isFetchingNextPage ? tr('Chargement…', 'Loading…') : tr('Afficher plus', 'Show more')
            }
            onClick={() => void fetchNextPage()}
          />
        </div>
      )}
    </div>
  );
}

// DrawerCTI — the "CTI" tab.
//
// This tab rendered the string "Coming soon" for every risk, including risks the
// CTI engine itself created. Those carry `source_cve_id`, and /cti/vulnerabilities/:cve
// returns the real NVD + CISA-KEV record for it, so the data was one call away
// (W0-05 / D10).
//
// Two honest outcomes, and which one you get depends on the risk rather than on
// the release:
//
//   - the risk names a CVE  → its real advisory: CVSS, CISA-KEV status and due
//     date, MITRE tactics/techniques, remediation, with a link into Threat Intel;
//   - the risk names none   → says so, and explains what would populate it,
//     instead of implying the whole tab is unbuilt.
export function DrawerCTI({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const when = usePreferredDate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const cve = r.raw.source_cve_id ?? '';
  // The advisory comes from the CTI routes, which open at Business (#783). On a
  // plan without them the tab names the CVE and says what would show it.
  const cti = useFeature('cti');

  const { data, isLoading, isError } = useQuery({
    queryKey: ['cti', 'vulnerability', cve],
    queryFn: () => ctiService.get(cve),
    enabled: !!cve && !cti.loading && !cti.locked,
  });

  if (!cve) {
    return (
      <div className="py-8 px-[22px]">
        <EmptyState
          icon={Bug}
          title={tr('Aucune CVE associée', 'No CVE linked')}
          description={tr(
            'Ce risque n’est rattaché à aucune vulnérabilité publique. Les risques créés par le moteur CTI, ou rattachés à une CVE, affichent ici leur avis complet (CVSS, CISA-KEV, MITRE ATT&CK).',
            'This risk is not linked to a public vulnerability. Risks raised by the CTI engine, or linked to a CVE, show the full advisory here (CVSS, CISA-KEV, MITRE ATT&CK).',
          )}
          primaryAction={
            <Btn
              label={tr('Ouvrir Threat Intel', 'Open Threat Intel')}
              onClick={() => navigate('/threat-map')}
            />
          }
        />
      </div>
    );
  }

  if (cti.locked) {
    const plan = cti.requiredPlan.charAt(0).toUpperCase() + cti.requiredPlan.slice(1);
    return (
      <div className="py-8 px-[22px]">
        <EmptyState
          icon={Bug}
          title={cve}
          description={tr(
            `Ce risque est rattaché à ${cve}. L’avis complet (CVSS, CISA-KEV, MITRE ATT&CK) vient du renseignement sur les menaces, disponible à partir du plan ${plan}.`,
            `This risk is linked to ${cve}. The full advisory (CVSS, CISA-KEV, MITRE ATT&CK) comes from threat intelligence, available from the ${plan} plan.`,
          )}
          primaryAction={
            <Btn
              label={tr('Voir les plans', 'See plans')}
              onClick={() => navigate('/settings?tab=billing')}
            />
          }
        />
      </div>
    );
  }

  if (isLoading || cti.loading)
    return (
      <div className="py-5 px-[22px]">
        <SkeletonRows rows={4} />
      </div>
    );

  if (isError || !data) {
    // Not an empty state: the risk DOES name a CVE, so "nothing here" would be
    // a different and wrong claim.
    return (
      <div className="py-8 px-[22px]">
        <ErrorState
          title={tr('Avis indisponible', 'Advisory unavailable')}
          description={tr(
            `${cve} n’a pas pu être lu depuis le moteur CTI. Le flux n’a peut-être pas encore été synchronisé.`,
            `${cve} could not be read from the CTI engine. The feed may not have been synchronised yet.`,
          )}
          onRetry={() => navigate('/threat-map')}
          retryLabel={tr('Ouvrir Threat Intel', 'Open Threat Intel')}
        />
      </div>
    );
  }

  return (
    <div className="py-5 px-[22px] flex flex-col gap-4">
      <div className="flex items-center gap-2.5 flex-wrap">
        <span className="mono text-[13px] font-bold text-ink">{data.cve_id}</span>
        {data.cvss_v3 > 0 && (
          <span
            className="mono text-[12px] font-semibold px-2 py-0.5 rounded"
            style={{ background: 'var(--bg-hover)', color: 'var(--fg-secondary)' }}
          >
            CVSS {data.cvss_v3.toFixed(1)}
          </span>
        )}
        {data.cisa_known && (
          <span
            className="text-[11px] font-bold uppercase tracking-[.05em] px-2 py-0.5 rounded-full"
            style={{
              color: 'var(--critical)',
              background: 'color-mix(in srgb,var(--critical) 14%,transparent)',
            }}
          >
            CISA-KEV
          </span>
        )}
      </div>

      {data.description && (
        <div className="text-[13px] text-ink-soft leading-relaxed">{data.description}</div>
      )}

      {data.cisa_known && data.cisa_due_date && (
        <Fact
          label={tr('Échéance CISA', 'CISA due date')}
          value={when.calendarDate(data.cisa_due_date)}
        />
      )}
      {!!data.mitre_tactics?.length && (
        <Fact
          label={tr('Tactiques MITRE', 'MITRE tactics')}
          value={data.mitre_tactics.join(', ')}
        />
      )}
      {!!data.mitre_techniques?.length && (
        <Fact
          label={tr('Techniques MITRE', 'MITRE techniques')}
          value={data.mitre_techniques.join(', ')}
        />
      )}
      {data.remediation && (
        <Fact label={tr('Remédiation', 'Remediation')} value={data.remediation} />
      )}

      <Btn
        label={tr('Voir dans Threat Intel', 'View in Threat Intel')}
        onClick={() => navigate(`/threat-map?q=${encodeURIComponent(data.cve_id)}`)}
      />
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[11.5px] font-semibold text-ink-muted uppercase tracking-wide mb-1">
        {label}
      </div>
      <div className="text-[13px] text-ink leading-relaxed">{value}</div>
    </div>
  );
}

// DrawerAI — the "IA" tab: generates a synthesis + suggested treatment plan for
// this risk via the live /ai/risks/:id/treatment-plan endpoint (spec §12.1). Claude
// when configured, deterministic template otherwise (shown in the provenance line).
export function DrawerAI({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const locale = lang === 'fr' ? 'fr' : 'en';
  const plan = useTreatmentPlan();
  const res = plan.data;

  const stratLabel: Record<string, string> = {
    mitigate: tr('Atténuer', 'Mitigate'),
    accept: tr('Accepter', 'Accept'),
    transfer: tr('Transférer', 'Transfer'),
    avoid: tr('Éviter', 'Avoid'),
  };
  const prioColor: Record<string, string> = {
    high: 'var(--critical)',
    medium: 'var(--high)',
    low: 'var(--accent)',
  };

  return (
    <div className="py-5 px-[22px]">
      <div className="flex items-start gap-2 mb-4">
        <div className="flex-1">
          <div className="text-[14px] font-semibold text-ink mb-1">
            {tr('Plan de traitement assisté par IA', 'AI-assisted treatment plan')}
          </div>
          <div className="text-[12.5px] text-ink-soft leading-relaxed">
            {tr(
              "L'IA synthétise ce risque et propose une stratégie et un plan d'actions, à partir de son score, sa criticité et l'actif lié.",
              'The AI synthesises this risk and proposes a strategy and action plan from its score, criticality and linked asset.',
            )}
          </div>
        </div>
      </div>

      <button
        onClick={() => plan.mutate({ riskId: r.id, locale })}
        disabled={plan.isPending}
        className="w-full h-11 rounded-[12px] flex items-center justify-center gap-2 text-fg-primary text-[13.5px] font-semibold disabled:opacity-60"
        style={{ background: 'var(--accent-solid)', color: 'var(--fg-on-solid)' }}
      >
        {plan.isPending ? <Loader2 size={17} className="animate-spin" /> : <Sparkles size={17} />}
        {plan.isPending
          ? tr('Génération…', 'Generating…')
          : res
            ? tr('Régénérer', 'Regenerate')
            : tr('Générer avec l’IA', 'Generate with AI')}
      </button>

      {plan.isPending && (
        // Informative wait (UX-09): the LLM call takes a few seconds — surface the
        // conceptual steps + the risk being analysed instead of a bare spinner.
        <ProgressState
          title={tr('Analyse en cours…', 'Analysing…')}
          stat={tr(`Risque : ${r.name}`, `Risk: ${r.name}`)}
          steps={[
            tr('Analyse du risque et de l’actif lié', 'Analysing the risk and linked asset'),
            tr('Choix de la stratégie de traitement', 'Choosing the treatment strategy'),
            tr('Priorisation du plan d’actions', 'Prioritising the action plan'),
          ]}
        />
      )}

      {plan.isError && (
        <div className="mt-4 text-[12.5px]" style={{ color: 'var(--critical)' }}>
          {tr('La génération a échoué. Réessayez.', 'Generation failed. Please try again.')}
        </div>
      )}

      {res && (
        <div
          className="mt-5 space-y-4"
          style={{ animation: 'or-fadeup var(--dur-slow) var(--ease-out)' }}
        >
          <div>
            <div className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide mb-1.5">
              {tr('Synthèse', 'Summary')}
            </div>
            <div className="text-[13px] text-ink leading-relaxed">{res.plan.summary}</div>
          </div>

          <div className="flex items-center gap-2">
            <span className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide">
              {tr('Stratégie', 'Strategy')}
            </span>
            <span
              className="text-[12px] font-semibold px-2.5 py-1 rounded-full"
              style={{
                background: 'var(--accent-soft)',
                color: 'var(--accent-500)',
                border: '1px solid var(--accent-line)',
              }}
            >
              {stratLabel[res.plan.recommended_strategy] ?? res.plan.recommended_strategy}
            </span>
          </div>

          <div>
            <div className="text-[11px] font-semibold text-ink-muted uppercase tracking-wide mb-2">
              {tr('Plan d’actions', 'Action plan')}
            </div>
            <div className="space-y-2.5">
              {res.plan.actions.map((a, i) => (
                <div
                  key={i}
                  className="p-3 rounded-[11px]"
                  style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span
                      className="w-5 h-5 rounded-full flex items-center justify-center text-[11px] font-bold text-fg-primary shrink-0"
                      style={{ background: prioColor[a.priority] ?? 'var(--accent)' }}
                    >
                      {i + 1}
                    </span>
                    <span className="text-[13px] font-semibold text-ink flex-1">{a.title}</span>
                    <span
                      className="text-[10.5px] font-semibold uppercase"
                      style={{ color: prioColor[a.priority] ?? 'var(--accent)' }}
                    >
                      {a.priority}
                    </span>
                  </div>
                  <div className="text-[12.5px] text-ink-soft leading-relaxed pl-7">
                    {a.description}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {res.plan.rationale && (
            <div className="text-[12px] text-ink-soft italic leading-relaxed">
              {res.plan.rationale}
            </div>
          )}

          <div
            className="text-[11px] text-ink-muted pt-1"
            style={{ borderTop: '1px solid var(--border)' }}
          >
            <span className="pt-2 inline-block">
              {tr('Généré par', 'Generated by')} :{' '}
              <span className="font-semibold">{res.generated_by}</span>
              {res.generated_by === 'template' &&
                ' · ' +
                  tr(
                    'mode local (configurez ANTHROPIC_API_KEY pour Claude)',
                    'local mode (set ANTHROPIC_API_KEY for Claude)',
                  )}
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

export function DrawerDetails({ r, onCreateMiti }: { r: UiRisk; onCreateMiti: () => void }) {
  const L = useUIStrings();
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const updateRisk = useRiskStore((s) => s.updateRisk);
  const { data: mappings } = useRiskMappings(r.id);
  const deleteMapping = useDeleteMapping(r.id);
  const createMapping = useCreateMapping(r.id);
  const [mappingDrafts, setMappingDrafts] = useState<MappingDraft[]>([]);
  const [savingOwner, setSavingOwner] = useState(false);
  const mitiCount = r.raw.mitigations_count ?? r.raw.mitigations?.length ?? 0;

  const field = (lbl: string, val: string) => (
    <div className="mb-4">
      <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-1.5">
        {lbl}
      </div>
      <div className="text-[13.5px] text-ink">{val}</div>
    </div>
  );

  // Assignment saves immediately. It is a one-field decision with an obvious
  // undo (pick someone else), so a Save button would only add a step people
  // forget — which is how "j'ai assigné mais rien ne s'est passé" happens.
  const assign = async (role: OwnershipRole, userId: string | null) => {
    setSavingOwner(true);
    try {
      await updateRisk(r.id, ownershipPatch({ [role]: userId }));
      toast.success(
        userId
          ? tr('Affectation enregistrée', 'Assignment saved')
          : tr('Affectation retirée', 'Assignment removed'),
      );
    } catch {
      toast.error(tr("L'affectation a échoué.", 'The assignment failed.'));
    } finally {
      setSavingOwner(false);
    }
  };

  const addMappings = async () => {
    for (const d of mappingDrafts) {
      try {
        await createMapping.mutateAsync({
          framework_id: d.framework_id,
          control_id: d.control_id ?? null,
        });
      } catch {
        toast.error(tr(`Impossible de rattacher ${d.label}`, `Could not map ${d.label}`));
      }
    }
    setMappingDrafts([]);
  };

  return (
    <div className="px-[22px] py-5">
      {field('Description', r.desc || '—')}
      <div className="grid grid-cols-2 gap-x-5">
        {field(lang === 'fr' ? 'Actif concerné' : 'Asset', r.asset)}
        {field(L.col_category, r.categoryName || '—')}
        {field(L.col_mod, r.mod)}
        {field(L.col_tags, r.tags.length ? r.tags.join(', ') : '—')}
      </div>

      {/* Ownership — three slots, one picker, available right here rather than
          behind an edit modal. */}
      <div className="mb-5">
        <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">
          {tr('Responsabilités', 'Ownership')}
        </div>
        <OwnershipFields
          value={r.raw}
          onChange={assign}
          permission="risks:update"
          disabled={!canUpdate || savingOwner}
        />
      </div>

      {/* Compliance references — real controls, each one clickable through. */}
      <div className="mb-5">
        <div className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">
          {L.col_fw}
        </div>
        {(mappings ?? []).length === 0 ? (
          <p className="text-[12.5px] text-ink-muted mb-2">
            {tr(
              'Ce risque n’est rattaché à aucun contrôle.',
              'This risk is not linked to any control.',
            )}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1.5 mb-2">
            {(mappings ?? []).map((m) => (
              <span
                key={m.id}
                className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] font-semibold"
                style={{
                  background: 'color-mix(in srgb, var(--accent) 14%, transparent)',
                  color: 'var(--accent-500)',
                }}
              >
                <button
                  type="button"
                  onClick={() => navigate(mappingHref(m))}
                  className="hover:underline"
                >
                  {mappingLabel(m)}
                </button>
                {canUpdate ? (
                  <button
                    type="button"
                    onClick={() => deleteMapping.mutate(m.id)}
                    aria-label={tr('Retirer', 'Remove')}
                    className="opacity-70 hover:opacity-100"
                  >
                    <X size={11} />
                  </button>
                ) : null}
              </span>
            ))}
          </div>
        )}
        {canUpdate ? (
          <>
            <ComplianceMappingField
              value={mappingDrafts}
              onChange={setMappingDrafts}
              onImportFramework={() => navigate('/compliance')}
              disabled={createMapping.isPending}
            />
            {mappingDrafts.length > 0 ? (
              <button
                type="button"
                onClick={addMappings}
                className="mt-2 rounded-full px-3 py-1.5 text-[12px] font-semibold"
                style={{
                  background: 'var(--accent-solid)',
                  color: 'var(--fg-on-solid)',
                }}
              >
                {tr('Rattacher', 'Map')}
              </button>
            ) : null}
          </>
        ) : null}
      </div>

      {/* §6: the button is DERIVED from what exists, not from what the user did
          a moment ago. Once a plan exists, "Créer une mitigation" is the wrong
          offer — the useful one is to go and look at it. */}
      {mitiCount > 0 ? (
        <button
          onClick={() => navigate(`/risks/mitigations?risk_id=${r.id}`)}
          className="mt-2 w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold transition-all hover:brightness-110"
          style={{
            background: 'var(--bg-elevated)',
            border: '1px solid var(--border-strong)',
            color: 'var(--fg-secondary)',
          }}
        >
          <ShieldCheck size={16} />{' '}
          {tr(`Voir les mitigations (${mitiCount})`, `View mitigations (${mitiCount})`)}
        </button>
      ) : (
        <button
          onClick={onCreateMiti}
          className="mt-2 w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-fg-on-solid transition-all hover:brightness-110"
          style={{ background: 'var(--accent-solid)' }}
        >
          <ShieldCheck size={16} /> {L.createMiti}
        </button>
      )}
    </div>
  );
}

export function DrawerScore({ r }: { r: UiRisk }) {
  // The working comes from the server (#486). The three gauges that stood here
  // were computed client-side from the mapped row, and the asset-criticality one
  // could contradict the factor the Score Engine actually used — directly above
  // the verified working. The terms and their ranges are in the working itself.
  return (
    <div className="p-[22px]">
      <ScoreWorking riskId={r.id} storedScore={r.score} />
    </div>
  );
}

/* ---------------- smart score (multifactor, spec §8) ---------------- */
export function DrawerSmart({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const navigate = useNavigate();
  const { data, isLoading, isError } = useRiskSmartScore(r.id);

  if (isLoading) {
    return (
      <div className="p-[22px]">
        <SkeletonRows rows={5} />
      </div>
    );
  }
  if (isError || !data) {
    return (
      <div className="py-10 px-[22px] text-center text-[13px] text-ink-soft">
        {tr('Impossible de calculer le score intelligent.', 'Could not compute the smart score.')}
      </div>
    );
  }
  return (
    <div className="p-[22px]">
      <SmartRiskRadar data={data} lang={lang} />
      <button
        onClick={() => navigate('/risks/weighting')}
        className="mt-4 w-full h-9 rounded-[10px] flex items-center justify-center gap-2 text-[12.5px] font-semibold transition-all hover:bg-hover"
        style={{ border: '1px solid var(--border-strong)', color: 'var(--fg-secondary)' }}
      >
        <SlidersHorizontal size={15} /> {tr('Configurer les pondérations', 'Configure weights')}
      </button>
      <p className="mt-3 text-[11px] text-ink-muted leading-snug">
        {tr(
          'Score multifactoriel (0–100) : criticité métier, exposition Internet, vulnérabilités, maturité des contrôles, historique d’incidents, exploitabilité, valeur financière et menaces actives (CTI). Le score classique P × I × Criticité reste inchangé.',
          'Multifactor score (0–100): business criticality, internet exposure, vulnerabilities, control maturity, incident history, exploitability, financial value and active threats (CTI). The classic P × I × Criticality score is unchanged.',
        )}
      </p>
    </div>
  );
}

/* ---------------- lifecycle ---------------- */
//
// There is no client-side copy of the state graph any more. This used to hold a
// `canTransition` that "mirrors the backend" — and mirrors drift: the button the
// UI offered and the transition the server would accept stopped agreeing, which
// is how a risk reached "traité" with open sub-actions. <LifecycleStepper>
// renders GET /risks/:id/transitions verbatim, blockers included.

/** State pill for the register row, from the single canonical lifecycle. */
export function PhasePill({ phase, lang }: { phase: RiskPhase; lang: LocaleCode }) {
  const closed = phase === 'closed';
  // The text step on the accent's own tint: --accent is the mark step and
  // measured 4.43:1 here on the azure variant (#793).
  const col = closed ? 'var(--fg-secondary)' : 'var(--accent-500)';
  const labels: Record<RiskPhase, [string, string]> = {
    identified: ['Identifié', 'Identified'],
    analyzed: ['Évalué', 'Assessed'],
    evaluated: ['Traitement planifié', 'Treatment planned'],
    treated: ['En traitement', 'In treatment'],
    monitored: ['Traité', 'Mitigated'],
    closed: ['Clôturé', 'Closed'],
  };
  return (
    <span
      className="inline-flex items-center gap-1.5 h-[22px] px-2.5 rounded-full text-[11.5px] font-semibold"
      style={{ color: col, background: 'var(--accent-soft)' }}
    >
      <RouteIcon size={12} /> {labels[phase]?.[lang === 'fr' ? 0 : 1] ?? phase}
    </span>
  );
}

export function DrawerLifecycle({
  r,
  onOpenMitigations,
}: {
  r: UiRisk;
  onOpenMitigations?: () => void;
}) {
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  return (
    <div className="px-[22px] py-5">
      <div className="text-[13px] text-ink-soft mb-4">
        {tr(
          'Le cycle de vie et le plan de mitigation sont un seul flux : une étape se débloque quand le travail correspondant est fait, pas quand on coche une case.',
          'The lifecycle and the mitigation plan are one flow: a step unlocks when the work behind it is done, not when someone ticks a box.',
        )}
      </div>
      <LifecycleStepper
        riskId={r.id}
        onOpenMitigations={onOpenMitigations}
        onOpenGovernance={() => navigate('/governance?tab=approvals')}
      />
    </div>
  );
}

/* ---------------- financial (CRQ) ---------------- */
export function DrawerFinancial({ r }: { r: UiRisk }) {
  const lang = useUIStore((s) => s.lang);
  const tr = (fr: string, en: string) => (lang === 'fr' ? fr : en);
  const updateRisk = useRiskStore((s) => s.updateRisk);
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const qc = useQueryClient();
  const raw = r.raw;
  // Live full assessment from the backend (SLE, downtime, worst/avg, ROSI).
  const { data: fin } = useRiskFinancial(r.id);

  const s = (v: number | null | undefined) => (v == null ? '' : String(v));
  const [sle, setSle] = useState(s(raw.sle_xaf));
  const [aro, setAro] = useState(s(raw.aro));
  const [dh, setDh] = useState(s(raw.downtime_hours));
  const [hc, setHc] = useState(s(raw.hourly_downtime_cost_xaf));
  const [dl, setDl] = useState(s(raw.data_loss_cost_xaf));
  const [fn, setFn] = useState(s(raw.fines_xaf));
  const [oc, setOc] = useState(s(raw.other_direct_cost_xaf));
  const [rc, setRc] = useState(s(raw.remediation_cost_xaf));
  const [eff, setEff] = useState(
    raw.mitigation_effectiveness != null ? Number(raw.mitigation_effectiveness) : 0,
  );
  const [busy, setBusy] = useState(false);
  const fmt = useFormat();

  // Money goes through `Intl` in the reader's locale: XAF prints "FCFA" with no
  // minor unit and USD prints "$" with cents suppressed, in both languages.
  const fmtXAF = (v?: number) =>
    v == null ? '—' : fmt.currency(Math.round(v), { currency: 'XAF' });
  const fmtUSD = (v?: number) =>
    v == null ? '—' : fmt.currency(v, { currency: 'USD', fractionDigits: 0 });
  const fmtPct = (ratio: number) => `${ratio >= 0 ? '+' : ''}${Math.round(ratio * 100)}%`;
  const num = (v: string) => (v.trim() === '' ? null : Number(v));

  const save = async () => {
    setBusy(true);
    try {
      await updateRisk(r.id, {
        sle_xaf: num(sle),
        aro: num(aro),
        downtime_hours: num(dh),
        hourly_downtime_cost_xaf: num(hc),
        data_loss_cost_xaf: num(dl),
        fines_xaf: num(fn),
        other_direct_cost_xaf: num(oc),
        remediation_cost_xaf: num(rc),
        mitigation_effectiveness: eff,
      });
      await qc.invalidateQueries({ queryKey: ['financial'] });
      toast.success(tr('Exposition recalculée', 'Exposure recalculated'));
    } catch {
      toast.error(tr('Échec du recalcul', 'Recalculation failed'));
    } finally {
      setBusy(false);
    }
  };

  const stat = (label: string, value: string, tone?: string) => (
    <div
      className="rounded-[10px] p-2.5"
      style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}
    >
      <div className="text-[10px] uppercase tracking-[.05em] text-ink-muted">{label}</div>
      <div className="mono text-[14px] font-bold mt-0.5" style={{ color: tone ?? 'var(--ink)' }}>
        {value}
      </div>
    </div>
  );
  const field = (label: string, val: string, set: (v: string) => void, step?: string) => (
    <label className="block">
      <span className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
        {label}
      </span>
      <input
        value={val}
        onChange={(e) => set(e.target.value)}
        type="number"
        step={step}
        min={0}
        className="mt-1.5 w-full rounded-[10px] px-3 py-2 text-[13px] text-ink outline-none"
        style={{ background: 'var(--bg-secondary)', border: '1px solid var(--border)' }}
      />
    </label>
  );

  return (
    <div className="px-[22px] py-5">
      <div className="text-[13px] text-ink-soft mb-4">
        {tr(
          'Quantification financière (FAIR/CRQ) — ALE = SLE × ARO ; ROSI = (ALE − ALE résiduel − remédiation) / remédiation.',
          'Financial quantification (FAIR/CRQ) — ALE = SLE × ARO; ROSI = (ALE − residual ALE − remediation) / remediation.',
        )}
      </div>

      {/* Headline ALE (XAF + USD) */}
      <div className="grid grid-cols-2 gap-3 mb-3">
        <div
          className="rounded-[12px] p-4"
          style={{
            border: '1px solid color-mix(in srgb,var(--accent) 30%,transparent)',
            background: 'color-mix(in srgb,var(--accent) 6%,transparent)',
          }}
        >
          <div className="text-[10.5px] uppercase tracking-[.06em] text-ink-muted">ALE (FCFA)</div>
          <div className="mono text-[20px] font-bold text-ink mt-1">
            {fmtXAF(fin?.ale.xaf ?? raw.ale_xaf)}
          </div>
        </div>
        <div
          className="rounded-[12px] p-4"
          style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}
        >
          <div className="text-[10.5px] uppercase tracking-[.06em] text-ink-muted">ALE (USD)</div>
          <div className="mono text-[20px] font-bold text-ink mt-1">
            {fmtUSD(fin?.ale.usd ?? raw.ale_usd)}
          </div>
        </div>
      </div>

      {/* Full assessment breakdown */}
      {fin && (
        <div className="grid grid-cols-3 gap-2 mb-3">
          {stat('SLE', fmtXAF(fin.sle.xaf))}
          {stat(tr('Coût interruptions', 'Downtime cost'), fmtXAF(fin.downtime_cost.xaf))}
          {stat(
            tr('Pire cas (ALE)', 'Worst-case ALE'),
            fmtXAF(fin.ale_worst.xaf),
            'var(--critical)',
          )}
          {stat(tr('ALE moyen', 'Average ALE'), fmtXAF(fin.ale_average.xaf))}
          {stat(tr('ALE résiduel', 'Residual ALE'), fmtXAF(fin.ale_after.xaf), 'var(--low)')}
          {stat(
            'ROSI',
            fin.rosi_computable ? fmtPct(fin.rosi) : '—',
            fin.rosi_computable ? (fin.rosi >= 0 ? 'var(--low)' : 'var(--critical)') : undefined,
          )}
        </div>
      )}
      <div className="text-[11.5px] text-ink-muted mb-4">
        {tr('Base SLE : ', 'SLE basis: ')}
        {fin?.sle_basis === 'explicit'
          ? tr('saisie explicite', 'explicit input')
          : fin?.sle_basis === 'composed'
            ? tr(
                'composé (interruptions + amendes + perte de données)',
                'composed (downtime + fines + data loss)',
              )
            : tr('valeur de référence par criticité', 'reference value by criticality')}
      </div>

      {canUpdate && (
        <div
          className="rounded-[12px] p-3.5"
          style={{ border: '1px solid var(--border)', background: 'var(--bg-hover)' }}
        >
          <div className="text-[11px] font-semibold uppercase tracking-[.05em] text-ink-muted mb-2">
            {tr('Pertes', 'Losses')}
          </div>
          <div className="grid grid-cols-2 gap-3 mb-3">
            {field(tr('SLE explicite (FCFA)', 'Explicit SLE (FCFA)'), sle, setSle)}
            {field(tr('ARO — Fréquence / an', 'ARO — Frequency / yr'), aro, setAro, '0.1')}
            {field(tr('Heures d’interruption', 'Downtime hours'), dh, setDh, '0.5')}
            {field(tr('Coût horaire (FCFA)', 'Hourly cost (FCFA)'), hc, setHc)}
            {field(tr('Perte de données (FCFA)', 'Data loss (FCFA)'), dl, setDl)}
            {field(tr('Amendes (FCFA)', 'Fines (FCFA)'), fn, setFn)}
            {field(tr('Autre coût direct (FCFA)', 'Other direct cost (FCFA)'), oc, setOc)}
          </div>
          <div className="text-[11px] font-semibold uppercase tracking-[.05em] text-ink-muted mb-2">
            {tr('Remédiation (ROSI)', 'Remediation (ROSI)')}
          </div>
          <div className="grid grid-cols-2 gap-3 mb-3 items-end">
            {field(tr('Coût de remédiation (FCFA)', 'Remediation cost (FCFA)'), rc, setRc)}
            <label className="block">
              <span className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted">
                {tr('Efficacité', 'Effectiveness')}
                <span className="mono text-ink">{Math.round(eff * 100)}%</span>
              </span>
              <input
                value={eff}
                onChange={(e) => setEff(Number(e.target.value))}
                type="range"
                min={0}
                max={1}
                step={0.05}
                className="mt-3 w-full accent-(--accent)"
              />
            </label>
          </div>
          <button
            disabled={busy}
            onClick={save}
            className="w-full h-10 rounded-[10px] flex items-center justify-center gap-2 text-[13px] font-semibold text-fg-on-solid disabled:opacity-60"
            style={{ background: 'var(--accent-solid)', color: 'var(--fg-on-solid)' }}
          >
            <Coins size={16} /> {tr('Recalculer l’exposition', 'Recalculate exposure')}
          </button>
          <div className="text-[11px] text-ink-muted mt-2">
            {tr(
              'SLE explicite prioritaire ; sinon composé depuis interruptions + amendes + perte de données ; sinon référence par criticité.',
              'Explicit SLE wins; else composed from downtime + fines + data loss; else reference by criticality.',
            )}
          </div>
        </div>
      )}
    </div>
  );
}
