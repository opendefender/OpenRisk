// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The risk drawer on the October 2026 redesign (#913): 560 px, a header with
// id · category, copy-link and close, the title, band and status pills, then a
// strip of tabs: the design's Aperçu / Mitigations / Historique first, and after
// them every tab the product already had (responsibilities and control mappings,
// lifecycle, score working, SmartScore, financial, CTI, AI). Nothing is hidden
// behind a menu; the strip scrolls sideways inside its own box.
//
// Aperçu edits in place and saves as you go: P and I through the 5-level
// pickers (level k writes the midpoint of its bucket), status and owner through
// listboxes. The three figures are labelled for what they are — inherent score,
// inherent ALE, residual score — in the one display format of the register.
// "Demander l'acceptation" and "Clore le risque" go through the lifecycle
// transitions endpoint, so they are enabled only when the server offers them.

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router';
import { toast } from 'sonner';
import { Database, Download, Link2, Pencil, Plus, X } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useRiskStore } from '../../hooks/useRiskStore';
import { formatDate } from '../../i18n/format';
import {
  Badge,
  Button,
  Listbox,
  TabPanel,
  Tabs,
  severityIntent,
  useDismissableLayer,
  type ListboxOption,
  type TabItem,
  type MatrixBucket,
} from '../../shared/ds';
import { bandTextColor } from '../../services/scoreService';
import { ownershipPatch, type Assignee } from '../../services/ownershipService';
import { mappingHref, mappingLabel } from '../../services/taxonomyService';
import { millions } from '../financial/money';
import { memberName } from './riskMembers';
import { TabLabel } from './RegisterToolbar';
import { LevelPicker } from './LevelPicker';
import {
  DrawerAI,
  DrawerCTI,
  DrawerDetails,
  DrawerFinancial,
  DrawerLifecycle,
  DrawerScore,
  DrawerSmart,
  DrawerTimeline,
} from './RiskDrawerSections';
import { DRAWER_TABS, type DrawerTab } from './riskDrawerTarget';
import { formatImpact, formatProbability, formatScore } from './riskFormat';
import {
  LEVELS,
  impactForLevel,
  impactLevel,
  probabilityForLevel,
  probabilityLevel,
} from './riskLevels';
import type { UiRisk } from './riskMap';
import {
  registerStatusIntent,
  reviewState,
  RISK_STATUSES,
  type RiskStatusValue,
} from './riskViews';
import type { RegisterMoney } from './useRiskRegisterData';
import { useRiskTransitions, useTransitionRisk, type RiskState } from './useLifecycle';
import { useRiskMappings } from './useTaxonomy';

/** The statuses a person may pick; "closed" goes through the lifecycle. */
const PICKABLE: readonly RiskStatusValue[] = ['open', 'in_progress', 'mitigated', 'accepted'];
const NO_OWNER = 'none';

interface RiskDrawerProps {
  r: UiRisk;
  /** The tab the drawer opens on (a deep link may name one). */
  initialTab: DrawerTab;
  members: readonly Assignee[];
  money: RegisterMoney;
  onClose: () => void;
  onEdit: () => void;
  onExport: () => void;
  onCreateMiti: () => void;
  /** A write the list does not mirror (a transition): reload list and counts. */
  onChanged: () => void;
}

export function RiskDrawer({
  r,
  initialTab,
  members,
  money,
  onClose,
  onEdit,
  onExport,
  onCreateMiti,
  onChanged,
}: RiskDrawerProps) {
  const { t } = useI18n();
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const [tab, setTab] = useState<DrawerTab>(initialTab);
  const panelRef = useRef<HTMLDivElement>(null);
  // Escape closes, focus moves in on open and back to the row on close, and Tab
  // stays inside, as for every shared overlay.
  useDismissableLayer(panelRef, { open: true, onClose });

  const mitiCount = r.raw.mitigations_count ?? r.raw.mitigations?.length ?? 0;
  const tabsId = `risk-drawer-${r.id}`;
  const tabItems: TabItem<DrawerTab>[] = DRAWER_TABS.map((id) => ({
    id,
    label:
      id === 'miti' ? (
        <TabLabel text={t(`riskRegister.drawer.tab.${id}`)} count={mitiCount} />
      ) : (
        t(`riskRegister.drawer.tab.${id}`)
      ),
    testId: `risk-tab-${id}`,
  }));
  // Keep the active tab in view when it changes (click, deep link, arrows).
  useEffect(() => {
    document
      .getElementById(`${tabsId}-tab-${tab}`)
      ?.scrollIntoView?.({ inline: 'nearest', block: 'nearest' });
  }, [tab, tabsId]);

  const copyLink = async () => {
    const url = `${window.location.origin}/risks?focus=${encodeURIComponent(r.id)}`;
    try {
      await navigator.clipboard.writeText(url);
      toast.success(t('riskRegister.drawer.linkCopied'));
    } catch {
      toast.error(t('riskRegister.drawer.linkCopyFailed'));
    }
  };

  const iconBtn = 'size-[30px] rounded-lg inline-flex items-center justify-center text-ink-muted';

  // Portalled to document.body: the page frame carries a transform (animation
  // fill-mode), and a transformed ancestor would become the containing block of
  // this `position: fixed` panel and anchor it to the page instead of the screen.
  return createPortal(
    <div
      className="fixed inset-0 z-70 flex justify-end"
      style={{
        background: 'var(--surface-overlay)',
        backdropFilter: 'blur(var(--overlay-blur))',
        animation: 'or-fadein var(--motion-enter)',
      }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="risk-drawer-title"
        tabIndex={-1}
        data-testid="risk-drawer"
        className="h-full flex flex-col bg-surface-1 border-l border-border-default outline-none"
        style={{
          width: 'min(100vw, 560px)',
          boxShadow: 'var(--shadow-lg)',
          animation: 'or-slidein var(--motion-panel)',
        }}
      >
        <div className="px-5 pt-[18px] shrink-0">
          <div className="flex items-center gap-2">
            <span className="mono text-[12px] text-ink-muted">{r.id.slice(0, 8)}</span>
            {r.categoryName && (
              <>
                <span aria-hidden="true" className="text-ink-muted">
                  ·
                </span>
                <span className="text-[12px] text-ink-muted truncate">{r.categoryName}</span>
              </>
            )}
            <span className="flex-1" />
            {canUpdate && (
              <button
                type="button"
                onClick={onEdit}
                aria-label={t('riskRegister.drawer.edit')}
                title={t('riskRegister.drawer.edit')}
                className={`${iconBtn} hover:bg-surface-3 hover:text-ink`}
              >
                <Pencil size={15} aria-hidden="true" />
              </button>
            )}
            <button
              type="button"
              onClick={onExport}
              aria-label={t('riskRegister.drawer.export')}
              title={t('riskRegister.drawer.export')}
              className={`${iconBtn} hover:bg-surface-3 hover:text-ink`}
            >
              <Download size={15} aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={() => void copyLink()}
              aria-label={t('riskRegister.drawer.copyLink')}
              title={t('riskRegister.drawer.copyLink')}
              className={`${iconBtn} hover:bg-surface-3 hover:text-ink`}
            >
              <Link2 size={15} aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={onClose}
              aria-label={t('riskRegister.drawer.close')}
              title={t('riskRegister.drawer.closeHint')}
              className={`${iconBtn} hover:bg-surface-3 hover:text-ink`}
            >
              <X size={17} aria-hidden="true" />
            </button>
          </div>
          <h2
            id="risk-drawer-title"
            className="disp m-0 mt-2 mb-2.5 text-[21px] font-semibold leading-tight tracking-[-0.01em] text-ink"
          >
            {r.name}
          </h2>
          <div className="flex gap-1.5 flex-wrap">
            <Badge intent={r.bandKnown ? (severityIntent[r.crit] ?? 'neutral') : 'neutral'} dot>
              {r.bandKnown ? t(`risks.band.${r.crit}`) : t('risks.band.unknown')}
            </Badge>
            <Badge intent={registerStatusIntent[r.statusValue] ?? 'neutral'}>
              {t(`riskRegister.status.${r.statusValue}`)}
            </Badge>
          </div>
          {/* Every tab stays a visible tab. The strip scrolls sideways inside its
              own box: no scrollbar chrome, a fade on the right edge says there is
              more, the active tab is scrolled into view, and the arrows, Home and
              End of the Tabs pattern move through it (focus scrolls natively). */}
          <div className="relative mt-4">
            <Tabs<DrawerTab>
              id={tabsId}
              label={t('riskRegister.drawer.sections')}
              className="gap-5 pr-8 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [mask-image:linear-gradient(to_right,#000_calc(100%-2rem),transparent)]"
              value={tab}
              onChange={setTab}
              items={tabItems}
            />
          </div>
        </div>

        <TabPanel tabsId={tabsId} id={tab} active className="flex-1 min-h-0 overflow-y-auto">
          {tab === 'overview' && (
            <Overview r={r} members={members} money={money} onChanged={onChanged} />
          )}
          {tab === 'miti' && (
            <Mitigations r={r} onCreateMiti={onCreateMiti} canUpdate={canUpdate} />
          )}
          {tab === 'history' && <DrawerTimeline r={r} />}
          {tab === 'ownership' && <DrawerDetails r={r} onCreateMiti={onCreateMiti} />}
          {tab === 'lifecycle' && (
            <DrawerLifecycle r={r} onOpenMitigations={() => setTab('miti')} />
          )}
          {tab === 'score' && <DrawerScore r={r} />}
          {tab === 'smart' && <DrawerSmart r={r} />}
          {tab === 'financial' && <DrawerFinancial r={r} />}
          {tab === 'cti' && <DrawerCTI r={r} />}
          {tab === 'ai' && <DrawerAI r={r} />}
        </TabPanel>

        <Footer r={r} onChanged={onChanged} onClose={onClose} />
      </div>
    </div>,
    document.body,
  );
}

/* ------------------------------------------------------------------ footer */

function Footer({
  r,
  onChanged,
  onClose,
}: {
  r: UiRisk;
  onChanged: () => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const transitions = useRiskTransitions(r.id);
  const transition = useTransitionRisk(r.id);
  const [pending, setPending] = useState<RiskState | null>(null);

  const option = (to: RiskState) => transitions.data?.options.find((o) => o.to === to);
  const accept = option('residual_accepted');
  const close = option('closed');

  if (!canUpdate) return null;

  const run = async (to: RiskState, doneKey: string, closeAfter: boolean) => {
    setPending(to);
    try {
      await transition.mutateAsync({ to });
      toast.success(t(doneKey));
      onChanged();
      if (closeAfter) onClose();
    } catch (err) {
      // The server's refusal is the explanation; a generic message would drop it.
      const message = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(message ?? t('riskRegister.drawer.transitionRefused'));
    } finally {
      setPending(null);
    }
  };

  const reasonOf = (o: ReturnType<typeof option>): string | undefined => {
    if (transitions.isError) return t('riskRegister.drawer.transitionsError');
    if (!o) return transitions.isLoading ? undefined : t('riskRegister.drawer.transitionNone');
    return o.allowed ? undefined : o.reason || t('riskRegister.drawer.transitionBlocked');
  };

  const acceptReason = reasonOf(accept);
  const closeReason = reasonOf(close);
  const busy = pending !== null;

  return (
    <div className="shrink-0 flex flex-wrap items-center gap-2 px-5 py-3.5 border-t border-border-subtle">
      <span title={acceptReason}>
        <Button
          variant="secondary"
          disabled={!accept?.allowed || busy}
          loading={pending === 'residual_accepted'}
          onClick={() =>
            void run('residual_accepted', 'riskRegister.drawer.acceptRequested', false)
          }
          data-testid="risk-accept"
        >
          {t('riskRegister.drawer.accept')}
        </Button>
      </span>
      {transitions.isError && (
        <button
          type="button"
          onClick={() => void transitions.refetch()}
          className="text-[12px] font-semibold text-accent-strong hover:underline"
        >
          {t('riskRegister.retry')}
        </button>
      )}
      <span className="flex-1" />
      <span title={closeReason}>
        <Button
          variant="primary"
          disabled={!close?.allowed || busy}
          loading={pending === 'closed'}
          onClick={() => void run('closed', 'riskRegister.drawer.closed', true)}
          data-testid="risk-close"
        >
          {t('riskRegister.drawer.closeRisk')}
        </Button>
      </span>
      {(acceptReason || closeReason) && (
        <p
          className="basis-full m-0 text-[11.5px] text-ink-muted"
          data-testid="risk-transition-hint"
        >
          {acceptReason && closeReason && acceptReason !== closeReason
            ? `${t('riskRegister.drawer.accept')} : ${acceptReason} · ${t('riskRegister.drawer.closeRisk')} : ${closeReason}`
            : (acceptReason ?? closeReason)}
        </p>
      )}
    </div>
  );
}

/* ---------------------------------------------------------------- overview */

const sectionLabel = 'text-[12px] font-semibold text-ink-soft';

function Overview({
  r,
  members,
  money,
  onChanged,
}: {
  r: UiRisk;
  members: readonly Assignee[];
  money: RegisterMoney;
  onChanged: () => void;
}) {
  const { t, locale } = useI18n();
  const canUpdate = useAuthStore((s) => s.hasPermission('risks:update'));
  const updateRisk = useRiskStore((s) => s.updateRisk);
  const [saving, setSaving] = useState<'p' | 'i' | 'status' | 'owner' | null>(null);
  // The review is judged against the moment the drawer opened.
  const [now] = useState(() => new Date());

  const pNames = LEVELS.map((k) => t(`risks.level.probability.${k}`));
  const iNames = LEVELS.map((k) => t(`risks.level.impact.${k}`));

  const write = async (
    field: NonNullable<typeof saving>,
    patch: Record<string, string | number | null>,
    okMessage: string,
  ) => {
    setSaving(field);
    try {
      await updateRisk(r.id, patch);
      toast.success(okMessage);
      // The PATCH answer carries neither the trend nor the ALE: re-read the page.
      onChanged();
    } catch {
      toast.error(t('riskRegister.drawer.saveFailed'));
    } finally {
      setSaving(null);
    }
  };

  const statusOptions: ListboxOption[] = RISK_STATUSES.map((s) => ({
    value: s,
    label: t(`riskRegister.status.${s}`),
    disabled: !PICKABLE.includes(s),
  }));
  const ownerOptions: ListboxOption[] = [
    { value: NO_OWNER, label: t('riskRegister.drawer.unassigned') },
    ...members.map((m) => ({ value: m.user_id, label: memberName(m) })),
  ];

  const review = reviewState(r.nextReviewAt, r.statusValue, now);
  const reviewText =
    review.kind === 'closed'
      ? t('riskRegister.review.closed')
      : review.kind === 'none' || !r.nextReviewAt
        ? t('riskRegister.review.none')
        : formatDate(locale, r.nextReviewAt, { day: 'numeric', month: 'short', year: 'numeric' });
  const reviewTone =
    review.kind === 'overdue'
      ? 'text-danger-text'
      : review.kind === 'soon'
        ? 'text-warning-text'
        : 'text-ink';

  // The figure big, its unit small beside it: a long unit never truncates the number.
  const ale =
    money.entitled && r.ale ? (
      <>
        {millions(money.toDisplay(r.ale), locale)}
        <span className="text-[11px] font-medium text-ink-muted"> M {money.currency}</span>
      </>
    ) : (
      '—'
    );

  const figure = (label: string, value: ReactNode, tone?: string, hint?: string) => (
    <div className="px-3.5 py-3 min-w-0" title={hint}>
      <div className="text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted">
        {label}
      </div>
      <div
        className="mono mt-1 flex flex-wrap items-baseline gap-x-1 text-[22px] font-semibold leading-tight"
        style={tone ? { color: tone } : undefined}
      >
        {value}
      </div>
    </div>
  );

  return (
    <div className="px-5 py-[18px]" data-testid="risk-overview">
      <div className="grid grid-cols-2 gap-3 mb-4">
        <LevelPicker
          label={t('riskRegister.drawer.probability')}
          level={probabilityLevel(r.prob)}
          levelNames={pNames}
          exact={formatProbability(locale, r.prob)}
          disabled={!canUpdate || saving !== null}
          testId="risk-pick-p"
          onPick={(k: MatrixBucket) =>
            void write(
              'p',
              { probability: probabilityForLevel(k) },
              t('riskRegister.drawer.levelSet', {
                field: t('riskRegister.drawer.probability'),
                level: k,
              }),
            )
          }
        />
        <LevelPicker
          label={t('riskRegister.drawer.impact')}
          level={impactLevel(r.impact)}
          levelNames={iNames}
          exact={formatImpact(locale, r.impact)}
          disabled={!canUpdate || saving !== null}
          testId="risk-pick-i"
          onPick={(k: MatrixBucket) =>
            void write(
              'i',
              { impact: impactForLevel(k) },
              t('riskRegister.drawer.levelSet', {
                field: t('riskRegister.drawer.impact'),
                level: k,
              }),
            )
          }
        />
      </div>

      <div
        className="grid grid-cols-3 border border-border-subtle rounded-xl mb-[18px] divide-x divide-border-subtle"
        data-testid="risk-figures"
      >
        {figure(
          t('riskRegister.drawer.inherentScore'),
          formatScore(locale, r.score),
          r.bandKnown ? bandTextColor(r.crit) : undefined,
        )}
        {figure(
          t('riskRegister.drawer.inherentAle'),
          ale,
          undefined,
          money.entitled ? undefined : t('riskRegister.ale.locked'),
        )}
        {figure(t('riskRegister.drawer.residualScore'), formatScore(locale, r.residual))}
      </div>

      <div className="grid grid-cols-2 gap-3 mb-[18px]">
        <label className="grid gap-1.5 min-w-0">
          <span className={sectionLabel}>{t('riskRegister.drawer.status')}</span>
          <Listbox
            size="sm"
            value={r.statusValue}
            options={statusOptions}
            disabled={!canUpdate || saving !== null}
            onChange={(v) =>
              void write(
                'status',
                { status: v },
                t('riskRegister.drawer.statusSet', { status: t(`riskRegister.status.${v}`) }),
              )
            }
            data-testid="risk-drawer-status"
          />
        </label>
        <label className="grid gap-1.5 min-w-0">
          <span className={sectionLabel}>{t('riskRegister.drawer.owner')}</span>
          <Listbox
            size="sm"
            value={r.ownerId ?? NO_OWNER}
            options={ownerOptions}
            disabled={!canUpdate || saving !== null}
            onChange={(v) =>
              void write(
                'owner',
                ownershipPatch({ owner: v === NO_OWNER ? null : v }),
                t('riskRegister.drawer.ownerSet'),
              )
            }
            data-testid="risk-drawer-owner"
          />
        </label>
      </div>

      <div className={`${sectionLabel} mb-1.5`}>{t('riskRegister.drawer.scenario')}</div>
      <p className="m-0 mb-[18px] text-[13.5px] leading-relaxed text-ink">
        {r.desc && r.desc !== '—' ? r.desc : t('riskRegister.drawer.noScenario')}
      </p>

      <div className={`${sectionLabel} mb-2`}>{t('riskRegister.drawer.assets')}</div>
      <div className="flex gap-1.5 flex-wrap mb-[18px]">
        {r.assetRefs.length === 0 ? (
          <span className="text-[12.5px] text-ink-muted">{t('riskRegister.drawer.noAssets')}</span>
        ) : (
          r.assetRefs.map((a) => (
            <Link
              key={a.id}
              to={`/assets?focus=${encodeURIComponent(a.id)}`}
              className="inline-flex items-center gap-1.5 h-[26px] px-2.5 rounded-lg border border-border-default bg-surface-2 text-[12.5px] text-ink hover:border-border-strong"
            >
              <Database size={13} aria-hidden="true" className="text-ink-muted" />
              {a.name}
            </Link>
          ))
        )}
      </div>

      <div className={`${sectionLabel} mb-2`}>{t('riskRegister.drawer.controls')}</div>
      <Controls riskId={r.id} />

      <div className="flex justify-between text-[12.5px] pt-3.5 border-t border-border-subtle">
        <span className="text-ink-soft">{t('riskRegister.drawer.nextReview')}</span>
        <span className={`font-semibold ${reviewTone}`} data-testid="risk-next-review">
          {reviewText}
        </span>
      </div>
    </div>
  );
}

function Controls({ riskId }: { riskId: string }) {
  const { t } = useI18n();
  const { data, isLoading, isError, refetch } = useRiskMappings(riskId);

  if (isLoading) {
    return (
      <div className="flex gap-1.5 mb-[18px]" aria-busy="true">
        <span className="or-skeleton h-[26px] w-24 rounded-lg" />
        <span className="or-skeleton h-[26px] w-20 rounded-lg" />
      </div>
    );
  }
  if (isError) {
    return (
      <p className="m-0 mb-[18px] text-[12.5px] text-ink-muted">
        {t('riskRegister.drawer.controlsError')}{' '}
        <button
          type="button"
          onClick={() => void refetch()}
          className="font-semibold text-accent-strong hover:underline"
        >
          {t('riskRegister.retry')}
        </button>
      </p>
    );
  }
  const mappings = data ?? [];
  return (
    <div className="flex gap-1.5 flex-wrap mb-[18px]">
      {mappings.length === 0 ? (
        <span className="text-[12.5px] text-ink-muted">{t('riskRegister.drawer.noControls')}</span>
      ) : (
        mappings.map((m) => (
          <Link
            key={m.id}
            to={mappingHref(m)}
            className="mono inline-flex items-center h-[26px] px-2.5 rounded-lg bg-surface-3 text-[12px] text-ink-soft hover:text-ink"
          >
            {mappingLabel(m)}
          </Link>
        ))
      )}
    </div>
  );
}

/* ------------------------------------------------------------- mitigations */

function Mitigations({
  r,
  onCreateMiti,
  canUpdate,
}: {
  r: UiRisk;
  onCreateMiti: () => void;
  canUpdate: boolean;
}) {
  const { t } = useI18n();
  const linked = r.raw.mitigations ?? [];

  const statusLabel = (status: string): string => {
    switch ((status ?? '').toLowerCase()) {
      case 'done':
        return t('riskRegister.drawer.miti.done');
      case 'in_progress':
        return t('riskRegister.drawer.miti.inProgress');
      case 'planned':
        return t('riskRegister.drawer.miti.planned');
      default:
        return status;
    }
  };

  return (
    <div className="px-5 py-[18px]" data-testid="risk-mitigations">
      {linked.length === 0 ? (
        <div className="p-6 text-center border border-dashed border-border-default rounded-xl text-[13px] text-ink-muted">
          {t('riskRegister.drawer.miti.empty')}
        </div>
      ) : (
        linked.map((m) => (
          <Link
            key={m.id}
            to={`/risks/mitigations?risk_id=${encodeURIComponent(r.id)}`}
            className="block p-3.5 border border-border-subtle rounded-xl mb-2.5 hover:border-border-strong"
          >
            <div className="flex justify-between gap-2.5">
              <span className="text-[13.5px] font-semibold text-ink">{m.title}</span>
              <span className="mono text-[12px] text-ink-muted shrink-0">{m.id.slice(0, 8)}</span>
            </div>
            <div className="flex items-center gap-2.5 mt-2.5">
              <span
                className="flex-1 h-1.5 rounded-md overflow-hidden"
                style={{ background: 'var(--chart-track)' }}
                role="progressbar"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={m.progress ?? 0}
                aria-label={m.title}
              >
                <span
                  className="block h-full bg-accent"
                  style={{ width: `${Math.min(100, Math.max(0, m.progress ?? 0))}%` }}
                />
              </span>
              <span className="mono text-[12px] text-ink">{m.progress ?? 0} %</span>
            </div>
            <div className="text-[12px] text-ink-muted mt-2">{statusLabel(m.status)}</div>
          </Link>
        ))
      )}
      {canUpdate && (
        <Button variant="secondary" icon={Plus} onClick={onCreateMiti} data-testid="risk-add-miti">
          {t('riskRegister.drawer.miti.add')}
        </Button>
      )}
    </div>
  );
}

/* ------------------------------------------------- loading / error of a deep link */

/**
 * The drawer's own loading and error states: a deep link to a risk that is not
 * on the loaded page fetches it, and the panel shows that is happening (or that
 * it failed, with the way out) instead of nothing.
 */
export function DrawerStatus({
  error,
  onRetry,
  onClose,
}: {
  error: boolean;
  onRetry: () => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const panelRef = useRef<HTMLDivElement>(null);
  useDismissableLayer(panelRef, { open: true, onClose });
  return createPortal(
    <div
      className="fixed inset-0 z-70 flex justify-end"
      style={{ background: 'var(--surface-overlay)', backdropFilter: 'blur(var(--overlay-blur))' }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={t('riskRegister.drawer.sections')}
        aria-busy={!error}
        tabIndex={-1}
        data-testid="risk-drawer-status"
        className="h-full flex flex-col bg-surface-1 border-l border-border-default outline-none"
        style={{ width: 'min(100vw, 560px)', boxShadow: 'var(--shadow-lg)' }}
      >
        <div className="flex justify-end px-5 pt-[18px]">
          <button
            type="button"
            onClick={onClose}
            aria-label={t('riskRegister.drawer.close')}
            className="size-[30px] rounded-lg inline-flex items-center justify-center text-ink-muted hover:bg-surface-3 hover:text-ink"
          >
            <X size={17} aria-hidden="true" />
          </button>
        </div>
        {error ? (
          <div className="px-5 py-8 text-center">
            <p className="m-0 text-[14px] font-semibold text-ink">
              {t('riskRegister.drawer.loadFailedTitle')}
            </p>
            <p className="m-0 mt-1 mb-4 text-[13px] text-ink-muted">
              {t('riskRegister.drawer.loadFailedBody')}
            </p>
            <Button variant="secondary" onClick={onRetry}>
              {t('riskRegister.retry')}
            </Button>
          </div>
        ) : (
          <div className="px-5 py-4 grid gap-3" aria-hidden="true">
            <span className="or-skeleton h-3 w-24 rounded-md" />
            <span className="or-skeleton h-7 w-3/4 rounded-md" />
            <span className="or-skeleton h-5 w-40 rounded-md" />
            <span className="or-skeleton h-32 w-full rounded-xl" />
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}
