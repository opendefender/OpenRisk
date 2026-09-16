// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// One vendor (#673, ADR 0004 D1–D4): its vendor→asset→risk chain, the
// questionnaires it was sent, and the actions on both.
//
// - Every asset and risk in the chain opens in the universal entity drawer via
//   a shareable URL, so a link from this page can be pasted into a ticket.
// - Adding and removing a link is optimistic and rolls back on refusal.
// - When a questionnaire's email did not go out, the server returns the link
//   once. It lives in this component's state only, is shown with a copy action,
//   and is gone when dismissed: it carries the vendor's one-time token.

import { useMemo, useState, type FormEvent } from 'react';
import { Link, useLocation, useParams } from 'react-router';
import { toast } from 'sonner';
import { ArrowLeft, Ban, Copy, Link2, PanelRight, RotateCw, Send, Unlink } from 'lucide-react';

import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { FeatureGate } from '../../shared/FeatureGate';
import { AlertDialog, Badge, Button, Field, Input, Select } from '../../shared/ds';
import { useRegisterCrumb } from '../../shared/crumbLabels';
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import { localeTag } from '../../i18n';
import { apiErrorMessage } from '../../lib/apiError';
import { drawerHref, useDrawerController } from '../entity-drawer';
import { CriticalityBadge } from '../assets/CriticalityBadge';
import { useAssets } from '../assets/useAssets';
import { useQuestionnaireTemplates } from './useQuestionnaireTemplates';
import {
  PENDING_LINK_PREFIX,
  useVendorAssessmentMutations,
  useVendorAssessments,
  useVendorChain,
  useVendorLinkMutations,
  useVendorRecord,
} from './useVendors';
import { useVendorLabels } from './useVendorLabels';
import { SendAssessmentModal } from './SendAssessmentModal';
import {
  formatNumber,
  httpStatus,
  isOpenAssessment,
  knownRiskLevel,
  statusIntent,
  tierIntent,
} from './vendorDisplay';
import { LINK_VERBS, validateLink, type FormErrors, type LinkDraft } from './vendorForms';
import type {
  VendorAssessment,
  VendorAssessmentDelivery,
  VendorChain,
  VendorChainLink,
} from './vendorService';

type DeliveryKind = 'send' | 'resend';

export function VendorDetailPage() {
  const { vendorId = '' } = useParams();
  const { t } = useI18n();
  const location = useLocation();
  const drawer = useDrawerController();
  const canManage = useAuthStore((s) => s.hasPermission('vendors:manage'));

  const chain = useVendorChain(vendorId);
  const record = useVendorRecord(vendorId);
  const [sendOpen, setSendOpen] = useState(false);
  const [delivery, setDelivery] = useState<{
    result: VendorAssessmentDelivery;
    kind: DeliveryKind;
  } | null>(null);

  useRegisterCrumb(`/vendors/${vendorId}`, chain.data?.vendor_name);

  const attributes = record.data?.attributes;
  const attr = (key: string): string => {
    const value = attributes?.[key];
    return typeof value === 'string' ? value.trim() : '';
  };
  const contactEmail = attr('contact_email');
  const here = `${location.pathname}${location.search}`;
  const notFound = httpStatus(chain.error) === 404;

  return (
    <PageFrame>
      <FeatureGate feature="vendor_risk">
        <Link
          to="/vendors"
          className="inline-flex items-center gap-1.5 text-[13px] text-ink-soft hover:text-ink mb-3"
        >
          <ArrowLeft size={15} aria-hidden="true" />
          {t('vendors.detail.back')}
        </Link>

        {chain.isLoading && (
          <div
            role="status"
            aria-busy="true"
            aria-label={t('vendors.detail.loading')}
            className="flex flex-col gap-3"
          >
            <div className="h-10 w-1/2 rounded-lg or-skeleton" />
            <div className="h-20 rounded-xl or-skeleton" />
            <div className="h-40 rounded-xl or-skeleton" />
          </div>
        )}

        {chain.isError && (
          <Card className="p-6 text-center">
            <p role="alert" className="text-[14px] text-ink mb-4">
              {notFound ? t('vendors.detail.notFound') : t('vendors.detail.loadFailed')}
            </p>
            {!notFound && (
              <Btn label={t('vendors.retry')} onClick={() => void chain.refetch()} />
            )}
          </Card>
        )}

        {chain.data && (
          <>
            <PageHeader
              title={chain.data.vendor_name}
              actions={
                <>
                  <Btn
                    icon={PanelRight}
                    label={t('vendors.detail.openDetails')}
                    onClick={() => drawer.open('vendor', vendorId)}
                  />
                  {canManage && (
                    <Btn
                      primary
                      icon={Send}
                      label={t('vendors.detail.send')}
                      onClick={() => setSendOpen(true)}
                    />
                  )}
                </>
              }
            />

            {record.data && (
              <Card className="p-4">
                <h2 className="sr-only">{t('vendors.detail.facts')}</h2>
                <dl className="grid gap-x-6 gap-y-2 text-[13px] sm:grid-cols-2">
                  {attr('legal_name') && (
                    <Fact term={t('vendors.detail.legalName')} value={attr('legal_name')} />
                  )}
                  {attr('country') && (
                    <Fact term={t('vendors.detail.country')} value={attr('country')} />
                  )}
                  {attr('service_provided') && (
                    <Fact term={t('vendors.detail.service')} value={attr('service_provided')} />
                  )}
                  <Fact
                    term={t('vendors.detail.contact')}
                    value={contactEmail || t('vendors.detail.noContact')}
                  />
                </dl>
              </Card>
            )}

            {delivery && (
              <DeliveryNotice
                delivery={delivery.result}
                kind={delivery.kind}
                onDismiss={() => setDelivery(null)}
              />
            )}

            <ChainSection
              vendorId={vendorId}
              chain={chain.data}
              canManage={canManage}
              here={here}
            />

            <AssessmentsSection
              vendorId={vendorId}
              canManage={canManage}
              onSend={() => setSendOpen(true)}
              onDelivery={(result, kind) => setDelivery({ result, kind })}
            />
          </>
        )}

        {sendOpen && chain.data && (
          <SendAssessmentModal
            vendorId={vendorId}
            vendorName={chain.data.vendor_name}
            contactEmail={contactEmail}
            onClose={() => setSendOpen(false)}
            onSent={(result) => {
              setSendOpen(false);
              setDelivery({ result, kind: 'send' });
            }}
          />
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

/* ------------------------------------------------------------------ chain */

function ChainSection({
  vendorId,
  chain,
  canManage,
  here,
}: {
  vendorId: string;
  chain: VendorChain;
  canManage: boolean;
  here: string;
}) {
  const { t, locale } = useI18n();
  const labels = useVendorLabels();
  const { link, unlink } = useVendorLinkMutations(vendorId);
  const tag = localeTag(locale);

  const remove = (l: VendorChainLink) =>
    unlink.mutate(l.link_id, {
      onSuccess: () => toast.success(t('vendors.chain.removed')),
      onError: () => toast.error(t('vendors.chain.removeFailed')),
    });

  return (
    <section aria-labelledby="vendor-chain-title" className="mt-6">
      <h2 id="vendor-chain-title" className="text-[15px] font-bold text-ink">
        {t('vendors.chain.title')} ({chain.links.length})
      </h2>
      <p className="text-[12.5px] text-ink-muted mb-3">{t('vendors.chain.subtitle')}</p>

      {chain.links.length === 0 ? (
        <Card className="p-5">
          <p className="text-[13.5px] text-ink-soft">
            {canManage ? t('vendors.chain.empty') : t('vendors.chain.emptyReadOnly')}
          </p>
        </Card>
      ) : (
        <ul className="list-none p-0 m-0 flex flex-col gap-3">
          {chain.links.map((l) => {
            const pending = l.link_id.startsWith(PENDING_LINK_PREFIX);
            return (
              <li key={l.link_id}>
                <Card className="p-4">
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-[12px] text-ink-muted">{labels.verb(l.verb)}</p>
                      <div className="flex flex-wrap items-center gap-2">
                        <Link
                          to={drawerHref(here, 'asset', l.asset.id)}
                          className="text-[14px] font-semibold text-ink underline-offset-2 hover:underline break-words"
                        >
                          {l.asset.name}
                        </Link>
                        {l.asset.criticality && <CriticalityBadge level={l.asset.criticality} />}
                      </div>
                    </div>
                    {pending ? (
                      <span role="status" className="text-[12.5px] text-ink-muted">
                        {t('vendors.chain.saving')}
                      </span>
                    ) : (
                      canManage && (
                        <Button
                          type="button"
                          variant="ghost"
                          icon={Unlink}
                          onClick={() => remove(l)}
                          aria-label={t('vendors.chain.remove', { asset: l.asset.name })}
                          title={t('vendors.chain.remove', { asset: l.asset.name })}
                        />
                      )
                    )}
                  </div>

                  {!pending && (
                    <div className="mt-3">
                      <p className="text-[12px] font-semibold text-ink-soft mb-1">
                        {t('vendors.chain.risks')} ({l.risks.length})
                      </p>
                      {l.risks.length === 0 ? (
                        <p className="text-[13px] text-ink-muted">{t('vendors.chain.noRisk')}</p>
                      ) : (
                        <ul className="list-none p-0 m-0 flex flex-col gap-1.5">
                          {l.risks.map((r) => {
                            const level = knownRiskLevel(r.criticality);
                            return (
                              <li key={r.id} className="flex flex-wrap items-center gap-2 text-[13px]">
                                <Link
                                  to={drawerHref(here, 'risk', r.id)}
                                  className="text-ink underline-offset-2 hover:underline break-words"
                                >
                                  {r.title}
                                </Link>
                                <span className="tabular-nums text-ink-muted">
                                  {t('vendors.chain.score', { score: formatNumber(r.score, tag) })}
                                </span>
                                {level && (
                                  <Badge intent={tierIntent(level)}>{labels.tier(level)}</Badge>
                                )}
                              </li>
                            );
                          })}
                        </ul>
                      )}
                    </div>
                  )}
                </Card>
              </li>
            );
          })}
        </ul>
      )}

      {canManage && <AddLinkForm link={link} />}
    </section>
  );
}

type LinkMutation = ReturnType<typeof useVendorLinkMutations>['link'];

function AddLinkForm({ link }: { link: LinkMutation }) {
  const { t } = useI18n();
  const labels = useVendorLabels();
  const { assets, isLoading, isError, refetch } = useAssets();
  const [draft, setDraft] = useState<LinkDraft>({ assetId: '', verb: LINK_VERBS[0] });
  const [errors, setErrors] = useState<FormErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  // A vendor cannot be linked to another vendor (the server refuses it), so
  // vendors are not offered.
  const eligible = useMemo(
    () =>
      assets
        .filter((a) => Boolean(a.id) && a.category !== 'vendor')
        .sort((x, y) => (x.name ?? '').localeCompare(y.name ?? '')),
    [assets],
  );

  const err = (field: string): string | undefined =>
    errors[field] ? t(`vendors.chain.errors.${errors[field]}`) : undefined;

  const change = (patch: Partial<LinkDraft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setErrors({});
    setSubmitError(null);
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const found = validateLink(draft);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    const asset = eligible.find((a) => a.id === draft.assetId);
    const verb = LINK_VERBS.find((v) => v === draft.verb);
    if (!asset?.id || !verb) return;
    setSubmitError(null);
    link.mutate(
      {
        input: { asset_id: asset.id, verb },
        asset: {
          id: asset.id,
          name: asset.name ?? '',
          type: asset.type ?? '',
          category: asset.category ?? '',
          criticality: asset.criticality ?? '',
        },
      },
      {
        onSuccess: () => toast.success(t('vendors.chain.added')),
        onError: (e2) =>
          setSubmitError(
            httpStatus(e2) === 409
              ? t('vendors.chain.duplicate')
              : t('vendors.chain.addFailed', {
                  message: apiErrorMessage(e2) || t('vendors.chain.unknownError'),
                }),
          ),
      },
    );
    setDraft((d) => ({ ...d, assetId: '' }));
  };

  return (
    <Card className="p-4 mt-3">
      <h3 className="text-[14px] font-bold text-ink mb-3">{t('vendors.chain.addTitle')}</h3>

      {isLoading ? (
        <div
          role="status"
          aria-busy="true"
          aria-label={t('vendors.chain.assetsLoading')}
          className="h-16 rounded-lg or-skeleton"
        />
      ) : isError ? (
        <div className="flex flex-wrap items-center gap-3">
          <p role="alert" className="text-[13.5px] text-ink">
            {t('vendors.chain.assetsFailed')}
          </p>
          <Button type="button" variant="secondary" onClick={() => void refetch()}>
            {t('vendors.retry')}
          </Button>
        </div>
      ) : eligible.length === 0 ? (
        <p className="text-[13.5px] text-ink-soft">
          {t('vendors.chain.noEligibleAsset')}{' '}
          <Link to="/assets" className="font-semibold text-accent underline">
            {t('vendors.chain.goToInventory')}
          </Link>
        </p>
      ) : (
        <form
          noValidate
          onSubmit={submit}
          aria-label={t('vendors.chain.addTitle')}
          className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] items-start"
        >
          <Field
            label={t('vendors.chain.asset')}
            htmlFor="vendor-link-asset"
            required
            message={err('assetId')}
            status={err('assetId') ? 'invalid' : 'default'}
          >
            <Select
              id="vendor-link-asset"
              value={draft.assetId}
              onChange={(e) => change({ assetId: e.target.value })}
            >
              <option value="">{t('vendors.chain.chooseAsset')}</option>
              {eligible.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field
            label={t('vendors.chain.verb')}
            htmlFor="vendor-link-verb"
            required
            message={err('verb')}
            status={err('verb') ? 'invalid' : 'default'}
          >
            <Select
              id="vendor-link-verb"
              value={draft.verb}
              onChange={(e) => change({ verb: e.target.value })}
            >
              {LINK_VERBS.map((v) => (
                <option key={v} value={v}>
                  {labels.verb(v)}
                </option>
              ))}
            </Select>
          </Field>
          <div className="sm:pt-6">
            <Button type="submit" variant="primary" icon={Link2}>
              {t('vendors.chain.add')}
            </Button>
          </div>
          {submitError && (
            <p
              role="alert"
              className="sm:col-span-3 text-[13px]"
              style={{ color: 'var(--critical)' }}
            >
              {submitError}
            </p>
          )}
        </form>
      )}
    </Card>
  );
}

/* ------------------------------------------------------------ assessments */

function AssessmentsSection({
  vendorId,
  canManage,
  onSend,
  onDelivery,
}: {
  vendorId: string;
  canManage: boolean;
  onSend: () => void;
  onDelivery: (result: VendorAssessmentDelivery, kind: DeliveryKind) => void;
}) {
  const { t, locale } = useI18n();
  const labels = useVendorLabels();
  const list = useVendorAssessments(vendorId);
  const templates = useQuestionnaireTemplates(true);
  const { revoke, resend } = useVendorAssessmentMutations(vendorId);
  const [revoking, setRevoking] = useState<VendorAssessment | null>(null);
  const tag = localeTag(locale);
  const date = (iso: string | null | undefined) =>
    iso ? new Date(iso).toLocaleDateString(tag) : '';

  const nameOf = (a: VendorAssessment): string => {
    const name = templates.data?.find((x) => x.id === a.template_id)?.name;
    return name
      ? `${name} · ${t('vendors.assessments.version', { version: a.template_version })}`
      : t('vendors.assessments.unnamed', { version: a.template_version });
  };

  const confirmRevoke = () => {
    if (!revoking) return;
    revoke.mutate(revoking.id, {
      onSuccess: () => toast.success(t('vendors.assessments.revoked')),
      onError: () => toast.error(t('vendors.assessments.revokeFailed')),
    });
    setRevoking(null);
  };

  const items = list.data ?? [];

  return (
    <section aria-labelledby="vendor-assessments-title" className="mt-8">
      <h2 id="vendor-assessments-title" className="text-[15px] font-bold text-ink mb-3">
        {t('vendors.assessments.title')}
        {list.data ? ` (${items.length})` : ''}
      </h2>

      {list.isLoading && (
        <div
          role="status"
          aria-busy="true"
          aria-label={t('vendors.assessments.loading')}
          className="flex flex-col gap-2"
        >
          <div className="h-16 rounded-xl or-skeleton" />
          <div className="h-16 rounded-xl or-skeleton" />
        </div>
      )}

      {list.isError && (
        <Card className="p-5 flex flex-wrap items-center gap-3">
          <p role="alert" className="text-[13.5px] text-ink">
            {t('vendors.assessments.loadFailed')}
          </p>
          <Btn label={t('vendors.retry')} onClick={() => void list.refetch()} />
        </Card>
      )}

      {list.isSuccess && items.length === 0 && (
        <Card className="p-5 flex flex-wrap items-center gap-3">
          <p className="text-[13.5px] text-ink-soft">{t('vendors.assessments.empty')}</p>
          {canManage && (
            <Btn primary icon={Send} label={t('vendors.detail.send')} onClick={onSend} />
          )}
        </Card>
      )}

      {items.length > 0 && (
        <ul className="list-none p-0 m-0 flex flex-col gap-3">
          {items.map((a) => {
            const sent = date(a.sent_at);
            const facts = [
              t('vendors.assessments.sentOn', { date: sent }),
              t('vendors.assessments.dueOn', { date: date(a.due_at) }),
              a.submitted_at
                ? t('vendors.assessments.submittedOn', { date: date(a.submitted_at) })
                : '',
              t('vendors.assessments.to', { email: a.contact_email }),
            ].filter(Boolean);
            return (
              <li key={a.id}>
                <Card className="p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex flex-col gap-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold text-ink break-words">{nameOf(a)}</span>
                        <Badge intent={statusIntent(a.status)} dot>
                          {labels.status(a.status)}
                        </Badge>
                        {a.score !== null && a.score !== undefined && (
                          <Badge intent={tierIntent(a.tier)}>
                            {t('vendors.assessments.score', { score: formatNumber(a.score, tag) })}
                            {a.tier ? ` · ${labels.tier(a.tier)}` : ''}
                          </Badge>
                        )}
                      </div>
                      <p className="text-[12.5px] text-ink-muted break-words">{facts.join(' · ')}</p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        to={`/vendors/${vendorId}/assessments/${a.id}`}
                        aria-label={t('vendors.assessments.reviewLabel', { date: sent })}
                        className="inline-flex h-8 items-center rounded-lg px-3 text-[13px] font-semibold text-accent underline-offset-2 hover:underline"
                      >
                        {t('vendors.assessments.review')}
                      </Link>
                      {canManage && isOpenAssessment(a.status) && (
                        <>
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            icon={RotateCw}
                            disabled={resend.isPending}
                            aria-label={t('vendors.assessments.resendLabel', { date: sent })}
                            onClick={() =>
                              resend.mutate(a.id, {
                                onSuccess: (result) => onDelivery(result, 'resend'),
                                onError: (e) =>
                                  toast.error(
                                    t('vendors.assessments.resendFailed', {
                                      message:
                                        apiErrorMessage(e) || t('vendors.send.unknownError'),
                                    }),
                                  ),
                              })
                            }
                          >
                            {t('vendors.assessments.resend')}
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            icon={Ban}
                            aria-label={t('vendors.assessments.revokeLabel', { date: sent })}
                            onClick={() => setRevoking(a)}
                          >
                            {t('vendors.assessments.revoke')}
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                </Card>
              </li>
            );
          })}
        </ul>
      )}

      <AlertDialog
        open={revoking !== null}
        onCancel={() => setRevoking(null)}
        onConfirm={confirmRevoke}
        title={t('vendors.assessments.revokeTitle')}
        description={t('vendors.assessments.revokeBody')}
        confirmLabel={t('vendors.assessments.revokeConfirm')}
        cancelLabel={t('vendors.assessments.cancel')}
        tone="destructive"
        busy={revoke.isPending}
      />
    </section>
  );
}

/* --------------------------------------------------------------- delivery */

function DeliveryNotice({
  delivery,
  kind,
  onDismiss,
}: {
  delivery: VendorAssessmentDelivery;
  kind: DeliveryKind;
  onDismiss: () => void;
}) {
  const { t } = useI18n();
  const email = delivery.assessment.contact_email;
  const url = delivery.questionnaire_url;

  let headline: string;
  switch (delivery.delivery) {
    case 'sent':
      headline = t('vendors.delivery.sent', { email });
      break;
    case 'unavailable':
      headline = t('vendors.delivery.unavailable', { email });
      break;
    case 'failed':
    default:
      headline = t('vendors.delivery.failed', { email });
  }

  const copy = async () => {
    if (!url) return;
    try {
      await navigator.clipboard.writeText(url);
      toast.success(t('vendors.delivery.copied'));
    } catch {
      toast.error(t('vendors.delivery.copyFailed'));
    }
  };

  return (
    <Card className="p-4 mt-4">
      <div role="status" className="flex flex-col gap-1">
        <p className="text-[14px] font-semibold text-ink">{headline}</p>
        {kind === 'resend' && (
          <p className="text-[13px] text-ink-soft">{t('vendors.delivery.resentNote')}</p>
        )}
        {delivery.delivery !== 'sent' && delivery.delivery_detail && (
          <p className="text-[12.5px] text-ink-muted break-words">
            {t('vendors.delivery.detail', { detail: delivery.delivery_detail })}
          </p>
        )}
      </div>
      {url && (
        <div className="mt-3 flex flex-col gap-2">
          <Field
            label={t('vendors.delivery.linkLabel')}
            description={t('vendors.delivery.once')}
            htmlFor="vendor-questionnaire-url"
          >
            <Input
              id="vendor-questionnaire-url"
              readOnly
              value={url}
              onFocus={(e) => e.currentTarget.select()}
            />
          </Field>
          <div>
            <Button type="button" variant="primary" icon={Copy} onClick={() => void copy()}>
              {t('vendors.delivery.copy')}
            </Button>
          </div>
        </div>
      )}
      <div className="mt-3">
        <Button type="button" variant="secondary" onClick={onDismiss}>
          {t('vendors.delivery.dismiss')}
        </Button>
      </div>
    </Card>
  );
}

export default VendorDetailPage;
