// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// CSV of register rows (#913): the header's "Exporter" (the current view) and
// the bulk bar's (the selection) both land here, and so does the drawer's single
// risk. Scores, P and I are written with the SAME formatters as the screen, so
// the file an auditor opens says what the screen said. The formula-injection
// guard is the shared table exporter's.

import { buildCsv, downloadCsv, type Column } from '../../shared/datatable';
import type { LocaleCode } from '../../i18n/locales';
import { millions } from '../financial/money';
import { formatImpact, formatProbability, formatScore } from './riskFormat';
import type { UiRisk } from './riskMap';
import type { RegisterMoney } from './useRiskRegisterData';

export interface RiskCsvContext {
  locale: LocaleCode;
  money: RegisterMoney;
  /** Translated labels, so the file follows the interface language. */
  label: (key: string, params?: Record<string, string | number>) => string;
}

function csvColumn(
  key: string,
  header: string,
  value: (r: UiRisk) => string | number,
): Column<UiRisk> {
  return { key, header, render: () => null, exportValue: value };
}

export function riskCsvColumns({ locale, money, label }: RiskCsvContext): Column<UiRisk>[] {
  const cols: Column<UiRisk>[] = [
    csvColumn('id', label('riskRegister.col.id'), (r) => r.id),
    csvColumn('title', label('riskRegister.col.risk'), (r) => r.name),
    csvColumn('category', label('riskRegister.col.category'), (r) => r.categoryName),
    csvColumn('probability', label('riskRegister.drawer.probability'), (r) =>
      formatProbability(locale, r.prob),
    ),
    csvColumn('impact', label('riskRegister.drawer.impact'), (r) => formatImpact(locale, r.impact)),
    // The inherent score, said so in the header: the residual has its own column.
    csvColumn('score', label('riskRegister.drawer.inherentScore'), (r) =>
      formatScore(locale, r.score),
    ),
    csvColumn('residual', label('riskRegister.drawer.residualScore'), (r) =>
      formatScore(locale, r.residual),
    ),
    csvColumn('band', label('riskRegister.col.band'), (r) =>
      r.bandKnown ? label(`risks.band.${r.crit}`) : '',
    ),
    csvColumn('status', label('riskRegister.col.status'), (r) =>
      label(`riskRegister.status.${r.statusValue}`),
    ),
    csvColumn('owner', label('riskRegister.col.owner'), (r) =>
      r.ownerName === '—' ? '' : r.ownerName,
    ),
  ];
  if (money.entitled) {
    cols.push(
      csvColumn('ale', label('riskRegister.col.aleUnit', { currency: money.currency }), (r) =>
        r.ale ? millions(money.toDisplay(r.ale), locale) : '',
      ),
    );
  }
  cols.push(
    csvColumn('trend', label('riskRegister.col.trend'), (r) =>
      r.trend ? label(`riskRegister.trend.${r.trend}`) : '',
    ),
    csvColumn('review', label('riskRegister.col.review'), (r) =>
      r.nextReviewAt ? r.nextReviewAt.slice(0, 10) : '',
    ),
    csvColumn('assets', label('riskRegister.drawer.assets'), (r) =>
      r.assetRefs.map((a) => a.name).join(' | '),
    ),
  );
  return cols;
}

/** Downloads the rows as CSV. Returns how many rows were written. */
export function exportRisksCsv(
  rows: readonly UiRisk[],
  ctx: RiskCsvContext,
  filename: string,
): number {
  downloadCsv(
    `${filename}-${new Date().toISOString().slice(0, 10)}.csv`,
    buildCsv([...rows], riskCsvColumns(ctx)),
  );
  return rows.length;
}
