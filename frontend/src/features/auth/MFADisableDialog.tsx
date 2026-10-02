// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// #754 — turning MFA off from Settings › Security.
//
// An open session is not proof of who is at the keyboard, so the server asks
// for the password again — or, for an account that signs in through an
// identity provider and has no password here, a current code from the
// authenticator app. The dialog says what is lost (the authenticator AND the
// backup codes), and a wrong answer is reported on the field itself.

import { Controller, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { isAxiosError } from 'axios';
import { z } from 'zod';
import { ShieldOff } from 'lucide-react';
import { toast } from 'sonner';
import { useState } from 'react';

import { Button, Field, Input, Modal, OtpField } from '../../shared/ds';
import { SkeletonRows } from '../../shared/ui';
import { useUIStore } from '../../store/uiStore';
import { useAuthStore } from '../../hooks/useAuthStore';
import { useDisableMFA, useDisableMFAProof } from './useMfa';
import type { DisableMFAErrorBody, DisableMFAProof } from './authService';

type Tr = (fr: string, en: string) => string;

function schema(tr: Tr, proof: DisableMFAProof) {
  return z.object({
    password:
      proof === 'password'
        ? z.string().min(1, tr('Saisissez votre mot de passe.', 'Enter your password.'))
        : z.string(),
    code:
      proof === 'code'
        ? z
            .string()
            .regex(/^\d{6}$/, tr('Saisissez les 6 chiffres du code.', 'Enter the 6-digit code.'))
        : z.string(),
  });
}

type Values = z.infer<ReturnType<typeof schema>>;

export function MFADisableDialog({ onClose }: { onClose: () => void }) {
  const lang = useUIStore((s) => s.lang);
  const tr: Tr = (fr, en) => (lang === 'fr' ? fr : en);
  const email = useAuthStore((s) => s.user?.email ?? '');
  const disable = useDisableMFA();
  const proofQuery = useDisableMFAProof();
  // The server can correct the guess: `wrong_code` to a password means this
  // account has no password and must confirm with its authenticator.
  const [corrected, setCorrected] = useState<DisableMFAProof | null>(null);
  const proof: DisableMFAProof = corrected ?? proofQuery.data ?? 'password';
  // A refusal the password field cannot fix (role, identity provider, throttle).
  const [blocked, setBlocked] = useState<string | null>(null);

  const {
    register,
    control,
    handleSubmit,
    setError,
    clearErrors,
    formState: { errors },
  } = useForm<Values>({
    resolver: zodResolver(schema(tr, proof)),
    defaultValues: { password: '', code: '' },
  });

  const onSubmit = (v: Values) => {
    setBlocked(null);
    disable.mutate(
      { proof: proof === 'code' ? { code: v.code } : { password: v.password }, locale: lang },
      {
        onSuccess: (res) => {
          toast.success(res.message);
          onClose();
        },
        onError: (err) => {
          const body = (isAxiosError(err) ? err.response?.data : undefined) as
            DisableMFAErrorBody | undefined;
          switch (body?.code) {
            case 'wrong_password':
              setError('password', { message: body.error });
              return;
            case 'wrong_code':
              if (proof !== 'code') {
                // We asked for a password this account does not have.
                clearErrors();
                setCorrected('code');
                return;
              }
              setError('code', { message: body.error });
              return;
            case 'mfa_required_by_role':
            case 'no_local_password':
            case 'too_many_attempts':
              setBlocked(body.error ?? '');
              return;
            case 'not_enrolled':
              // Already off (another tab, another device): nothing left to do.
              toast.success(body.error ?? '');
              onClose();
              return;
            default:
              toast.error(
                body?.error ||
                  tr(
                    'La désactivation a échoué. Réessayez dans un instant.',
                    'Could not turn it off. Try again in a moment.',
                  ),
              );
          }
        },
      },
    );
  };

  const busy = disable.isPending;
  const resolving = proofQuery.isLoading && corrected === null;

  return (
    <Modal
      open
      onClose={onClose}
      size="sm"
      dismissable={!busy}
      closeLabel={tr('Fermer', 'Close')}
      title={tr('Désactiver le MFA', 'Turn off MFA')}
      subtitle={
        proof === 'code'
          ? tr(
              "Confirmez avec un code de votre application d'authentification.",
              'Confirm with a code from your authenticator app.',
            )
          : tr('Confirmez votre mot de passe pour continuer.', 'Confirm your password to continue.')
      }
      leading={
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-danger-surface text-danger-text">
          <ShieldOff size={18} aria-hidden="true" />
        </span>
      }
      footer={
        <>
          <Button variant="ghost" onClick={onClose} disabled={busy}>
            {tr('Annuler', 'Cancel')}
          </Button>
          <Button
            variant="destructive"
            type="submit"
            form="mfa-disable-form"
            loading={busy}
            disabled={blocked !== null || resolving}
            data-testid="mfa-disable-submit"
          >
            {tr('Désactiver', 'Turn off')}
          </Button>
        </>
      }
    >
      <form
        id="mfa-disable-form"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
        className="space-y-3"
      >
        <p className="text-sm leading-relaxed text-fg-secondary">
          {proof === 'code'
            ? tr(
                "Votre application d'authentification et vos codes de secours cesseront de fonctionner. La connexion via votre fournisseur d'identité suffira pour ouvrir votre compte. Un e-mail vous confirmera le changement.",
                'Your authenticator app and your backup codes will stop working. Signing in through your identity provider alone will open your account. You will get an email confirming the change.',
              )
            : tr(
                "Votre application d'authentification et vos codes de secours cesseront de fonctionner. Le mot de passe seul suffira pour ouvrir votre compte. Un e-mail vous confirmera le changement.",
                'Your authenticator app and your backup codes will stop working. A password alone will open your account. You will get an email confirming the change.',
              )}
        </p>
        {resolving ? (
          <SkeletonRows rows={1} height={40} />
        ) : proof === 'code' ? (
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
                  autoFocus
                  label={tr("Code de l'application d'authentification", 'Authenticator app code')}
                  invalid={!!errors.code}
                  describedBy={errors.code ? 'mfa-disable-code-error' : undefined}
                  testId="mfa-disable-code"
                />
              )}
            />
            {errors.code && (
              <p id="mfa-disable-code-error" role="alert" className="text-xs text-danger-text">
                {errors.code.message}
              </p>
            )}
          </div>
        ) : (
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
                data-testid="mfa-disable-password"
              />
            </Field>
          </>
        )}
        {blocked !== null && (
          <p
            role="alert"
            className="rounded-md bg-surface-3 p-3 text-sm text-fg-secondary"
            data-testid="mfa-disable-blocked"
          >
            {blocked}
          </p>
        )}
      </form>
    </Modal>
  );
}
