// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Send a questionnaire to a vendor (#673, ADR 0004 D3/D4).
//
// The contact email and the language default server-side to the vendor record
// and the questionnaire. When the vendor has no contact email on record the
// field becomes required here, because the server would refuse the send.

import { useState } from 'react';
import { Link } from 'react-router';
import { Send } from 'lucide-react';

import { Button, Field, Input, Modal, Select } from '../../shared/ds';
import { useI18n } from '../../hooks/useI18n';
import { apiErrorMessage } from '../../lib/apiError';
import { useQuestionnaireTemplates } from './useQuestionnaireTemplates';
import { useVendorAssessmentMutations } from './useVendors';
import type { VendorAssessmentDelivery } from './vendorService';
import {
  emptySendDraft,
  toDateInput,
  toSendInput,
  validateSend,
  type FormErrors,
  type SendDraft,
} from './vendorForms';

export interface SendAssessmentModalProps {
  vendorId: string;
  vendorName: string;
  /** The vendor's contact_email attribute, '' when none is on record. */
  contactEmail: string;
  onClose: () => void;
  onSent: (delivery: VendorAssessmentDelivery) => void;
}

export function SendAssessmentModal({
  vendorId,
  vendorName,
  contactEmail,
  onClose,
  onSent,
}: SendAssessmentModalProps) {
  const { t } = useI18n();
  const templates = useQuestionnaireTemplates(false);
  const { send } = useVendorAssessmentMutations(vendorId);

  const [today] = useState(() => new Date());
  const [draft, setDraft] = useState<SendDraft>(() => emptySendDraft(today));
  const [errors, setErrors] = useState<FormErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);

  const emailRequired = contactEmail.trim() === '';
  const active = (templates.data ?? []).filter((tpl) => !tpl.archived_at);
  const noTemplate = templates.isSuccess && active.length === 0;

  const change = (patch: Partial<SendDraft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setErrors({});
    setSubmitError(null);
  };

  const err = (field: string): string | undefined =>
    errors[field] ? t(`vendors.send.errors.${errors[field]}`) : undefined;

  const submit = () => {
    if (send.isPending) return;
    const found = validateSend(draft, { now: new Date(), emailRequired });
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    send.mutate(toSendInput(draft), {
      onSuccess: (delivery) => onSent(delivery),
      onError: (e) =>
        setSubmitError(
          t('vendors.send.failed', {
            message: apiErrorMessage(e) || t('vendors.send.unknownError'),
          }),
        ),
    });
  };

  return (
    <Modal
      open
      onClose={onClose}
      dismissable={!send.isPending}
      title={t('vendors.send.title')}
      subtitle={t('vendors.send.subtitle', { vendor: vendorName })}
      closeLabel={t('vendors.send.cancel')}
      footer={
        <div className="flex w-full flex-wrap items-center justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={send.isPending}>
            {t('vendors.send.cancel')}
          </Button>
          <Button
            type="button"
            variant="primary"
            icon={Send}
            onClick={submit}
            disabled={send.isPending || noTemplate}
          >
            {send.isPending ? t('vendors.send.sending') : t('vendors.send.submit')}
          </Button>
        </div>
      }
    >
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        {templates.isLoading && (
          <div
            role="status"
            aria-busy="true"
            aria-label={t('vendors.send.templatesLoading')}
            className="h-16 rounded-lg or-skeleton"
          />
        )}

        {templates.isError && (
          <div className="flex flex-wrap items-center gap-3">
            <p role="alert" className="text-[13.5px] text-ink">
              {t('vendors.send.templatesFailed')}
            </p>
            <Button type="button" variant="secondary" onClick={() => void templates.refetch()}>
              {t('vendors.retry')}
            </Button>
          </div>
        )}

        {noTemplate && (
          <div className="flex flex-col items-start gap-2">
            <p className="text-[13.5px] text-ink">{t('vendors.send.noTemplate')}</p>
            <Link
              to="/vendors/questionnaires/new"
              className="text-[13.5px] font-semibold text-accent underline"
            >
              {t('vendors.send.createTemplate')}
            </Link>
          </div>
        )}

        {active.length > 0 && (
          <Field
            label={t('vendors.send.template')}
            htmlFor="send-template"
            required
            message={err('templateId')}
            status={err('templateId') ? 'invalid' : 'default'}
          >
            <Select
              id="send-template"
              value={draft.templateId}
              onChange={(e) => change({ templateId: e.target.value })}
            >
              <option value="">{t('vendors.send.chooseTemplate')}</option>
              {active.map((tpl) => (
                <option key={tpl.id} value={tpl.id}>
                  {tpl.name} · v{tpl.version}
                </option>
              ))}
            </Select>
          </Field>
        )}

        <Field
          label={t('vendors.send.dueDate')}
          description={t('vendors.send.dueHelp')}
          htmlFor="send-due"
          required
          message={err('dueDate')}
          status={err('dueDate') ? 'invalid' : 'default'}
        >
          <Input
            id="send-due"
            type="date"
            min={toDateInput(today)}
            value={draft.dueDate}
            onChange={(e) => change({ dueDate: e.target.value })}
          />
        </Field>

        <Field
          label={t('vendors.send.email')}
          description={
            emailRequired
              ? t('vendors.send.emailRequired')
              : t('vendors.send.emailDefault', { email: contactEmail })
          }
          htmlFor="send-email"
          required={emailRequired}
          message={err('contactEmail')}
          status={err('contactEmail') ? 'invalid' : 'default'}
        >
          <Input
            id="send-email"
            type="email"
            autoComplete="off"
            placeholder={contactEmail || undefined}
            value={draft.contactEmail}
            onChange={(e) => change({ contactEmail: e.target.value })}
          />
        </Field>

        <Field label={t('vendors.send.language')} htmlFor="send-language">
          <Select
            id="send-language"
            value={draft.contactLanguage}
            onChange={(e) => {
              const v = e.target.value;
              change({ contactLanguage: v === 'fr' || v === 'en' ? v : '' });
            }}
          >
            <option value="">{t('vendors.send.languageDefault')}</option>
            <option value="fr">{t('vendors.send.languages.fr')}</option>
            <option value="en">{t('vendors.send.languages.en')}</option>
          </Select>
        </Field>

        {submitError && (
          <p role="alert" className="text-[13px]" style={{ color: 'var(--critical)' }}>
            {submitError}
          </p>
        )}
      </form>
    </Modal>
  );
}
