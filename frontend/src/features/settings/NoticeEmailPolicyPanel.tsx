// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #997 — Settings › Notifications › "E-mails d'avis".
//
// How long notice e-mails about one item are grouped, for the whole
// organization. The first e-mail goes out at once; the changes that follow
// within the window arrive together, in one digest, when it ends. None is
// dropped. The options come from the server, so the panel offers exactly what
// it accepts. A member who is not an administrator sees the value read-only.

import { toast } from 'sonner';
import { z } from 'zod';

import { Card, SkeletonRows, ErrorState } from '../../shared/ui';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useNoticeEmailPolicy, useSaveNoticeEmailPolicy } from '../notifications/useNotifications';
import { windowLabel, windowSummary } from './noticeEmailWindow';

type Tr = (fr: string, en: string) => string;

export function NoticeEmailPolicyPanel({ tr }: { tr: Tr }) {
  // The same gate as the route: admin, or root, which outranks it.
  const canEdit = useAuthStore((s) => s.hasRole('admin'));
  const { data: policy, isLoading, isError, refetch } = useNoticeEmailPolicy();
  const save = useSaveNoticeEmailPolicy();

  if (isLoading)
    return (
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <SkeletonRows rows={2} height={36} />
      </Card>
    );
  if (isError || !policy) {
    return (
      <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
        <ErrorState
          title={tr('Réglage des e-mails indisponible', 'E-mail setting unavailable')}
          description={tr(
            "Impossible de lire le réglage de l'organisation. Réessayez ; si le problème persiste, contactez un administrateur.",
            "Could not read the organization's setting. Retry; if it persists, contact an administrator.",
          )}
          onRetry={() => void refetch()}
          retryLabel={tr('Réessayer', 'Retry')}
        />
      </Card>
    );
  }

  const schema = z
    .number()
    .int()
    .refine((v) => policy.options.includes(v), {
      message: tr('Choisissez une des durées proposées.', 'Pick one of the offered durations.'),
    });

  const choose = (minutes: number) => {
    const parsed = schema.safeParse(minutes);
    if (!parsed.success) {
      toast.error(parsed.error.issues[0]?.message ?? tr('Valeur refusée.', 'Value refused.'));
      return;
    }
    if (parsed.data === policy.window_minutes || save.isPending) return;
    save.mutate(parsed.data, {
      onSuccess: () => toast.success(tr('Réglage enregistré', 'Setting saved')),
      onError: () =>
        toast.error(
          tr(
            "Enregistrement impossible : le réglage précédent est rétabli. Vérifiez que vous êtes administrateur de l'organisation.",
            'Could not save: the previous setting is back. Check that you are an administrator of the organization.',
          ),
        ),
    });
  };

  return (
    <Card style={{ padding: '20px 22px', marginBottom: 16 }}>
      <div className="flex flex-wrap items-center gap-2 mb-1.5">
        <h2 className="text-[15px] font-semibold text-ink m-0">
          {tr("E-mails d'avis", 'Notice e-mails')}
        </h2>
        <span
          className="text-[11px] font-semibold px-2 py-0.5 rounded-full text-ink-soft"
          style={{ background: 'var(--bg-hover)' }}
        >
          {policy.configured
            ? tr("Réglage de l'organisation", 'Organization setting')
            : tr('Valeur par défaut', 'Default value')}
        </span>
      </div>
      <p className="text-[13px] text-ink-soft leading-relaxed mb-4">
        {tr(
          "Les affectations, échéances, validations et changements d'état peuvent envoyer un e-mail. Pour éviter qu'une série de changements sur un même élément n'en envoie autant, ils sont regroupés. Aucun e-mail n'est perdu, et la cloche reste immédiate.",
          'Assignments, deadlines, validations and state changes can send an e-mail. So that a run of changes to one item does not send as many, they are grouped. No e-mail is lost, and the bell stays immediate.',
        )}
      </p>

      <fieldset className="border-0 p-0 m-0" disabled={!canEdit} aria-describedby="notice-email-summary">
        <legend className="text-[11px] font-semibold uppercase tracking-[.04em] text-ink-muted mb-2">
          {tr('Regrouper les changements sur', 'Group changes over')}
        </legend>
        <div
          className="inline-flex flex-wrap rounded-[10px] p-1 gap-1"
          style={{ border: '1px solid var(--border-strong)', background: 'var(--bg-elevated)' }}
        >
          {policy.options.map((minutes) => {
            const selected = minutes === policy.window_minutes;
            return (
              <label
                key={minutes}
                className={`relative h-9 min-w-[64px] px-3 rounded-[8px] text-[13px] font-semibold inline-flex items-center justify-center transition-colors focus-within:outline focus-within:outline-2 focus-within:outline-offset-1 focus-within:outline-[var(--focus-ring-color)] ${
                  canEdit ? 'cursor-pointer' : 'cursor-not-allowed opacity-70'
                }`}
                style={
                  selected
                    ? { background: 'var(--accent-solid)', color: 'var(--fg-on-solid)' }
                    : { color: 'var(--fg-secondary)' }
                }
              >
                <input
                  type="radio"
                  name="notice-email-window"
                  value={minutes}
                  checked={selected}
                  onChange={() => choose(minutes)}
                  className="sr-only"
                />
                {windowLabel(minutes, tr)}
                {minutes === policy.default_minutes && (
                  <span className="sr-only">{tr(' (par défaut)', ' (default)')}</span>
                )}
              </label>
            );
          })}
        </div>
      </fieldset>

      <p id="notice-email-summary" className="text-[13px] text-ink leading-relaxed mt-3 mb-0" aria-live="polite">
        {windowSummary(policy.window_minutes, tr)}
      </p>
      {!canEdit && (
        <p className="text-[12px] text-ink-muted mt-2 mb-0">
          {tr(
            'Seul un administrateur de l’organisation peut modifier ce réglage.',
            'Only an administrator of the organization can change this setting.',
          )}
        </p>
      )}
    </Card>
  );
}
