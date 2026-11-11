// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #1032 — regenerating MFA backup codes from Settings › Security.
//
// New codes stand in for the second factor, so the server asks for that factor:
// a current code from the authenticator app, always, plus the password when the
// account has one. The new codes are shown once, with copy and download, and
// the old ones stop working the moment they appear.

import { Controller, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { isAxiosError } from 'axios';
import { z } from 'zod';
import { Copy, Download, KeyRound } from 'lucide-react';
import { toast } from 'sonner';
import { useState } from 'react';

import { Button, Field, Input, Modal, OtpField } from '../../shared/ds';
import { SkeletonRows } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useHasLocalPassword, useRegenerateBackupCodes } from './useMfa';
import type { RegenerateBackupCodesErrorBody } from './authService';

type Tr = (fr: string, en: string) => string;

function schema(tr: Tr, needsPassword: boolean) {
  return z.object({
    password: needsPassword
      ? z.string().min(1, tr('Saisissez votre mot de passe.', 'Enter your password.'))
      : z.string(),
    code: z
      .string()
      .regex(/^\d{6}$/, tr('Saisissez les 6 chiffres du code.', 'Enter the 6-digit code.')),
  });
}

type Values = z.infer<ReturnType<typeof schema>>;

function downloadCodes(codes: string[], email: string, tr: Tr) {
  const header = tr(
    `Codes de secours OpenRisk — ${email}\nChaque code ne fonctionne qu'une fois.\n\n`,
    `OpenRisk backup codes — ${email}\nEach code works once.\n\n`,
  );
  const blob = new Blob([header + codes.join('\n') + '\n'], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'openrisk-backup-codes.txt';
  a.click();
  URL.revokeObjectURL(url);
}

export function MFABackupCodesDialog({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr: Tr = (fr, en) => (lang === 'fr' ? fr : en);
  const email = useAuthStore((s) => s.user?.email ?? '');
  const regenerate = useRegenerateBackupCodes();
  const passwordQuery = useHasLocalPassword();
  // Unknown reads as "has a password": the server ignores a password sent for
  // an account without one, and refuses a missing one for an account with one.
  const needsPassword = passwordQuery.data !== false;
  // A refusal the fields cannot fix (throttle).
  const [blocked, setBlocked] = useState<string | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [copied, setCopied] = useState(false);

  const {
    register,
    control,
    handleSubmit,
    setError,
    clearErrors,
    setValue,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema(tr, needsPassword)),
    defaultValues: { password: '', code: '' },
  });

  const onSubmit = (v: Values) => {
    setBlocked(null);
    regenerate.mutate(
      {
        proof: needsPassword ? { password: v.password, code: v.code } : { code: v.code },
        locale: lang,
      },
      {
        onSuccess: (res) => {
          setCodes(res.backup_codes ?? []);
        },
        onError: (err) => {
          const body = (isAxiosError(err) ? err.response?.data : undefined) as
            RegenerateBackupCodesErrorBody | undefined;
          switch (body?.code) {
            case 'wrong_password':
              setError('password', { message: body.error });
              return;
            case 'wrong_code':
              // The code was consumed or wrong either way: ask for the next one.
              setValue('code', '');
              setError('code', { message: body.error });
              return;
            case 'too_many_attempts':
              setBlocked(body.error ?? '');
              return;
            case 'not_enrolled':
              toast.error(body.error ?? '');
              onClose();
              return;
            default:
              toast.error(
                body?.error ||
                  tr(
                    'La génération a échoué. Vos anciens codes restent valables. Réessayez dans un instant.',
                    'Could not generate new codes. Your old codes still work. Try again in a moment.',
                  ),
              );
          }
        },
      },
    );
  };

  const copyCodes = async () => {
    if (!codes?.length) return;
    try {
      await navigator.clipboard.writeText(codes.join('\n'));
      setCopied(true);
      toast.success(tr('Codes copiés', 'Codes copied'));
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error(tr('Copie impossible', 'Could not copy'));
    }
  };

  const busy = regenerate.isPending;
  const resolving = passwordQuery.isLoading;

  if (codes !== null) {
    return (
      <Modal
        open
        onClose={onClose}
        size="sm"
        closeLabel={tr('Fermer', 'Close')}
        title={tr('Nouveaux codes de secours', 'New backup codes')}
        subtitle={tr(
          'Ils ne seront plus affichés. Vos anciens codes ne fonctionnent plus.',
          'They will not be shown again. Your old codes no longer work.',
        )}
        leading={
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface-3 text-fg-primary">
            <KeyRound size={18} aria-hidden="true" />
          </span>
        }
        footer={
          <Button onClick={onClose} data-testid="mfa-regenerate-done">
            {tr("J'ai mis mes codes à l'abri", 'I have saved my codes')}
          </Button>
        }
      >
        {codes.length === 0 ? (
          <p role="alert" className="text-sm text-danger-text" data-testid="mfa-regenerate-empty">
            {tr(
              "Le serveur n'a renvoyé aucun code. Réessayez ; si cela se reproduit, contactez votre administrateur.",
              'The server returned no codes. Try again; if this happens again, contact your administrator.',
            )}
          </p>
        ) : (
          <div className="space-y-3">
            <p className="text-sm leading-relaxed text-fg-secondary">
              {tr(
                "Conservez-les hors ligne. Chacun ne fonctionne qu'une fois, et ils sont votre seule issue si vous perdez votre appareil.",
                'Keep these offline. Each works once, and they are your only way back in if you lose your device.',
              )}
            </p>
            <ul className="grid grid-cols-2 gap-1.5" data-testid="mfa-regenerate-codes">
              {codes.map((c) => (
                <li key={c}>
                  <code
                    className="block rounded-[8px] px-2 py-1.5 text-center text-[12px] text-ink"
                    style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border)' }}
                  >
                    {c}
                  </code>
                </li>
              ))}
            </ul>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                onClick={copyCodes}
                data-testid="mfa-regenerate-copy"
                icon={Copy}
                feedback={copied ? 'success' : undefined}
              >
                {tr('Copier', 'Copy')}
              </Button>
              <Button
                variant="secondary"
                onClick={() => downloadCodes(codes, email, tr)}
                data-testid="mfa-regenerate-download"
                icon={Download}
              >
                {tr('Télécharger (.txt)', 'Download (.txt)')}
              </Button>
            </div>
          </div>
        )}
      </Modal>
    );
  }

  return (
    <Modal
      open
      onClose={onClose}
      size="sm"
      dismissable={!busy}
      closeLabel={tr('Fermer', 'Close')}
      title={tr('Régénérer les codes de secours', 'Regenerate backup codes')}
      subtitle={
        needsPassword
          ? tr(
              "Confirmez avec votre mot de passe et un code de votre application d'authentification.",
              'Confirm with your password and a code from your authenticator app.',
            )
          : tr(
              "Confirmez avec un code de votre application d'authentification.",
              'Confirm with a code from your authenticator app.',
            )
      }
      leading={
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-surface-3 text-fg-primary">
          <KeyRound size={18} aria-hidden="true" />
        </span>
      }
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {tr('Annuler', 'Cancel')}
          </Button>
          <Button
            type="submit"
            form="mfa-regenerate-form"
            loading={busy}
            disabled={blocked !== null || resolving}
            data-testid="mfa-regenerate-submit"
          >
            {tr('Générer de nouveaux codes', 'Generate new codes')}
          </Button>
        </>
      }
    >
      <form
        id="mfa-regenerate-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="space-y-3"
      >
        <p className="text-sm leading-relaxed text-fg-secondary">
          {tr(
            'Vos codes de secours actuels cesseront de fonctionner dès que les nouveaux seront affichés. Votre application d’authentification ne change pas. Un e-mail vous confirmera le changement.',
            'Your current backup codes stop working as soon as the new ones are shown. Your authenticator app does not change. You will get an email confirming the change.',
          )}
        </p>
        {resolving ? (
          <SkeletonRows rows={2} height={40} />
        ) : (
          <>
            {needsPassword && (
              <>
                {/* Tells password managers which account this password belongs to. */}
                <input
                  type="text"
                  name="username"
                  autoComplete="username"
                  value={email}
                  readOnly
                  hidden
                />
                <Field
                  label={tr('Mot de passe actuel', 'Current password')}
                  required
                  message={errors.password?.message}
                  status={errors.password ? 'invalid' : 'default'}
                >
                  <Input
                    {...register('password')}
                    type="password"
                    autoComplete="current-password"
                    autoFocus
                    data-testid="mfa-regenerate-password"
                  />
                </Field>
              </>
            )}
            <div className="flex flex-col gap-1.5">
              <span className="text-sm font-medium text-fg-primary">
                {tr("Code de l'application d'authentification", 'Authenticator app code')}
              </span>
              <Controller
                control={control}
                name="code"
                render={({ field }) => (
                  <OtpField
                    value={field.value}
                    onValueChange={(next) => {
                      field.onChange(next);
                      clearErrors('code');
                    }}
                    length={6}
                    autoFocus={!needsPassword}
                    label={tr("Code de l'application d'authentification", 'Authenticator app code')}
                    invalid={!!errors.code}
                    describedBy={errors.code ? 'mfa-regenerate-code-error' : undefined}
                    testId="mfa-regenerate-code"
                  />
                )}
              />
              {errors.code && (
                <p id="mfa-regenerate-code-error" role="alert" className="text-xs text-danger-text">
                  {errors.code.message}
                </p>
              )}
            </div>
          </>
        )}
        {blocked !== null && (
          <p
            role="alert"
            className="rounded-md bg-surface-3 p-3 text-sm text-fg-secondary"
            data-testid="mfa-regenerate-blocked"
          >
            {blocked}
          </p>
        )}
      </form>
    </Modal>
  );
}
