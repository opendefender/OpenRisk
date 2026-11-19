// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The register's columns on the redesign (#913): id, risk (title, category,
// first two assets), score with P and I, band, status, owner, ALE, trend and
// next review. What the design drops but the product keeps — lifecycle phase,
// control reference, free tags, last update — stays one click away in the
// column chooser, hidden by default.
//
// Sorting is offered only where the server can do it: title, score, band,
// status and review date. The id (a UUID prefix) and the ALE (computed after
// the page is read, not a column) are not sortable, and the ALE header says so.

import { useMemo, type ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { AlertTriangle, Clock } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { formatDate } from '../../i18n/format';
import { Badge, riskStatusIntent, severityIntent } from '../../shared/ds';
import type { Column } from '../../shared/datatable';
import { FwBadge } from '../../shared/ui';
import { bandTextColor } from '../../services/scoreService';
import type { Assignee } from '../../services/ownershipService';
import { millions } from '../financial/money';
import { memberName } from './riskMembers';
import { PhasePill } from './RiskDrawerSections';
import { formatPI, formatScore } from './riskFormat';
import { initialsOf, type UiRisk } from './riskMap';
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
        key: 'id',
        header: t('riskRegister.col.id'),
        width: 96,
        render: (r) => (
          <span className="mono text-[12px] text-ink-muted whitespace-nowrap">
            {r.id.slice(0, 8)}
          </span>
        ),
      },
      {
        key: 'title',
        header: t('riskRegister.col.risk'),
        sortKey: 'title',
        hideable: false,
        render: (r) => {
          const assets = r.assetRefs.slice(0, 2).map((a) => a.name);
          const more = r.assetRefs.length - assets.length;
          const sub = [
            r.categoryName,
            assets.length ? `${assets.join(', ')}${more > 0 ? ` +${more}` : ''}` : '',
          ]
            .filter(Boolean)
            .join(' · ');
          return (
            <div className="min-w-0 max-w-[360px]">
              <div className="text-[13px] font-medium text-ink truncate">{r.name}</div>
              {sub && <div className="text-[11.5px] text-ink-muted truncate">{sub}</div>}
            </div>
          );
        },
      },
      {
        key: 'score',
        header: t('riskRegister.col.score'),
        sortKey: 'score',
        align: 'right',
        width: 150,
        render: (r) => (
          <span className="whitespace-nowrap">
            {/* Coloured by the SERVER's band, never by a threshold on the number. */}
            <span
              className="mono text-[14px] font-semibold"
              style={{ color: bandTextColor(r.crit) }}
            >
              {formatScore(locale, r.score)}
            </span>
            <span className="mono text-[11px] text-ink-muted ml-1.5">
              {formatPI(locale, r.prob, r.impact)}
            </span>
          </span>
        ),
      },
      {
        key: 'band',
        header: t('riskRegister.col.band'),
        sortKey: 'criticality',
        width: 110,
        render: (r) => (
          <Badge intent={severityIntent[r.crit] ?? 'neutral'} dot>
            {t(`risks.band.${r.crit}`)}
          </Badge>
        ),
      },
      {
        key: 'status',
        header: t('riskRegister.col.status'),
        sortKey: 'status',
        width: 130,
        render: (r) => (
          <Badge intent={riskStatusIntent[r.statusValue] ?? 'neutral'}>
            {t(`riskRegister.status.${r.statusValue}`)}
          </Badge>
        ),
      },
      {
        key: 'owner',
        header: t('riskRegister.col.owner'),
        width: 190,
        render: (r) => {
          const name = ownerOf(r);
          if (!name) return muted;
          return (
            <span className="inline-flex items-center gap-2 min-w-0 max-w-full">
              <span
                aria-hidden="true"
                className="size-6 rounded-full bg-surface-3 inline-flex items-center justify-center text-[10px] font-semibold text-ink-soft shrink-0"
              >
                {initialsOf(name)}
              </span>
              <span className="text-[13px] text-ink-soft truncate">{name}</span>
            </span>
          );
        },
      },
      ...(money.entitled
        ? [
            {
              key: 'ale',
              header: (
                <span title={t('riskRegister.col.aleUnsortable')}>
                  {t('riskRegister.col.ale', { currency: money.currency })}
                </span>
              ),
              headerLabel: t('riskRegister.col.ale', { currency: money.currency }),
              align: 'right' as const,
              width: 110,
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
        width: 70,
        render: (r) => <TrendCell trend={r.trend} />,
      },
      {
        key: 'review',
        header: t('riskRegister.col.review'),
        sortKey: 'next_review_at',
        width: 130,
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
