// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The register's columns, flat as in the V2 mockup (#913): risk (title, then
// "#id  asset" in mono), score (P and I in its tooltip and in the drawer),
// criticality as a square mark and coloured text, status as plain text, owner,
// next review. ALE, trend, last update, phase, control reference and tags stay
// one click away in the column chooser, hidden by default.
//
// Sorting is offered only where the server can do it: title, score, band,
// status and review date. The id (a UUID prefix) and the ALE (computed after
// the page is read, not a column) are not sortable, and the ALE header says so.

import { useMemo, type ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { AlertTriangle, Clock } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { formatDate } from '../../i18n/format';
import { critColor } from '../../shared/riskColors';
import type { Column } from '../../shared/datatable';
import { FwBadge } from '../../shared/ui';
import type { Assignee } from '../../services/ownershipService';
import { millions } from '../financial/money';
import { memberName } from './riskMembers';
import { PhasePill } from './RiskDrawerSections';
import { formatPI, formatScore } from './riskFormat';
import type { UiRisk } from './riskMap';
import { reviewState } from './riskViews';
import type { RegisterMoney } from './useRiskRegisterData';
import { useUIStore } from '../../store/uiStore';
import { TrendCell } from './TrendCell';

const muted = <span className="text-ink-muted text-[12px]">—</span>;

export function useRiskColumns({
  members,
  money,
  now,
}: {
  members: readonly Assignee[];
  money: RegisterMoney;
  now: Date;
}): Column<UiRisk>[] {
  const { t, locale } = useI18n();
  const lang = useUIStore((s) => s.lang);
  const navigate = useNavigate();

  const names = useMemo(() => new Map(members.map((m) => [m.user_id, memberName(m)])), [members]);
  const ownerOf = (r: UiRisk): string =>
    (r.ownerId ? names.get(r.ownerId) : undefined) ?? (r.ownerName === '—' ? '' : r.ownerName);

  return useMemo<Column<UiRisk>[]>(() => {
    const cols: Column<UiRisk>[] = [
      {
        key: 'title',
        header: t('riskRegister.col.risk'),
        sortKey: 'title',
        hideable: false,
        render: (r) => {
          const assets = r.assetRefs.slice(0, 2).map((a) => a.name);
          const more = r.assetRefs.length - assets.length;
          const assetLine = assets.length
            ? `${assets.join(', ')}${more > 0 ? ` +${more}` : ''}`
            : '';
          return (
            <div className="min-w-0 max-w-[360px]">
              <div className="text-[14px] font-medium text-ink truncate" title={r.name}>
                {r.name}
              </div>
              <div className="mono text-[12px] text-ink-muted truncate flex gap-2.5">
                <span className="shrink-0">#{r.id.slice(0, 8)}</span>
                {assetLine && <span className="truncate">{assetLine}</span>}
              </div>
            </div>
          );
        },
      },
      {
        key: 'score',
        header: t('riskRegister.col.score'),
        sortKey: 'score',
        align: 'right',
        width: 88,
        // The number is neutral ink: the band's colour lives in the mark beside
        // it, so a score is never coloured by a threshold applied here. P and I
        // are in the tooltip and in the drawer.
        render: (r) => {
          const pi = formatPI(locale, r.prob, r.impact);
          return (
            <span className="mono text-[14px] text-ink whitespace-nowrap" title={pi}>
              {formatScore(locale, r.score)}
              <span className="sr-only"> ({pi})</span>
            </span>
          );
        },
      },
      {
        key: 'band',
        header: t('riskRegister.col.band'),
        // No sort: the band's names do not sort in severity order (the score does).
        width: 112,
        render: (r) =>
          r.bandKnown ? (
            <span
              className="inline-flex items-center gap-2 text-[13px] whitespace-nowrap"
              style={{ color: critColor[r.crit] }}
            >
              <span aria-hidden="true" className="size-1.5 rounded-[1px] bg-current" />
              {t(`risks.band.${r.crit}`)}
            </span>
          ) : (
            muted
          ),
      },
      {
        key: 'status',
        header: t('riskRegister.col.status'),
        sortKey: 'status',
        width: 112,
        // Plain text, as in the mockup: an open risk reads strongest, the rest
        // recede. No pill.
        render: (r) => (
          <span
            className={`text-[13px] whitespace-nowrap ${
              r.statusValue === 'open' ? 'text-ink font-medium' : 'text-ink-soft'
            }`}
          >
            {t(`riskRegister.status.${r.statusValue}`)}
          </span>
        ),
      },
      {
        key: 'owner',
        header: t('riskRegister.col.owner'),
        width: 160,
        render: (r) => {
          const name = ownerOf(r);
          return name ? (
            <span className="block text-[13px] text-ink-soft truncate max-w-[150px]" title={name}>
              {name}
            </span>
          ) : (
            <span className="text-[13px] text-ink-muted">{t('riskRegister.owner.none')}</span>
          );
        },
      },
      ...(money.entitled
        ? [
            {
              key: 'ale',
              header: (
                <span title={t('riskRegister.col.aleUnsortable')}>{t('riskRegister.col.ale')}</span>
              ),
              headerLabel: t('riskRegister.col.aleUnit', { currency: money.currency }),
              align: 'right' as const,
              width: 72,
              defaultHidden: true,
              render: (r: UiRisk): ReactNode =>
                r.ale ? (
                  <span className="mono text-[13px] font-semibold text-ink whitespace-nowrap">
                    {millions(money.toDisplay(r.ale), locale)}
                  </span>
                ) : (
                  muted
                ),
            },
          ]
        : []),
      {
        key: 'trend',
        header: t('riskRegister.col.trend'),
        align: 'right',
        width: 64,
        defaultHidden: true,
        render: (r) => <TrendCell trend={r.trend} />,
      },
      {
        key: 'review',
        header: t('riskRegister.col.review'),
        sortKey: 'next_review_at',
        width: 92,
        render: (r) => {
          const s = reviewState(r.nextReviewAt, r.statusValue, now);
          switch (s.kind) {
            case 'closed':
              return (
                <span className="text-[12.5px] text-ink-soft">
                  {t('riskRegister.review.closed')}
                </span>
              );
            case 'none':
              return muted;
            case 'overdue':
            case 'soon':
            case 'later':
            default: {
              const date = r.nextReviewAt
                ? formatDate(locale, r.nextReviewAt, { day: 'numeric', month: 'short' })
                : '—';
              const tone =
                s.kind === 'overdue'
                  ? 'text-danger-text font-semibold'
                  : s.kind === 'soon'
                    ? 'text-warning-text font-semibold'
                    : 'text-ink-soft';
              return (
                <span
                  className={`inline-flex items-center gap-1 text-[12.5px] whitespace-nowrap ${tone}`}
                >
                  {s.kind === 'overdue' && <AlertTriangle size={12} aria-hidden="true" />}
                  {s.kind === 'soon' && <Clock size={12} aria-hidden="true" />}
                  {date}
                  {s.kind === 'overdue' && (
                    <span className="sr-only">{t('riskRegister.review.overdue')}</span>
                  )}
                  {s.kind === 'soon' && (
                    <span className="sr-only">{t('riskRegister.review.soon')}</span>
                  )}
                </span>
              );
            }
          }
        },
      },
      // --- Kept from the previous register, hidden until asked for.
      {
        key: 'phase',
        header: t('riskRegister.col.phase'),
        defaultHidden: true,
        render: (r) => <PhasePill phase={r.phase} lang={lang} />,
      },
      {
        key: 'framework',
        header: t('riskRegister.col.framework'),
        defaultHidden: true,
        render: (r) =>
          r.fw !== '—' ? (
            <a
              href={r.fwHref}
              onClick={(e) => {
                e.stopPropagation();
                e.preventDefault();
                navigate(r.fwHref);
              }}
              className="inline-flex items-center gap-1"
            >
              <FwBadge fw={r.fw} />
              {r.mappings.length > 1 ? (
                <span className="text-[11px] text-ink-muted">+{r.mappings.length - 1}</span>
              ) : null}
            </a>
          ) : (
            muted
          ),
      },
      {
        key: 'tags',
        header: t('riskRegister.col.tags'),
        defaultHidden: true,
        render: (r) =>
          r.tags.length ? (
            <span className="flex flex-wrap gap-1">
              {r.tags.slice(0, 2).map((tag) => (
                <span
                  key={tag}
                  className="rounded-full border border-border-default px-1.5 py-0.5 text-[11px] text-ink-soft"
                >
                  {tag}
                </span>
              ))}
              {r.tags.length > 2 ? (
                <span className="text-[11px] text-ink-muted">+{r.tags.length - 2}</span>
              ) : null}
            </span>
          ) : (
            muted
          ),
      },
      {
        key: 'updated',
        header: t('riskRegister.col.updated'),
        sortKey: 'updated_at',
        defaultHidden: true,
        render: (r) => <span className="text-[12px] text-ink-soft whitespace-nowrap">{r.mod}</span>,
      },
    ];
    return cols;
    // `ownerOf` closes over `names`; both change together.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [t, locale, lang, money, names, now, navigate]);
}
