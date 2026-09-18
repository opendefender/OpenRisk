// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Create, edit or view one vendor questionnaire template (#681, ADR 0004 D3).
//
// Saving an existing template writes a NEW VERSION. Questionnaires already sent
// were snapshotted when they left, so an edit here can never rewrite what a
// vendor was asked or a score already computed — the screen says so before the
// author saves, because "will this change what Acme already answered?" is the
// question every author asks.
//
// Questions are reordered with buttons, not drag and drop: the charter requires
// the editor to be operable end to end from the keyboard.

import { useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import axios from 'axios';
import { toast } from 'sonner';
import { Archive, ArrowDown, ArrowLeft, ArrowUp, Plus, Trash2 } from 'lucide-react';

import { PageFrame, PageHeader, Btn, Card } from '../../shared/ui';
import { FeatureGate } from '../../shared/FeatureGate';
import { AlertDialog, Button, Checkbox, Field, Input, Select, Textarea } from '../../shared/ds';
import { useI18n } from '../../hooks/useI18n';
import { useAuthStore } from '../../hooks/useAuthStore';
import {
  useQuestionnaireTemplate,
  useQuestionnaireTemplateMutations,
} from './useQuestionnaireTemplates';
import {
  MAX_NAME,
  MAX_TEXT,
  emptyDraft,
  fromTemplate,
  move,
  newOption,
  newQuestion,
  toTemplateInput,
  validateTemplate,
  type FormErrors,
  type QuestionDraft,
  type TemplateDraft,
} from './questionnaireTemplateForm';

function serverMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as { error?: string; message?: string } | undefined;
    return data?.error ?? data?.message ?? err.message;
  }
  return err instanceof Error ? err.message : '';
}

function statusOf(err: unknown): number | undefined {
  return axios.isAxiosError(err) ? err.response?.status : undefined;
}

function numberFrom(raw: string): number {
  return raw.trim() === '' ? Number.NaN : Number(raw);
}

export function QuestionnaireTemplateEditorPage() {
  const { templateId } = useParams();
  const isNew = !templateId;
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const canManage = useAuthStore((s) => s.hasPermission('vendors:manage'));

  const query = useQuestionnaireTemplate(templateId);
  const { create, update, archive } = useQuestionnaireTemplateMutations();

  const [initialNew] = useState(() => emptyDraft(locale === 'en' ? 'en' : 'fr'));
  const loaded = query.data;
  const base = useMemo(() => (loaded ? fromTemplate(loaded) : null), [loaded]);
  const [edits, setEdits] = useState<TemplateDraft | null>(null);
  const draft: TemplateDraft | null = edits ?? (isNew ? initialNew : base);

  const [errors, setErrors] = useState<FormErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [archiveOpen, setArchiveOpen] = useState(false);

  const archived = Boolean(loaded?.archived_at);
  const readOnly = !canManage || archived;
  const saving = create.isPending || update.isPending;

  const change = (mutate: (d: TemplateDraft) => TemplateDraft) => {
    if (!draft || readOnly) return;
    setEdits(mutate(draft));
    setErrors({});
    setSubmitError(null);
  };

  const changeQuestion = (index: number, patch: Partial<QuestionDraft>) =>
    change((d) => ({
      ...d,
      questions: d.questions.map((q, i) => (i === index ? { ...q, ...patch } : q)),
    }));

  const err = (path: string): string | undefined =>
    errors[path] ? t(`vendorTemplates.errors.${errors[path]}`) : undefined;

  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!draft || readOnly) return;
    const found = validateTemplate(draft);
    setErrors(found);
    if (Object.keys(found).length > 0) {
      setSubmitError(t('vendorTemplates.editor.fixErrors'));
      return;
    }
    setSubmitError(null);
    const input = toTemplateInput(draft);

    if (isNew) {
      create.mutate(input, {
        onSuccess: (created) => {
          // The same editor instance stays mounted across /new → /:id, so the
          // local draft must be dropped or it would keep showing over the saved
          // template the server just returned.
          setEdits(null);
          toast.success(t('vendorTemplates.editor.created'));
          navigate(`/vendors/questionnaires/${created.id}`, { replace: true });
        },
        onError: (e2) =>
          setSubmitError(t('vendorTemplates.editor.saveFailed', { message: serverMessage(e2) })),
      });
      return;
    }
    update.mutate(
      { id: templateId, input },
      {
        onSuccess: (saved) => {
          setEdits(null);
          toast.success(t('vendorTemplates.editor.saved', { version: saved.version }));
        },
        onError: (e2) =>
          setSubmitError(t('vendorTemplates.editor.saveFailed', { message: serverMessage(e2) })),
      },
    );
  };

  const confirmArchive = () => {
    if (!templateId) return;
    archive.mutate(templateId, {
      onSuccess: () => toast.success(t('vendorTemplates.editor.archivedToast')),
      onError: () => toast.error(t('vendorTemplates.editor.archiveFailed')),
    });
    setArchiveOpen(false);
  };

  const title = isNew
    ? t('vendorTemplates.editor.newTitle')
    : readOnly
      ? t('vendorTemplates.editor.viewTitle')
      : t('vendorTemplates.editor.editTitle');

  return (
    <PageFrame>
      <FeatureGate feature="vendor_risk">
        <Link
          to="/vendors/questionnaires"
          className="inline-flex items-center gap-1.5 text-[13px] text-ink-soft hover:text-ink mb-3"
        >
          <ArrowLeft size={15} aria-hidden="true" />
          {t('vendorTemplates.editor.back')}
        </Link>
        <PageHeader
          title={loaded?.name && !isNew ? loaded.name : title}
          badge={
            loaded ? (
              <span className="text-xs text-ink-muted tabular-nums">v{loaded.version}</span>
            ) : undefined
          }
          actions={
            !isNew && canManage && !archived && loaded ? (
              <Btn
                icon={Archive}
                label={t('vendorTemplates.editor.archive')}
                onClick={() => setArchiveOpen(true)}
              />
            ) : undefined
          }
        />

        {!isNew && query.isLoading && (
          <div
            aria-busy="true"
            role="status"
            aria-label={t('vendorTemplates.loading')}
            className="flex flex-col gap-3"
          >
            <div className="h-24 rounded-xl or-skeleton" />
            <div className="h-48 rounded-xl or-skeleton" />
          </div>
        )}

        {!isNew && query.isError && (
          <Card className="p-6 text-center">
            <p role="alert" className="text-[14px] text-ink mb-4">
              {statusOf(query.error) === 404
                ? t('vendorTemplates.editor.notFound')
                : t('vendorTemplates.editor.loadFailed')}
            </p>
            {statusOf(query.error) !== 404 && (
              <Btn label={t('vendorTemplates.retry')} onClick={() => void query.refetch()} />
            )}
          </Card>
        )}

        {draft && (
          <form onSubmit={onSubmit} noValidate aria-label={title}>
            {archived && <Notice>{t('vendorTemplates.editor.archivedNotice')}</Notice>}
            {!archived && !canManage && (
              <Notice>{t('vendorTemplates.editor.readOnlyNotice')}</Notice>
            )}
            {!isNew && !readOnly && loaded && (
              <Notice>
                {t('vendorTemplates.editor.versionNotice', { next: loaded.version + 1 })}
              </Notice>
            )}

            <Card className="p-5 flex flex-col gap-4">
              <Field
                label={t('vendorTemplates.editor.name')}
                htmlFor="tpl-name"
                required
                message={err('name')}
                status={err('name') ? 'invalid' : 'default'}
              >
                <Input
                  id="tpl-name"
                  value={draft.name}
                  maxLength={MAX_NAME}
                  disabled={readOnly}
                  onChange={(e) => change((d) => ({ ...d, name: e.target.value }))}
                />
              </Field>
              <Field label={t('vendorTemplates.editor.description')} htmlFor="tpl-description">
                <Textarea
                  id="tpl-description"
                  value={draft.description}
                  rows={2}
                  disabled={readOnly}
                  onChange={(e) => change((d) => ({ ...d, description: e.target.value }))}
                />
              </Field>
              <Field label={t('vendorTemplates.editor.language')} htmlFor="tpl-language">
                <Select
                  id="tpl-language"
                  value={draft.language}
                  disabled={readOnly}
                  onChange={(e) =>
                    change((d) => ({ ...d, language: e.target.value === 'en' ? 'en' : 'fr' }))
                  }
                >
                  <option value="fr">{t('vendorTemplates.languages.fr')}</option>
                  <option value="en">{t('vendorTemplates.languages.en')}</option>
                </Select>
              </Field>
            </Card>

            <h2 className="text-[15px] font-bold text-ink mt-6 mb-2">
              {t('vendorTemplates.editor.questions')} ({draft.questions.length})
            </h2>
            {err('questions') && (
              <p role="alert" className="text-[13px] mb-2" style={{ color: 'var(--critical)' }}>
                {err('questions')}
              </p>
            )}

            <ol className="list-none p-0 m-0 flex flex-col gap-3">
              {draft.questions.map((q, index) => (
                <li key={q.key}>
                  <QuestionEditor
                    question={q}
                    index={index}
                    count={draft.questions.length}
                    readOnly={readOnly}
                    t={t}
                    err={err}
                    onChange={(patch) => changeQuestion(index, patch)}
                    onMove={(to) =>
                      change((d) => ({ ...d, questions: move(d.questions, index, to) }))
                    }
                    onRemove={() =>
                      change((d) => ({
                        ...d,
                        questions: d.questions.filter((_, i) => i !== index),
                      }))
                    }
                  />
                </li>
              ))}
            </ol>

            {!readOnly && (
              <div className="mt-3">
                <Button
                  type="button"
                  variant="secondary"
                  icon={Plus}
                  onClick={() =>
                    change((d) => ({ ...d, questions: [...d.questions, newQuestion(d.language)] }))
                  }
                >
                  {t('vendorTemplates.editor.addQuestion')}
                </Button>
              </div>
            )}

            {!readOnly && (
              <div
                className="sticky bottom-0 mt-6 -mx-1 px-1 py-3 flex flex-wrap items-center gap-3"
                style={{
                  background: 'var(--bg-primary)',
                  borderTop: '1px solid var(--border-subtle)',
                }}
              >
                {submitError && (
                  <p
                    role="alert"
                    className="flex-1 min-w-[200px] text-[13px]"
                    style={{ color: 'var(--critical)' }}
                  >
                    {submitError}
                  </p>
                )}
                <div className="ml-auto">
                  <Btn
                    type="submit"
                    primary
                    loading={saving}
                    disabled={saving}
                    label={
                      saving
                        ? t('vendorTemplates.editor.saving')
                        : isNew
                          ? t('vendorTemplates.editor.create')
                          : t('vendorTemplates.editor.save')
                    }
                  />
                </div>
              </div>
            )}
          </form>
        )}

        <AlertDialog
          open={archiveOpen}
          onCancel={() => setArchiveOpen(false)}
          onConfirm={confirmArchive}
          title={t('vendorTemplates.editor.archiveTitle')}
          description={t('vendorTemplates.editor.archiveBody')}
          confirmLabel={t('vendorTemplates.editor.archiveConfirm')}
          cancelLabel={t('vendorTemplates.editor.cancel')}
          tone="destructive"
          busy={archive.isPending}
        />
      </FeatureGate>
    </PageFrame>
  );
}

function Notice({ children }: { children: React.ReactNode }) {
  return (
    <p
      role="status"
      className="mb-3 rounded-[10px] px-3.5 py-2.5 text-[13px] text-ink"
      style={{ background: 'var(--bg-hover)', border: '1px solid var(--border-subtle)' }}
    >
      {children}
    </p>
  );
}

type T = (key: string, params?: Record<string, string | number>) => string;

function QuestionEditor({
  question: q,
  index,
  count,
  readOnly,
  t,
  err,
  onChange,
  onMove,
  onRemove,
}: {
  question: QuestionDraft;
  index: number;
  count: number;
  readOnly: boolean;
  t: T;
  err: (path: string) => string | undefined;
  onChange: (patch: Partial<QuestionDraft>) => void;
  onMove: (to: number) => void;
  onRemove: () => void;
}) {
  const position = index + 1;
  const id = `tpl-q-${q.key}`;
  const path = `questions.${index}`;

  return (
    <Card className="p-4">
      <fieldset className="flex flex-col gap-3 border-0 p-0 m-0" disabled={readOnly}>
        <legend className="w-full flex items-center justify-between gap-2 mb-1">
          <span className="text-[13.5px] font-bold text-ink">
            {t('vendorTemplates.editor.questionLabel', { position })}
          </span>
          {!readOnly && (
            <span className="flex items-center gap-1">
              <Button
                type="button"
                variant="ghost"
                icon={ArrowUp}
                disabled={index === 0}
                onClick={() => onMove(index - 1)}
                aria-label={t('vendorTemplates.editor.moveUp', { position })}
                title={t('vendorTemplates.editor.moveUp', { position })}
              />
              <Button
                type="button"
                variant="ghost"
                icon={ArrowDown}
                disabled={index === count - 1}
                onClick={() => onMove(index + 1)}
                aria-label={t('vendorTemplates.editor.moveDown', { position })}
                title={t('vendorTemplates.editor.moveDown', { position })}
              />
              <Button
                type="button"
                variant="ghost"
                icon={Trash2}
                onClick={onRemove}
                aria-label={t('vendorTemplates.editor.removeQuestion', { position })}
                title={t('vendorTemplates.editor.removeQuestion', { position })}
              />
            </span>
          )}
        </legend>

        <Field
          label={t('vendorTemplates.editor.text')}
          htmlFor={`${id}-text`}
          required
          message={err(`${path}.text`)}
          status={err(`${path}.text`) ? 'invalid' : 'default'}
        >
          <Textarea
            id={`${id}-text`}
            value={q.text}
            rows={2}
            maxLength={MAX_TEXT}
            onChange={(e) => onChange({ text: e.target.value })}
          />
        </Field>

        <Field label={t('vendorTemplates.editor.help')} htmlFor={`${id}-help`}>
          <Input
            id={`${id}-help`}
            value={q.help}
            maxLength={MAX_TEXT}
            onChange={(e) => onChange({ help: e.target.value })}
          />
        </Field>

        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t('vendorTemplates.editor.answerType')} htmlFor={`${id}-type`}>
            <Select
              id={`${id}-type`}
              value={q.answerType}
              onChange={(e) =>
                onChange({ answerType: e.target.value === 'text' ? 'text' : 'choice' })
              }
            >
              <option value="choice">{t('vendorTemplates.editor.answerChoice')}</option>
              <option value="text">{t('vendorTemplates.editor.answerText')}</option>
            </Select>
          </Field>
          {q.answerType === 'choice' && (
            <Field
              label={t('vendorTemplates.editor.weight')}
              description={t('vendorTemplates.editor.weightHelp')}
              htmlFor={`${id}-weight`}
              message={err(`${path}.weight`)}
              status={err(`${path}.weight`) ? 'invalid' : 'default'}
            >
              <Input
                id={`${id}-weight`}
                type="number"
                min={0}
                max={10}
                step={0.5}
                value={Number.isNaN(q.weight) ? '' : q.weight}
                onChange={(e) => onChange({ weight: numberFrom(e.target.value) })}
              />
            </Field>
          )}
        </div>

        {q.answerType === 'choice' && (
          <fieldset className="border-0 p-0 m-0">
            <legend className="text-[12.5px] font-semibold text-ink-soft mb-1.5">
              {t('vendorTemplates.editor.options')}
            </legend>
            <p className="text-[12px] text-ink-muted mb-2">
              {t('vendorTemplates.editor.optionPointsHelp')}
            </p>
            {err(`${path}.options`) && (
              <p role="alert" className="text-[12.5px] mb-2" style={{ color: 'var(--critical)' }}>
                {err(`${path}.options`)}
              </p>
            )}
            <ol className="list-none p-0 m-0 flex flex-col gap-2">
              {q.options.map((o, oi) => {
                const optionPath = `${path}.options.${oi}`;
                const idx = oi + 1;
                return (
                  <li key={o.key} className="grid gap-2 sm:grid-cols-[1fr_140px_auto] items-start">
                    <Field
                      label={t('vendorTemplates.editor.optionLabel', { index: idx })}
                      htmlFor={`${id}-opt-${o.key}-label`}
                      message={err(`${optionPath}.label`)}
                      status={err(`${optionPath}.label`) ? 'invalid' : 'default'}
                    >
                      <Input
                        id={`${id}-opt-${o.key}-label`}
                        value={o.label}
                        onChange={(e) =>
                          onChange({
                            options: q.options.map((x, xi) =>
                              xi === oi ? { ...x, label: e.target.value } : x,
                            ),
                          })
                        }
                      />
                    </Field>
                    <Field
                      label={t('vendorTemplates.editor.optionPoints', { index: idx })}
                      htmlFor={`${id}-opt-${o.key}-points`}
                      message={err(`${optionPath}.points`)}
                      status={err(`${optionPath}.points`) ? 'invalid' : 'default'}
                    >
                      <Input
                        id={`${id}-opt-${o.key}-points`}
                        type="number"
                        min={0}
                        max={1}
                        step={0.1}
                        value={Number.isNaN(o.points) ? '' : o.points}
                        onChange={(e) =>
                          onChange({
                            options: q.options.map((x, xi) =>
                              xi === oi ? { ...x, points: numberFrom(e.target.value) } : x,
                            ),
                          })
                        }
                      />
                    </Field>
                    {!readOnly && (
                      <div className="sm:pt-6">
                        <Button
                          type="button"
                          variant="ghost"
                          icon={Trash2}
                          onClick={() =>
                            onChange({ options: q.options.filter((_, xi) => xi !== oi) })
                          }
                          aria-label={t('vendorTemplates.editor.removeOption', { index: idx })}
                          title={t('vendorTemplates.editor.removeOption', { index: idx })}
                        />
                      </div>
                    )}
                  </li>
                );
              })}
            </ol>
            {!readOnly && (
              <div className="mt-2">
                <Button
                  type="button"
                  variant="ghost"
                  icon={Plus}
                  onClick={() => onChange({ options: [...q.options, newOption(q.options)] })}
                >
                  {t('vendorTemplates.editor.addOption')}
                </Button>
              </div>
            )}
          </fieldset>
        )}

        <div className="flex flex-wrap gap-x-6 gap-y-2">
          <Checkbox
            label={t('vendorTemplates.editor.required')}
            checked={q.required}
            onChange={(e) => onChange({ required: e.target.checked })}
          />
          <Checkbox
            label={t('vendorTemplates.editor.naAllowed')}
            checked={q.naAllowed}
            onChange={(e) => onChange({ naAllowed: e.target.checked })}
          />
        </div>

        <Field
          label={t('vendorTemplates.editor.controlRef')}
          description={t('vendorTemplates.editor.controlRefHelp')}
          htmlFor={`${id}-ref`}
        >
          <Input
            id={`${id}-ref`}
            value={q.controlRef}
            maxLength={200}
            onChange={(e) => onChange({ controlRef: e.target.value })}
          />
        </Field>
      </fieldset>
    </Card>
  );
}

export default QuestionnaireTemplateEditorPage;
