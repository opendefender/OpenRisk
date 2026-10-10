// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Vendors on the October 2026 redesign (#908): a fast list and a clear
// profile, side by side. Each row carries the vendor's grade (the server's
// risk tier of its latest assessment, read A–D), its service and how critical
// it is, where its assessment stands and the next due date. The panel shows
// the selected vendor's risk score, the data it holds, its dates and owner,
// the weak points of its latest scored assessment, and the actions: send a
// questionnaire, create a risk already linked to it, open the full profile.
//
// A vendor is an asset of category vendor (#673, ADR 0004 D1), so "add a
// vendor" opens the asset form with the Supplier type. The list is searched
// and paged by the server; the selection lives in the URL (?v=<id>).

import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { useQueryClient } from '@tanstack/react-query';
import { ClipboardList, Plus, Search, X } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { formatDate, formatNumber } from '../../i18n/format';
import { FeatureGate } from '../../shared/FeatureGate';
import { Badge, Button } from '../../shared/ds';
import { PageFrame, PageHeader } from '../../shared/ui';
import { BlockEmpty, BlockError, BlockSkeleton } from '../dashboard/console/Panel';
import { CreateAssetModal } from '../assets/CreateAssetModal';
import { CreateRiskModal } from '../risks/CreateRiskModal';
import { SendAssessmentModal } from './SendAssessmentModal';
import {
  VENDORS_KEY,
  useVendorAssessment,
  useVendorAssessments,
  useVendorRecord,
  useVendorRegister,
} from './useVendors';
import { useVendorLabels } from './useVendorLabels';
import {
  knownStatus,
  knownTier,
  ratingLetter,
  standingIntent,
  standingOf,
  tierIntent,
  type VendorStanding,
} from './vendorDisplay';
import type { VendorAssessment, VendorRegisterEntry } from './vendorService';

const PAGE = 50;
const LETTER_COLOR: Record<string, string> = {
  A: 'var(--risk-low)',
  B: 'var(--chart-1)',
  C: 'var(--risk-high)',
  D: 'var(--risk-critical)',
};

export function VendorsPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [params, setParams] = useSearchParams();
  const selectedId = params.get('v');
  const canCreateAsset = useAuthStore((s) => s.hasPermission('assets:create'));
  const [creating, setCreating] = useState(false);
  const [draft, setDraft] = useState('');
  const [search, setSearch] = useState('');
  const [limit, setLimit] = useState(PAGE);

  // Debounced server search.
  useEffect(() => {
    const id = setTimeout(() => {
      setSearch(draft.trim());
      setLimit(PAGE);
    }, 250);
    return () => clearTimeout(id);
  }, [draft]);

  const register = useVendorRegister({ limit, offset: 0, ...(search ? { search } : {}) });
  // The critical count asks the server, so it is right beyond the first page.
  const critical = useVendorRegister({ limit: 1, offset: 0, service_criticality: 'critique' });
  const rows = useMemo(() => register.data?.items ?? [], [register.data]);
  const total = register.data?.total ?? 0;

  const select = (id: string | null) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (id) next.set('v', id);
        else next.delete('v');
        return next;
      },
      { replace: true },
    );

  const selected = rows.find((r) => r.id === selectedId) ?? null;

  // When the layout has wrapped (narrow screens), the profile sits below the
  // list: bring it into view after a pick, or the tap seems to do nothing.
  const listRef = useRef<HTMLElement>(null);
  const asideRef = useRef<HTMLElement>(null);
  const pick = (id: string | null) => {
    select(id);
    if (!id) return;
    requestAnimationFrame(() => {
      const list = listRef.current;
      const aside = asideRef.current;
      if (!list || !aside || aside.offsetTop <= list.offsetTop) return;
      const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
      aside.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'start' });
    });
  };

  return (
    <PageFrame wide>
      <FeatureGate feature="vendor_risk">
        <PageHeader
          className="!mb-[18px]"
          title={t('vendors.title')}
          subtitle={
            register.data
              ? [
                  t('vendors.board.summary', { count: total }),
                  t('vendors.board.summaryCritical', { count: critical.data?.total ?? 0 }),
                ].join(' · ')
              : undefined
          }
          actions={
            <>
              <Button
                variant="secondary"
                icon={ClipboardList}
                onClick={() => navigate('/vendors/questionnaires')}
              >
                {t('vendors.questionnaires')}
              </Button>
              {canCreateAsset && (
                <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                  {t('vendors.board.add')}
                </Button>
              )}
            </>
          }
        />

        <div className="flex flex-wrap gap-4 items-start">
          <section ref={listRef} className="flex-[1.6_1_520px] min-w-0">
            <label className="relative flex items-center mb-3">
              <Search
                size={15}
                className="absolute left-2.5 text-ink-muted pointer-events-none"
                aria-hidden="true"
              />
              <input
                type="search"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                placeholder={t('vendors.board.search')}
                aria-label={t('vendors.board.search')}
                className="w-full h-9 pl-8 pr-2 rounded-[10px] text-[13px] text-ink bg-surface-1 border border-border-default"
                data-testid="vendor-search"
              />
            </label>
            <div className="bg-surface-1 border border-border-subtle rounded-[14px] overflow-hidden">
              {register.isLoading ? (
                <div className="p-4">
                  <BlockSkeleton lines={6} height={44} />
                </div>
              ) : register.isError ? (
                <div className="p-4">
                  <BlockError onRetry={() => void register.refetch()} />
                </div>
              ) : rows.length === 0 ? (
                <div className="p-6 text-center">
                  {search ? (
                    <BlockEmpty>{t('vendors.board.noMatch')}</BlockEmpty>
                  ) : (
                    <>
                      <h2 className="m-0 text-[15px] font-semibold text-ink mb-1.5">
                        {t('vendors.emptyTitle')}
                      </h2>
                      <p className="m-0 text-[13px] text-ink-soft mb-4 max-w-xl mx-auto">
                        {canCreateAsset ? t('vendors.emptyBody') : t('vendors.emptyReadOnly')}
                      </p>
                      {canCreateAsset && (
                        <Button variant="primary" icon={Plus} onClick={() => setCreating(true)}>
                          {t('vendors.board.add')}
                        </Button>
                      )}
                    </>
                  )}
                </div>
              ) : (
                <ul className="m-0 p-0 list-none" aria-label={t('vendors.board.listLabel')}>
                  {rows.map((v) => (
                    <VendorRow
                      key={v.id}
                      v={v}
                      on={v.id === selectedId}
                      onPick={() => pick(v.id === selectedId ? null : v.id)}
                    />
                  ))}
                </ul>
              )}
            </div>
            {rows.length < total && (
              <div className="flex justify-center mt-3">
                <Button
                  variant="secondary"
                  size="sm"
                  loading={register.isFetching}
                  onClick={() => setLimit((l) => l + PAGE)}
                  data-testid="vendor-more"
                >
                  {t('vendors.board.more')}
                </Button>
              </div>
            )}
          </section>

          <aside ref={asideRef} className="flex-[1_1_360px] min-w-[300px] lg:sticky lg:top-4">
            {selectedId ? (
              <VendorPanel id={selectedId} row={selected} onClose={() => select(null)} />
            ) : (
              <EmptyPanel rows={rows} loading={register.isLoading} />
            )}
          </aside>
        </div>

        <CreateAssetModal
          isOpen={creating}
          initialType="Supplier"
          onClose={() => {
            setCreating(false);
            void qc.invalidateQueries({ queryKey: VENDORS_KEY });
          }}
        />
      </FeatureGate>
    </PageFrame>
  );
}

/* ------------------------------------------------------------------ row */

function VendorRow({ v, on, onPick }: { v: VendorRegisterEntry; on: boolean; onPick: () => void }) {
  const { t, locale } = useI18n();
  const labels = useVendorLabels();
  const la = v.latest_assessment;
  const letter = ratingLetter(la?.tier);
  const standing = standingOf(la);
  const open = knownStatus(la?.status) === 'sent' || knownStatus(la?.status) === 'in_progress';
  return (
    <li className="relative border-t border-border-subtle first:border-t-0">
      {/* The selection keyline, the product's mark for "this one". */}
      {on && <span className="absolute inset-y-0 left-0 w-[2px] bg-accent" aria-hidden="true" />}
      <button
        type="button"
        onClick={onPick}
        aria-pressed={on}
        data-testid="vendor-row"
        className={`w-full grid grid-cols-[40px_1fr_auto] md:grid-cols-[40px_1fr_120px_170px_86px] gap-x-4 gap-y-1 items-center px-4 py-3 text-left transition-colors ${
          on ? 'bg-surface-2' : 'hover:bg-surface-2'
        }`}
      >
        <LetterTile letter={letter} tier={la?.tier} />
        <span className="min-w-0">
          <span className="block text-[14px] font-semibold text-ink truncate">{v.name}</span>
          <span className="block text-[12.5px] text-ink-muted truncate">
            {v.service_provided || v.legal_name || '—'}
          </span>
        </span>
        <span className="hidden md:block text-[12px] font-semibold text-ink-soft">
          {v.service_criticality
            ? t('vendors.board.service', {
                level: labels.serviceCriticality(v.service_criticality),
              })
            : t('vendors.board.noService')}
        </span>
        <span className="justify-self-end md:justify-self-start">
          <Badge intent={standingIntent(standing)}>{t(`vendors.board.standing.${standing}`)}</Badge>
        </span>
        <span
          className={`hidden md:block mono text-[12px] text-right ${
            standing === 'overdue' ? 'text-danger-text' : 'text-ink-soft'
          }`}
        >
          {open && la?.due_at
            ? formatDate(locale, la.due_at, { day: 'numeric', month: 'short' })
            : t('vendors.board.noDue')}
        </span>
      </button>
    </li>
  );
}

function LetterTile({
  letter,
  tier,
  size = 36,
}: {
  letter: string | null;
  tier?: string | null;
  size?: number;
}) {
  const { t } = useI18n();
  const labels = useVendorLabels();
  const color = letter ? LETTER_COLOR[letter] : 'var(--fg-muted)';
  const label = letter
    ? t('vendors.board.ratingTitle', { tier: labels.tier(tier ?? '') })
    : t('vendors.board.unrated');
  return (
    <span
      className="inline-flex items-center justify-center rounded-[9px] border border-border-default bg-surface-0 mono font-bold"
      style={{ width: size, height: size, color, fontSize: size * 0.5 }}
      title={label}
      aria-label={label}
      data-testid="vendor-letter"
    >
      {letter ?? '–'}
    </span>
  );
}

/* --------------------------------------------------------------- panels */

function EmptyPanel({ rows, loading }: { rows: VendorRegisterEntry[]; loading: boolean }) {
  const { t } = useI18n();
  const count = (s: VendorStanding) =>
    rows.filter((r) => standingOf(r.latest_assessment) === s).length;
  const overdue = count('overdue');
  const action = count('action');
  const never = count('never');
  return (
    <section
      className="bg-surface-1 border border-border-subtle rounded-[14px] px-5 py-[18px]"
      data-testid="vendor-panel-empty"
    >
      <h2 className="m-0 text-[15px] font-semibold text-ink">
        {t('vendors.board.panelEmptyTitle')}
      </h2>
      <p className="m-0 mt-1 text-[12.5px] text-ink-soft">{t('vendors.board.panelEmptyBody')}</p>
      {!loading && rows.length > 0 && (
        <>
          <div className="text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted mt-5 mb-2">
            {t('vendors.board.attention')}
          </div>
          {overdue + action + never === 0 ? (
            <p className="m-0 text-[12.5px] text-ink-muted">{t('vendors.board.attNone')}</p>
          ) : (
            <ul className="m-0 p-0 list-none grid gap-1.5 text-[13px] text-ink">
              {overdue > 0 && <li>{t('vendors.board.attOverdue', { count: overdue })}</li>}
              {action > 0 && <li>{t('vendors.board.attAction', { count: action })}</li>}
              {never > 0 && <li>{t('vendors.board.attNever', { count: never })}</li>}
            </ul>
          )}
        </>
      )}
    </section>
  );
}

function VendorPanel({
  id,
  row,
  onClose,
}: {
  id: string;
  row: VendorRegisterEntry | null;
  onClose: () => void;
}) {
  const { t, locale } = useI18n();
  const labels = useVendorLabels();
  const record = useVendorRecord(id);
  const assessments = useVendorAssessments(id);
  const canSend = useAuthStore((s) => s.hasPermission('vendors:manage'));
  const canCreateRisk = useAuthStore((s) => s.hasPermission('risks:create'));
  const [sending, setSending] = useState(false);
  const [riskOpen, setRiskOpen] = useState(false);

  // The latest SCORED assessment: a newer questionnaire still out for answers
  // has no score, and the profile must not go blank meanwhile.
  const scored = useMemo<VendorAssessment | undefined>(() => {
    const list = (assessments.data ?? []).filter(
      (a) => knownStatus(a.status) === 'submitted' && a.score !== null && a.score !== undefined,
    );
    return [...list].sort((a, b) => (b.submitted_at ?? '').localeCompare(a.submitted_at ?? ''))[0];
  }, [assessments.data]);
  const detail = useVendorAssessment(scored?.id);

  const attrs = (record.data?.attributes ?? {}) as Record<string, unknown>;
  const name = row?.name ?? record.data?.name ?? '';
  const la = row?.latest_assessment ?? null;
  const standing = standingOf(la);
  const letter = ratingLetter(scored?.tier ?? la?.tier);
  const score = scored?.score ?? null;
  const tier = knownTier(scored?.tier);
  const dataShared = typeof attrs.data_shared === 'string' ? attrs.data_shared : '';
  const contractEnd =
    row?.contract_end || (typeof attrs.contract_end === 'string' ? attrs.contract_end : '');
  const open = knownStatus(la?.status) === 'sent' || knownStatus(la?.status) === 'in_progress';

  const weakest = useMemo(() => {
    const d = detail.data;
    if (!d) return [];
    const text = new Map((d.items ?? []).map((i) => [i.id, i.text]));
    return (d.score_breakdown ?? [])
      .filter((c) => (c.contribution ?? 0) > 0)
      .sort((a, b) => (b.contribution ?? 0) - (a.contribution ?? 0))
      .slice(0, 5)
      .map((c) => ({ id: c.item_id, text: text.get(c.item_id) ?? '', pts: c.contribution ?? 0 }));
  }, [detail.data]);

  const dt = 'text-ink-muted';
  const dd = 'm-0 font-semibold text-ink';
  const date = (v?: string | null) => (v ? formatDate(locale, v) : '—');

  if (!row && record.isLoading) {
    return (
      <section className="bg-surface-1 border border-border-subtle rounded-[14px] p-5">
        <BlockSkeleton lines={6} height={22} />
      </section>
    );
  }
  if (!row && record.isError) {
    return (
      <section className="bg-surface-1 border border-border-subtle rounded-[14px] p-5">
        <BlockError onRetry={() => void record.refetch()} />
      </section>
    );
  }

  return (
    <section
      className="bg-surface-1 border border-border-subtle rounded-[14px] px-5 py-5"
      data-testid="vendor-panel"
    >
      <div className="flex items-start gap-3.5">
        <LetterTile letter={letter} tier={scored?.tier ?? la?.tier} size={56} />
        <div className="flex-1 min-w-0">
          <h2 className="m-0 text-[19px] font-semibold text-ink leading-tight">{name}</h2>
          <div className="text-[12.5px] text-ink-soft mt-0.5">
            {[
              row?.service_provided,
              row?.service_criticality
                ? t('vendors.board.service', {
                    level: labels.serviceCriticality(row.service_criticality),
                  })
                : '',
            ]
              .filter(Boolean)
              .join(' · ')}
          </div>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label={t('vendors.board.close')}
          className="text-ink-muted hover:text-ink p-1"
        >
          <X size={16} />
        </button>
      </div>

      <div className="mt-5" title={t('vendors.board.riskScoreHint')}>
        <div className="flex items-center justify-between text-[11px] tracking-[.06em] uppercase font-semibold text-ink-muted mb-1.5">
          <span>{t('vendors.board.riskScore')}</span>
          {score !== null && tier && <Badge intent={tierIntent(tier)}>{labels.tier(tier)}</Badge>}
        </div>
        <div className="flex items-center gap-3">
          <span className="flex-1 h-1.5 rounded-full bg-[var(--chart-track)] overflow-hidden">
            <span
              className="block h-full rounded-full"
              style={{
                width: `${score ?? 0}%`,
                background: letter ? LETTER_COLOR[letter] : 'var(--fg-muted)',
              }}
            />
          </span>
          <span className="mono text-[13px] font-semibold text-ink" data-testid="vendor-score">
            {score !== null
              ? `${formatNumber(locale, score, { maximumFractionDigits: 0 })}/100`
              : '—'}
          </span>
        </div>
      </div>

      <dl className="grid grid-cols-[150px_1fr] gap-x-4 gap-y-2.5 text-[13px] m-0 mt-5">
        <dt className={dt}>{t('vendors.board.dataShared')}</dt>
        <dd className={dd}>
          {dataShared ? t(`vendors.board.data.${dataShared}`, dataShared) : '—'}
        </dd>
        <dt className={dt}>{t('vendors.board.contractEnd')}</dt>
        <dd className={dd}>{date(contractEnd)}</dd>
        {/* A due date only means something while a questionnaire is open. */}
        {open && (
          <>
            <dt className={dt}>{t('vendors.board.questionnaireDue')}</dt>
            <dd className={standing === 'overdue' ? 'm-0 font-semibold text-danger-text' : dd}>
              {date(la?.due_at)}
            </dd>
          </>
        )}
        <dt className={dt}>{t('vendors.board.owner')}</dt>
        <dd className={dd}>{row?.owner || record.data?.owner || '—'}</dd>
        <dt className={dt}>{t('vendors.board.status')}</dt>
        <dd className={dd}>
          <Badge intent={standingIntent(standing)}>{t(`vendors.board.standing.${standing}`)}</Badge>
        </dd>
      </dl>

      <div className="text-[10.5px] tracking-[.06em] uppercase font-semibold text-ink-muted mt-6 mb-1">
        {t('vendors.board.weakest')}
      </div>
      {assessments.isLoading || (scored && detail.isLoading) ? (
        <BlockSkeleton lines={3} height={16} />
      ) : assessments.isError || detail.isError ? (
        <BlockError
          onRetry={() => void (assessments.isError ? assessments.refetch() : detail.refetch())}
        />
      ) : !scored ? (
        <p className="m-0 text-[12.5px] text-ink-muted" data-testid="vendor-weakest-empty">
          {la ? t('vendors.board.notScored') : t('vendors.board.neverAssessed')}
        </p>
      ) : weakest.length === 0 ? (
        <p className="m-0 text-[12.5px] text-ink-muted">{t('vendors.board.weakestNone')}</p>
      ) : (
        <>
          <p className="m-0 mb-2 text-[11.5px] text-ink-muted">{t('vendors.board.weakestHint')}</p>
          <ul className="m-0 p-0 list-none grid gap-2" data-testid="vendor-weakest">
            {weakest.map((w) => (
              <li
                key={w.id}
                className="grid grid-cols-[1fr_auto] gap-3 items-baseline text-[12.5px]"
              >
                <span className="text-ink-soft">{w.text}</span>
                <span className="mono text-ink font-semibold">
                  {t('vendors.board.points', {
                    pts: formatNumber(locale, w.pts, { maximumFractionDigits: 1 }),
                  })}
                </span>
              </li>
            ))}
          </ul>
          <Link
            to={`/vendors/${id}/assessments/${scored.id}`}
            className="inline-block mt-2 text-[12px] font-semibold text-accent hover:underline"
          >
            {t('vendors.board.openAssessment')}
          </Link>
        </>
      )}

      <div className="flex flex-wrap gap-2 mt-6">
        {canSend && (
          <Button
            variant="primary"
            className="flex-1"
            onClick={() => setSending(true)}
            data-testid="vendor-send"
          >
            {t('vendors.board.send')}
          </Button>
        )}
        {canCreateRisk && (
          <Button variant="secondary" onClick={() => setRiskOpen(true)} data-testid="vendor-risk">
            {t('vendors.board.createRisk')}
          </Button>
        )}
      </div>
      <Link
        to={`/vendors/${id}`}
        className="block mt-3 text-center text-[12.5px] text-ink-soft hover:text-ink hover:underline"
      >
        {t('vendors.board.openProfile')}
      </Link>

      {sending && (
        <SendAssessmentModal
          vendorId={id}
          vendorName={name}
          contactEmail={typeof attrs.contact_email === 'string' ? attrs.contact_email : ''}
          onClose={() => setSending(false)}
          onSent={() => setSending(false)}
        />
      )}
      {riskOpen && (
        <CreateRiskModal
          isOpen
          onClose={() => setRiskOpen(false)}
          initialAssetIds={[id]}
          initialTitle={t('vendors.board.riskTitle', { name })}
        />
      )}
    </section>
  );
}

export default VendorsPage;
