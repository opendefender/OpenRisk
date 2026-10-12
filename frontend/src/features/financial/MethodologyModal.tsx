// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// "Méthodologie" (spec §4): how a quantified figure was produced — the model,
// its inputs and where each came from, the assumptions, run parameters and the
// rate used. Opened from the financial page's subtitle and from the simulator.

import { BookOpen } from 'lucide-react';

import { useI18n } from '../../hooks/useI18n';
import { formatDate, formatDateTime, formatNumber } from '../../i18n/format';
import { Modal } from '../../shared/ds';
import type { Methodology } from './financialService';

export function MethodologyModal({ m, onClose }: { m: Methodology; onClose: () => void }) {
  const { t, locale } = useI18n();
  const rows: { k: string; v: string }[] = [
    { k: t('financial.methodology.model'), v: m.model },
    { k: t('financial.methodology.formula'), v: m.formula_version },
    { k: t('financial.methodology.iterations'), v: formatNumber(locale, m.iterations) },
    { k: t('financial.methodology.seed'), v: String(m.seed) },
    {
      k: t('financial.methodology.computedAt'),
      v: m.computed_at ? formatDateTime(locale, m.computed_at) : '—',
    },
    {
      k: t('financial.methodology.currency'),
      v: `${m.currency} · 1 ${m.currency} = ${formatNumber(locale, m.fx_rate_xaf)} FCFA (${
        m.fx_as_of ? formatDate(locale, m.fx_as_of) : '—'
      })`,
    },
  ];
  return (
    <Modal
      open
      onClose={onClose}
      title={t('financial.methodology.title')}
      leading={<BookOpen size={16} className="text-accent" aria-hidden="true" />}
      closeLabel={t('common.close')}
    >
      <div className="text-[12.5px]">
        <div className="grid gap-1.5 mb-4">
          {rows.map((r) => (
            <div key={r.k} className="flex items-start justify-between gap-4">
              <span className="text-ink-muted">{r.k}</span>
              <span className="text-ink text-right mono">{r.v}</span>
            </div>
          ))}
        </div>

        <div className="text-[11px] uppercase tracking-[.05em] text-ink-muted mb-2">
          {t('financial.methodology.inputs')}
        </div>
        <table className="w-full mb-4">
          <tbody>
            {m.inputs.map((inp) => (
              <tr key={inp.key} className="border-t border-border-subtle">
                <td className="py-1.5 pr-2 text-ink-soft">{inp.label}</td>
                <td className="py-1.5 text-right mono text-ink">
                  {formatNumber(locale, inp.value)}{' '}
                  <span className="text-ink-muted">{inp.unit}</span>
                </td>
                <td className="py-1.5 pl-2 text-right">
                  <SourceChip source={inp.source} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        <div className="text-[11px] uppercase tracking-[.05em] text-ink-muted mb-2">
          {t('financial.methodology.assumptions')}
        </div>
        <ul className="list-disc pl-5 grid gap-1 text-ink-soft mb-3">
          {m.assumptions.map((a) => (
            <li key={a}>{a}</li>
          ))}
        </ul>

        <a
          href={m.doc_url}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 text-[12px] text-accent hover:underline"
        >
          <BookOpen size={13} aria-hidden="true" /> {t('financial.methodology.doc')}
        </a>
      </div>
    </Modal>
  );
}

function SourceChip({ source }: { source: string }) {
  const { t } = useI18n();
  const map: Record<string, { label: string; tone: string }> = {
    'risk-input': { label: t('financial.methodology.src.input'), tone: 'var(--success-text)' },
    derived: { label: t('financial.methodology.src.derived'), tone: 'var(--accent)' },
    'reference-model': {
      label: t('financial.methodology.src.reference'),
      tone: 'var(--warning-text)',
    },
  };
  const s = map[source] ?? { label: source, tone: 'var(--fg-muted)' };
  return (
    <span
      className="text-[10.5px] px-1.5 py-0.5 rounded"
      style={{ background: `color-mix(in srgb, ${s.tone} 14%, transparent)`, color: s.tone }}
    >
      {s.label}
    </span>
  );
}
