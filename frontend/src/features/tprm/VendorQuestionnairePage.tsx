// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// The vendor questionnaire — where the link in a questionnaire email lands
// (#674, ADR 0004 D4).
//
// Public by necessity: the vendor contact has no OpenRisk account. So this page
// lives outside the app shell, sends no session (see vendorQuestionnaireService),
// and reads its one credential — the token — from the URL FRAGMENT, which the
// browser never sends to the server when the page loads. The token is never
// rendered and never logged.
//
// The page speaks the contact's language first (the questionnaire's), with a
// local toggle. It does NOT change the viewer's saved language preference: the
// person answering may also use OpenRisk on this browser, for another company.
//
// Dead links get honest, distinct words: an invalid link, a link that stopped
// working (replaced, withdrawn, expired), too many attempts, or a network
// problem. "Something went wrong" tells a legitimate holder nothing to act on.

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from 'react';
import { useLocation } from 'react-router';
import { z } from 'zod';
import { AlertTriangle, CheckCircle2, Languages, Lock } from 'lucide-react';

import { OpenRiskLogo } from '../../shared/Logo';
import { AlertDialog, Checkbox, RadioGroup } from '../../shared/ds';
import { catalogs, localeTag, translate, type LocaleCode, type TranslateParams } from '../../i18n';
import { useUIStore } from '../../store/uiStore';
import {
  classifyQuestionnaireError,
  vendorQuestionnaireService,
  type QuestionnaireFailure,
  type VendorAnswerInput,
  type VendorQuestionnaireItem,
  type VendorQuestionnaireView,
} from './vendorQuestionnaireService';
import { tokenFromHash } from './questionnaireToken';

/** Mirrors the server's caps (domain.MaxVendorAnswerTextLen / CommentLen). */
const MAX_ANSWER_LEN = 5000;
const MAX_COMMENT_LEN = 2000;

type Draft = { value: string; na: boolean; comment: string };

type LoadState =
  | { kind: 'loading' }
  | { kind: 'failed'; failure: QuestionnaireFailure }
  | { kind: 'ready'; view: VendorQuestionnaireView };

type Notice = { tone: 'error' | 'info'; text: string };

function isQuestionnaireLanguage(l: string | undefined): l is LocaleCode {
  return l === 'fr' || l === 'en';
}

function draftOf(item: VendorQuestionnaireItem): Draft {
  return {
    value: item.answer_value ?? '',
    na: item.answer_na ?? false,
    comment: item.answer_comment ?? '',
  };
}

function sameDraft(a: Draft | undefined, b: Draft | undefined): boolean {
  return !!a && !!b && a.value === b.value && a.na === b.na && a.comment === b.comment;
}

// ---------------------------------------------------------------------------
// Validation — Zod, mirroring the server's rules so the vendor learns before the
// round trip. The server still decides.
// ---------------------------------------------------------------------------

const entrySchema = z.object({
  position: z.number().int().positive(),
  required: z.boolean(),
  answered: z.boolean(),
  value: z.string().max(MAX_ANSWER_LEN),
  comment: z.string().max(MAX_COMMENT_LEN),
});
type Entry = z.infer<typeof entrySchema>;

const draftSchema = z.array(entrySchema);

const submissionSchema = draftSchema.superRefine((entries, ctx) => {
  entries.forEach((entry, index) => {
    if (entry.required && !entry.answered) {
      ctx.addIssue({ code: 'custom', path: [index], message: 'required' });
    }
  });
});

function problemsOf(error: z.ZodError, entries: ReadonlyArray<Entry>) {
  const missing = new Set<number>();
  const tooLong = new Set<number>();
  for (const issue of error.issues) {
    const index = issue.path[0];
    if (typeof index !== 'number' || !entries[index]) continue;
    if (issue.message === 'required') missing.add(entries[index].position);
    else tooLong.add(entries[index].position);
  }
  return {
    missing: [...missing].sort((a, b) => a - b),
    tooLong: [...tooLong].sort((a, b) => a - b),
  };
}

// ---------------------------------------------------------------------------

export function VendorQuestionnairePage() {
  const { hash } = useLocation();
  const token = useMemo(() => tokenFromHash(hash), [hash]);
  const fallbackLang = useUIStore((s) => s.lang);

  const [chosenLang, setChosenLang] = useState<LocaleCode | null>(null);
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const [dirty, setDirty] = useState<ReadonlySet<string>>(new Set());
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState<Date | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [justSubmitted, setJustSubmitted] = useState(false);
  const [attempt, setAttempt] = useState(0);

  // Read inside async handlers, where the render's `drafts` may be stale.
  const draftsRef = useRef(drafts);
  useEffect(() => {
    draftsRef.current = drafts;
  }, [drafts]);

  // No token, no request: the invalid-link state is derived, not stored.
  const current: LoadState = token ? state : { kind: 'failed', failure: 'invalid' };
  const view = current.kind === 'ready' ? current.view : null;
  const lang: LocaleCode =
    chosenLang ??
    (view && isQuestionnaireLanguage(view.language)
      ? view.language
      : isQuestionnaireLanguage(fallbackLang)
        ? fallbackLang
        : 'fr');

  const tt = useCallback(
    (key: string, params?: TranslateParams) =>
      translate(catalogs, lang, `vendorQuestionnaire.${key}`, { params }),
    [lang],
  );

  // No Referer leaves this page, whatever a browser extension or a future link
  // does: the page URL carries the token in its fragment.
  useEffect(() => {
    const meta = document.createElement('meta');
    meta.name = 'referrer';
    meta.content = 'no-referrer';
    document.head.appendChild(meta);
    return () => {
      meta.remove();
    };
  }, []);

  useEffect(() => {
    document.title = `${tt('pageTitle')} · OpenRisk`;
  }, [tt]);

  useEffect(() => {
    // A link without a token is answered by `current` below, with no request.
    if (!token) return;
    let cancelled = false;
    vendorQuestionnaireService
      .get(token)
      .then((loaded) => {
        if (cancelled) return;
        setState({ kind: 'ready', view: loaded });
        setDrafts(Object.fromEntries(loaded.items.map((item) => [item.id, draftOf(item)])));
        setDirty(new Set());
      })
      .catch((err: unknown) => {
        if (!cancelled) setState({ kind: 'failed', failure: classifyQuestionnaireError(err) });
      });
    return () => {
      cancelled = true;
    };
  }, [token, attempt]);

  const readOnly = !view || view.read_only;

  const entries: Entry[] = useMemo(
    () =>
      (view?.items ?? []).map((item) => {
        const d = drafts[item.id] ?? draftOf(item);
        return {
          position: item.position,
          required: item.required,
          answered: d.na || d.value.trim() !== '',
          value: d.value,
          comment: d.comment,
        };
      }),
    [view, drafts],
  );
  const answered = entries.filter((e) => e.answered).length;
  const requiredLeft = entries.filter((e) => e.required && !e.answered).length;

  const update = (id: string, patch: Partial<Draft>) => {
    setDrafts((prev) => {
      const base = prev[id] ?? { value: '', na: false, comment: '' };
      return { ...prev, [id]: { ...base, ...patch } };
    });
    setDirty((prev) => new Set(prev).add(id));
    setNotice(null);
  };

  /** Handles a failed write. Returns false so callers can stop. */
  const failWrite = (err: unknown): false => {
    const failure = classifyQuestionnaireError(err);
    switch (failure) {
      case 'invalid':
      case 'gone':
        setState({ kind: 'failed', failure });
        break;
      case 'locked':
        // Submitted elsewhere (another tab): show what the server holds.
        setAttempt((a) => a + 1);
        break;
      case 'rate_limited':
        setNotice({ tone: 'error', text: tt('errors.saveRateLimited') });
        break;
      case 'rejected':
        setNotice({ tone: 'error', text: tt('errors.saveRejected') });
        break;
      default:
        setNotice({ tone: 'error', text: tt('errors.saveFailed') });
    }
    return false;
  };

  const saveDraft = async (): Promise<boolean> => {
    if (!view || dirty.size === 0) return true;
    const check = draftSchema.safeParse(entries);
    if (!check.success) {
      const { tooLong } = problemsOf(check.error, entries);
      setNotice({ tone: 'error', text: tt('tooLong', { position: tooLong[0] ?? '' }) });
      return false;
    }

    const ids = [...dirty];
    const sent: Record<string, Draft> = Object.fromEntries(ids.map((id) => [id, drafts[id]]));
    const answers: VendorAnswerInput[] = ids.map((id) => ({
      item_id: id,
      answer_value: sent[id].na ? null : sent[id].value,
      answer_na: sent[id].na,
      answer_comment: sent[id].comment,
    }));

    setSaving(true);
    try {
      const saved = await vendorQuestionnaireService.saveAnswers(token, answers);
      setState({ kind: 'ready', view: saved });
      // Only what was sent AND not edited since counts as saved.
      setDirty((prev) => {
        const next = new Set(prev);
        for (const id of ids) {
          if (sameDraft(draftsRef.current[id], sent[id])) next.delete(id);
        }
        return next;
      });
      setSavedAt(new Date());
      return true;
    } catch (err) {
      return failWrite(err);
    } finally {
      setSaving(false);
    }
  };

  const requestSubmit = (e: FormEvent) => {
    e.preventDefault();
    const check = submissionSchema.safeParse(entries);
    if (!check.success) {
      const { missing, tooLong } = problemsOf(check.error, entries);
      setNotice({
        tone: 'error',
        text:
          missing.length > 0
            ? tt('missingRequired', { positions: missing.join(', ') })
            : tt('tooLong', { position: tooLong[0] ?? '' }),
      });
      return;
    }
    setNotice(null);
    setConfirmOpen(true);
  };

  const confirmSubmit = async () => {
    setSubmitting(true);
    try {
      if (!(await saveDraft())) {
        setConfirmOpen(false);
        return;
      }
      const submitted = await vendorQuestionnaireService.submit(token);
      setState({ kind: 'ready', view: submitted });
      setDirty(new Set());
      setJustSubmitted(true);
      setConfirmOpen(false);
    } catch (err) {
      setConfirmOpen(false);
      failWrite(err);
    } finally {
      setSubmitting(false);
    }
  };

  const retry = () => {
    setState({ kind: 'loading' });
    setAttempt((a) => a + 1);
  };

  const dateTag = localeTag(lang);
  const organization = view?.organization_name || tt('unknownOrganization');

  return (
    <main className="min-h-screen" style={{ background: 'var(--bg-app)' }}>
      <div className="mx-auto w-full max-w-[720px] px-4 py-6 sm:py-10">
        <header className="mb-6 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            <OpenRiskLogo size={28} />
            <span className="text-[17px] font-bold text-ink">OpenRisk</span>
          </div>
          <button
            type="button"
            onClick={() => setChosenLang(lang === 'fr' ? 'en' : 'fr')}
            aria-label={tt('switchLanguage')}
            title={tt('switchLanguage')}
            data-testid="questionnaire-lang-toggle"
            className="h-10 px-3 rounded-[11px] inline-flex items-center gap-1.5 text-ink-muted hover:text-ink transition-colors"
            style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
          >
            <Languages size={17} aria-hidden="true" />
            <span className="text-[12px] font-semibold uppercase">{lang}</span>
          </button>
        </header>

        {current.kind === 'loading' && <QuestionnaireSkeleton label={tt('loading')} />}

        {current.kind === 'failed' && (
          <FailurePanel failure={current.failure} tt={tt} onRetry={retry} />
        )}

        {view && (
          <form onSubmit={requestSubmit} noValidate>
            <Card>
              <h1 className="text-[20px] font-bold text-ink mb-2">{tt('pageTitle')}</h1>
              <p className="text-[14px] text-ink-soft leading-relaxed">
                {view.vendor_name
                  ? tt('intro', { organization, vendor: view.vendor_name })
                  : tt('introNoVendor', { organization })}
              </p>
              <p className="text-[13.5px] text-ink-soft mt-1">
                {tt('dueDate', {
                  date: new Date(view.due_at).toLocaleDateString(dateTag, {
                    day: 'numeric',
                    month: 'long',
                    year: 'numeric',
                  }),
                })}
              </p>
              <p className="text-[12.5px] text-ink-muted mt-3" aria-live="polite">
                {tt('progress', { answered, total: entries.length })}
                {!readOnly && requiredLeft > 0 && (
                  <> · {tt('requiredLeft', { count: requiredLeft })}</>
                )}
              </p>
            </Card>

            {justSubmitted && (
              <StatusBanner tone="success" icon={<CheckCircle2 size={20} aria-hidden="true" />}>
                <h2 className="text-[15px] font-bold text-ink">{tt('submittedTitle')}</h2>
                <p className="text-[13.5px] text-ink-soft">
                  {tt('submittedBody', { organization })}
                </p>
              </StatusBanner>
            )}
            {!justSubmitted && readOnly && (
              <StatusBanner tone="neutral" icon={<Lock size={18} aria-hidden="true" />}>
                <p className="text-[13.5px] text-ink">{tt('readOnly')}</p>
              </StatusBanner>
            )}

            <ol className="list-none p-0 m-0 flex flex-col gap-3 mt-4">
              {view.items.map((item) => (
                <li key={item.id}>
                  <QuestionCard
                    item={item}
                    draft={drafts[item.id] ?? draftOf(item)}
                    disabled={readOnly || saving || submitting}
                    tt={tt}
                    onChange={(patch) => update(item.id, patch)}
                  />
                </li>
              ))}
            </ol>

            {!readOnly && (
              <div
                className="sticky bottom-0 mt-4 -mx-4 px-4 py-3 flex flex-wrap items-center gap-3"
                style={{ background: 'var(--bg-app)', borderTop: '1px solid var(--border-subtle)' }}
              >
                <div
                  className="flex-1 min-w-[180px] text-[12.5px] text-ink-muted"
                  aria-live="polite"
                >
                  {saving
                    ? tt('saving')
                    : dirty.size > 0
                      ? tt('unsaved')
                      : savedAt
                        ? tt('savedAt', {
                            time: savedAt.toLocaleTimeString(dateTag, {
                              hour: '2-digit',
                              minute: '2-digit',
                            }),
                          })
                        : null}
                </div>
                <button
                  type="button"
                  onClick={() => void saveDraft()}
                  disabled={saving || submitting || dirty.size === 0}
                  className="h-11 px-4 rounded-[10px] text-[13.5px] font-semibold text-ink disabled:opacity-50"
                  style={{
                    background: 'var(--bg-hover)',
                    border: '1px solid var(--border-strong)',
                  }}
                >
                  {saving ? tt('saving') : tt('saveDraft')}
                </button>
                <button
                  type="submit"
                  disabled={saving || submitting}
                  className="h-11 px-5 rounded-[10px] text-[13.5px] font-semibold disabled:opacity-50"
                  style={{ background: 'var(--accent-solid)', color: 'var(--fg-on-solid)' }}
                >
                  {submitting ? tt('submitting') : tt('submit')}
                </button>
                {notice && (
                  <p
                    role="alert"
                    className="w-full text-[13px] flex items-start gap-2"
                    style={{
                      color: notice.tone === 'error' ? 'var(--critical)' : 'var(--fg-secondary)',
                    }}
                  >
                    <AlertTriangle size={15} className="shrink-0 mt-0.5" aria-hidden="true" />
                    <span>{notice.text}</span>
                  </p>
                )}
              </div>
            )}

            <AlertDialog
              open={confirmOpen}
              onCancel={() => setConfirmOpen(false)}
              onConfirm={() => void confirmSubmit()}
              title={tt('confirmTitle')}
              description={tt('confirmBody', { organization })}
              confirmLabel={tt('confirmSubmit')}
              cancelLabel={tt('keepEditing')}
              busy={submitting}
            />
          </form>
        )}
      </div>
    </main>
  );
}

// ---------------------------------------------------------------------------

type Tt = (key: string, params?: TranslateParams) => string;

function Card({ children }: { children: ReactNode }) {
  return (
    <section
      className="rounded-[16px] p-5"
      style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
    >
      {children}
    </section>
  );
}

function StatusBanner({
  tone,
  icon,
  children,
}: {
  tone: 'success' | 'neutral';
  icon: ReactNode;
  children: ReactNode;
}) {
  const color = tone === 'success' ? 'var(--low)' : 'var(--fg-secondary)';
  return (
    <div
      role="status"
      className="mt-4 rounded-[14px] p-4 flex items-start gap-3"
      style={{
        background: `color-mix(in srgb, ${color} 12%, transparent)`,
        border: `1px solid color-mix(in srgb, ${color} 35%, transparent)`,
      }}
    >
      <span className="shrink-0 mt-0.5" style={{ color }}>
        {icon}
      </span>
      <div className="flex flex-col gap-1">{children}</div>
    </div>
  );
}

function QuestionCard({
  item,
  draft,
  disabled,
  tt,
  onChange,
}: {
  item: VendorQuestionnaireItem;
  draft: Draft;
  disabled: boolean;
  tt: Tt;
  onChange: (patch: Partial<Draft>) => void;
}) {
  const helpId = item.help ? `q-${item.id}-help` : undefined;
  const heading = (
    <span className="text-[14px] font-semibold text-ink">
      {item.position}. {item.text}
      {item.required && (
        <span className="ml-1.5 text-[11.5px] font-medium" style={{ color: 'var(--fg-secondary)' }}>
          ({tt('required')})
        </span>
      )}
    </span>
  );

  return (
    <Card>
      {item.answer_type === 'choice' ? (
        <RadioGroup
          legend={heading}
          description={item.help || undefined}
          options={item.options.map((o) => ({ value: o.value, label: o.label }))}
          value={draft.na ? null : draft.value || null}
          onValueChange={(value) => onChange({ value, na: false })}
          disabled={disabled || draft.na}
          name={`q-${item.id}`}
        />
      ) : (
        <div className="flex flex-col gap-1.5">
          <label htmlFor={`q-${item.id}-answer`}>{heading}</label>
          {item.help && (
            <p id={helpId} className="text-[12.5px] text-ink-muted">
              {item.help}
            </p>
          )}
          <textarea
            id={`q-${item.id}-answer`}
            value={draft.value}
            onChange={(e) => onChange({ value: e.target.value })}
            disabled={disabled || draft.na}
            maxLength={MAX_ANSWER_LEN}
            rows={4}
            aria-describedby={helpId}
            aria-required={item.required}
            className="w-full rounded-[10px] p-3 text-[14px] text-ink disabled:opacity-60"
            style={{ background: 'var(--bg-app)', border: '1px solid var(--border-strong)' }}
          />
        </div>
      )}

      {item.na_allowed && (
        <div className="mt-3">
          <Checkbox
            label={tt('notApplicable')}
            checked={draft.na}
            disabled={disabled}
            onChange={(e) => onChange({ na: e.target.checked })}
          />
        </div>
      )}

      <div className="mt-3 flex flex-col gap-1">
        <label htmlFor={`q-${item.id}-comment`} className="text-[12.5px] text-ink-soft">
          {tt('comment')}
        </label>
        <textarea
          id={`q-${item.id}-comment`}
          value={draft.comment}
          onChange={(e) => onChange({ comment: e.target.value })}
          disabled={disabled}
          maxLength={MAX_COMMENT_LEN}
          rows={2}
          className="w-full rounded-[10px] p-2.5 text-[13px] text-ink disabled:opacity-60"
          style={{ background: 'var(--bg-app)', border: '1px solid var(--border-strong)' }}
        />
      </div>
    </Card>
  );
}

function QuestionnaireSkeleton({ label }: { label: string }) {
  // RULE 8: skeletons, never a full-page spinner.
  return (
    <div aria-busy="true" role="status" aria-label={label} className="flex flex-col gap-3">
      <div className="h-28 rounded-[16px] or-skeleton" />
      <div className="h-36 rounded-[16px] or-skeleton" />
      <div className="h-36 rounded-[16px] or-skeleton" />
    </div>
  );
}

function FailurePanel({
  failure,
  tt,
  onRetry,
}: {
  failure: QuestionnaireFailure;
  tt: Tt;
  onRetry: () => void;
}) {
  let titleKey: string;
  let bodyKey: string;
  let retry = false;
  switch (failure) {
    case 'gone':
      titleKey = 'errors.goneTitle';
      bodyKey = 'errors.gone';
      break;
    case 'rate_limited':
      titleKey = 'errors.rateLimitedTitle';
      bodyKey = 'errors.rateLimited';
      retry = true;
      break;
    case 'unavailable':
    case 'locked':
    case 'rejected':
      titleKey = 'errors.unavailableTitle';
      bodyKey = 'errors.unavailable';
      retry = true;
      break;
    case 'invalid':
    default:
      titleKey = 'errors.invalidTitle';
      bodyKey = 'errors.invalid';
  }

  return (
    <Card>
      <div className="text-center py-3" data-testid={`questionnaire-failure-${failure}`}>
        <span
          aria-hidden="true"
          className="w-12 h-12 rounded-[14px] mx-auto mb-4 flex items-center justify-center"
          style={{
            background: 'color-mix(in srgb, var(--high) 14%, transparent)',
            color: 'var(--high)',
          }}
        >
          <AlertTriangle size={24} />
        </span>
        <h1 className="text-[17px] font-bold text-ink mb-2">{tt(titleKey)}</h1>
        <p role="alert" className="text-[13.5px] text-ink-soft leading-relaxed">
          {tt(bodyKey)}
        </p>
        {retry && (
          <button
            type="button"
            onClick={onRetry}
            className="mt-5 h-11 px-5 rounded-[10px] text-[13.5px] font-semibold"
            style={{ background: 'var(--accent-solid)', color: 'var(--fg-on-solid)' }}
          >
            {tt('errors.retry')}
          </button>
        )}
      </div>
    </Card>
  );
}

export default VendorQuestionnairePage;
