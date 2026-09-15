// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Review one questionnaire a vendor was sent (#673, ADR 0004 D5).
//
// Everything numeric on this screen is the backend's: the score, the tier and
// each question's contribution come from the assessment as submitted
// (vendorscore/1). Nothing is added up, rounded into a tier or recomputed here;
// a client-side copy of the formula would be a second source of truth that
// could quietly disagree with the one the audit trail records.

import { Link, useParams } from 'react-router';
import { ArrowLeft } from 'lucide-react';

import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { FeatureGate } from '../../shared/FeatureGate';
import { Badge } from '../../shared/ds';
import { useRegisterCrumb } from '../../shared/crumbLabels';
import { useI18n } from '../../hooks/useI18n';
import { localeTag } from '../../i18n';
import { useQuestionnaireTemplates } from './useQuestionnaireTemplates';
import { useVendorAssessment, useVendorChain } from './useVendors';
import { useVendorLabels } from './useVendorLabels';
import {
  formatNumber,
  httpStatus,
  knownStatus,
  statusIntent,
  tierIntent,
} from './vendorDisplay';
import type { VendorAssessmentItem } from './vendorService';

export function VendorAssessmentPage() {
  const { vendorId = '', assessmentId = '' } = useParams();
  const { t, locale } = useI18n();
  const labels = useVendorLabels();
  const query = useVendorAssessment(assessmentId);
  const chain = useVendorChain(vendorId);
  const templates = useQuestionnaireTemplates(true);
  useRegisterCrumb(`/vendors/${vendorId}`, chain.data?.vendor_name);

  const tag = localeTag(locale);
  const date = (iso: string | null | undefined) =>
    iso ? new Date(iso).toLocaleDateString(tag) : '';
  const num = (n: number) => formatNumber(n, tag, 2);

  const a = query.data;
  // An assessment reached under another vendor's URL is not this vendor's.
  const foreign = Boolean(a && a.vendor_asset_id !== vendorId);
  const notFound = httpStatus(query.error) === 404 || foreign;

  const templateName = a ? templates.data?.find((x) => x.id === a.template_id)?.name : undefined;
  const title = a
    ? templateName
      ? `${templateName} · ${t('vendors.assessments.version', { version: a.template_version })}`
      : t('vendors.assessments.unnamed', { version: a.template_version })
    : t('vendors.review.title');

  const answer = (item: VendorAssessmentItem): string => {
    if (item.answer_na) return t('vendors.review.na');
    const v = item.answer_value;
    if (v === null || v === undefined || v === '') return t('vendors.review.noAnswer');
    if (item.answer_type === 'choice') {
      return item.options?.find((o) => o.value === v)?.label ?? v;
    }
    return v;
  };

  const items = a ? [...(a.items ?? [])].sort((x, y) => x.position - y.position) : [];
  const breakdown = new Map((a?.score_breakdown ?? []).map((c) => [c.item_id, c]));

  return (
    <PageFrame>
      <FeatureGate feature="vendor_risk">
        <Link
          to={`/vendors/${vendorId}`}
          className="inline-flex items-center gap-1.5 text-[13px] text-ink-soft hover:text-ink mb-3"
        >
          <ArrowLeft size={15} aria-hidden="true" />
          {t('vendors.review.back')}
        </Link>

        {query.isLoading && (
          <div
            role="status"
            aria-busy="true"
            aria-label={t('vendors.review.loading')}
            className="flex flex-col gap-3"
          >
            <div className="h-10 w-2/3 rounded-lg or-skeleton" />
            <div className="h-36 rounded-xl or-skeleton" />
            <div className="h-24 rounded-xl or-skeleton" />
          </div>
        )}

        {(query.isError || foreign) && (
          <Card className="p-6 text-center">
            <p role="alert" className="text-[14px] text-ink mb-4">
              {notFound ? t('vendors.review.notFound') : t('vendors.review.loadFailed')}
            </p>
            {!notFound && (
              <Btn label={t('vendors.retry')} onClick={() => void query.refetch()} />
            )}
          </Card>
        )}

        {a && !foreign && (
          <>
            <PageHeader
              title={title}
              badge={
                <Badge intent={statusIntent(a.status)} dot>
                  {labels.status(a.status)}
                </Badge>
              }
            />

            <Card className="p-5">
              <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
                <div>
                  <p className="text-[12.5px] font-semibold text-ink-soft">
                    {t('vendors.review.score')}
                  </p>
                  {a.score !== null && a.score !== undefined ? (
                    <p className="flex items-baseline gap-2">
                      <span className="text-[32px] font-bold tabular-nums text-ink">
                        {num(a.score)}
                      </span>
                      <span className="text-[13px] text-ink-muted">{t('vendors.review.outOf')}</span>
                    </p>
                  ) : (
                    <p className="text-[14px] text-ink mt-1">
                      {knownStatus(a.status) === 'submitted'
                        ? t('vendors.review.notScorable')
                        : t('vendors.review.notSubmitted')}
                    </p>
                  )}
                </div>
                {a.score !== null && a.score !== undefined && a.tier && (
                  <Badge size="md" intent={tierIntent(a.tier)}>
                    {labels.tier(a.tier)}
                  </Badge>
                )}
              </div>

              <div className="mt-3 flex flex-col gap-1 text-[12.5px] text-ink-muted">
                {a.score !== null && a.score !== undefined && (
                  <p>{t('vendors.review.higherRiskier')}</p>
                )}
                {a.scoring_version && (
                  <p>{t('vendors.review.scoringVersion', { version: a.scoring_version })}</p>
                )}
                <p>{t('vendors.review.selfAttested')}</p>
              </div>

              <dl className="mt-4 grid gap-x-6 gap-y-2 text-[13px] sm:grid-cols-2">
                <Fact term={t('vendors.review.contact')} value={a.contact_email} />
                <Fact term={t('vendors.review.sent')} value={date(a.sent_at)} />
                <Fact term={t('vendors.review.due')} value={date(a.due_at)} />
                {a.submitted_at && (
                  <Fact term={t('vendors.review.submitted')} value={date(a.submitted_at)} />
                )}
              </dl>
            </Card>

            <h2 className="text-[15px] font-bold text-ink mt-6 mb-1">
              {t('vendors.review.answers')} ({items.length})
            </h2>
            {a.score_breakdown && a.score_breakdown.length > 0 && (
              <p className="text-[12.5px] text-ink-muted mb-3">{t('vendors.review.formula')}</p>
            )}

            <ol className="list-none p-0 m-0 flex flex-col gap-3">
              {items.map((item, index) => {
                const c = breakdown.get(item.id);
                return (
                  <li key={item.id}>
                    <Card className="p-4">
                      <p className="text-[12px] text-ink-muted">
                        {t('vendors.review.questionLabel', { position: index + 1 })}
                        {item.control_ref
                          ? ` · ${t('vendors.review.controlRef', { ref: item.control_ref })}`
                          : ''}
                      </p>
                      <p className="text-[14px] font-semibold text-ink break-words">{item.text}</p>
                      <p className="mt-2 text-[13.5px] text-ink break-words">
                        {t('vendors.review.answerLine', { answer: answer(item) })}
                      </p>
                      {item.answer_comment && (
                        <p className="mt-1 text-[13px] text-ink-soft break-words">
                          {t('vendors.review.commentLine', { comment: item.answer_comment })}
                        </p>
                      )}
                      {item.answer_type === 'text' ? (
                        <p className="mt-2 text-[12.5px] text-ink-muted">
                          {t('vendors.review.notScored')}
                        </p>
                      ) : c ? (
                        c.na ? (
                          <p className="mt-2 text-[12.5px] text-ink-muted">
                            {t('vendors.review.excluded')}
                          </p>
                        ) : (
                          <dl className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-[12.5px]">
                            <InlineFact term={t('vendors.review.weight')} value={num(c.weight)} />
                            <InlineFact term={t('vendors.review.points')} value={num(c.points)} />
                            <InlineFact
                              term={t('vendors.review.contribution')}
                              value={num(c.contribution)}
                            />
                          </dl>
                        )
                      ) : null}
                    </Card>
                  </li>
                );
              })}
            </ol>
          </>
        )}
      </FeatureGate>
    </PageFrame>
  );
}

function Fact({ term, value }: { term: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-ink-muted">{term}</dt>
      <dd className="m-0 text-ink break-words">{value}</dd>
    </div>
  );
}

function InlineFact({ term, value }: { term: string; value: string }) {
  return (
    <div className="flex gap-1.5">
      <dt className="text-ink-muted">{term}</dt>
      <dd className="m-0 tabular-nums text-ink">{value}</dd>
    </div>
  );
}

export default VendorAssessmentPage;
